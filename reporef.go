package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	errorUnknownOrUnsupportedProvider = errors.New("Unknown or unsupported git provider.")
	errorWontUpdateCommitReporef      = errors.New("The reference is a commit, commits will not change though time so update is not required.")
	errorSetupFailed                  = errors.New("Could not setup reporef.")
)

// refType, used to identify how to handle a reporef
type refType string

var (
	refTypeCommit = refType("commit")
	refTypeBranch = refType("branch") // or tag

	regexpCommitHash = regexp.MustCompile("^([a-f0-9]{40})$")
)

type reporef struct {
	identifier  string       // e.g. "github.com/GeertJohan/yubigo@4e3b79592f949dd09320c611fa60732777099f87"
	gitProvider *gitProvider // e.g. gitProviderGithub
	user        string       // e.g. "GeertJohan"
	repo        string       // e.g. "yubigo"
	ref         string       // e.g. "4e3b79592f949dd09320c611fa60732777099f87"
	refType     refType

	originalRepoPath string // e.g. "github.com/GeertJohan/yubigo"

	httpGitFileHandler http.Handler

	updateTime time.Time
	updateLock sync.Mutex
}

var (
	reporefByIdentifier     = make(map[string]*reporef)
	reporefByIdentifierLock sync.Mutex
)

// Create or pick-existing reporef from requestURI (git request or url)
func reporefFromRequestURI(requestURI string) (*reporef, error) {
	var err error
	r := &reporef{}

	// Remove eventual slash at the beginning of the URI
	requestURI = strings.TrimLeft(requestURI, "/")

	// Discard GET parameters from requestURI
	if questionMarkPos := strings.Index(requestURI, "?"); questionMarkPos > 0 {
		requestURI = requestURI[:questionMarkPos]
	}

	// Split the requestURI into fields
	fields := strings.Split(requestURI, "/")

	// Continueing steps differ for each git provider
	r.gitProvider, err = gitProviderFromHost(fields[0])
	if err != nil {
		return nil, errorUnknownOrUnsupportedProvider
	}
	switch r.gitProvider {
	case gitProviderGithub:
		// Most unique way to identify this reporef is to include the repoPath and the ref.
		r.identifier = strings.Join(fields[0:3], "/")
	default:
		panic("Missing case for a valid git provider in switch statement.")
	}

	// See if this reporef already exists in the reporef map
	// The map is completely locked, and is unlocked until this function returns.
	// This means that only one reporef can be created or retrieved from the map at the same time.
	reporefByIdentifierLock.Lock()
	defer reporefByIdentifierLock.Unlock()
	if existingReporef, exists := reporefByIdentifier[r.identifier]; exists {
		return existingReporef, nil
	}

	// Continue to fill in this reporef..
	var reporefField string
	switch r.gitProvider {
	case gitProviderGithub:
		r.user = fields[1]
		reporefField = fields[2]
	default:
		panic("Missing case for a valid git provider in switch statement.")
	}

	// Obtain ref from last field. Store in r.ref. Remove from last field.
	if equalsSignPos := strings.LastIndex(reporefField, "@"); equalsSignPos > 0 {
		r.repo = reporefField[:equalsSignPos]
		r.ref = reporefField[equalsSignPos+1:]
		if regexpCommitHash.MatchString(r.ref) {
			r.refType = refTypeCommit
		} else {
			r.refType = refTypeBranch
		}
	} else {
		r.repo = reporefField
		r.ref = "master"
		r.refType = refTypeBranch
	}

	// Thig might not work for all providers..
	r.originalRepoPath = r.gitProvider.host + "/" + r.user + "/" + r.repo

	//++ lock reporef map for write
	//++ check again that reporef does not exists ?? race condition here....

	fmt.Printf("%#v\n", r)

	// initial updateRepository
	err = r.updateRepository()
	if err != nil {
		return nil, errorSetupFailed
	}

	// create fileserver
	r.httpGitFileHandler = http.StripPrefix("/"+r.identifier, http.FileServer(http.Dir(gitDataPath+r.identifier+"/.git/")))

	// done creating the reporef.. save it in-memory
	reporefByIdentifier[r.identifier] = r

	// Done
	return r, nil
}

func (r *reporef) isGitHttpRequest(uri string) bool {
	if strings.HasPrefix(uri, "/"+r.identifier+"/objects/") || strings.HasPrefix(uri, "/"+r.identifier+"/info/refs") || strings.HasPrefix(uri, "/"+r.identifier+"/HEAD") {
		return true
	}
	return false
}

// Updates the repository if the ref is a branch and the updateTime was a while ago.
// If the repository is up to date
// ++ request the update with by sending a chan on a chan.
// ++ initial sender wait for the update (if any) to be done..
// ++ 'infodesk' with a queue of waiting channels.. use a double goroutine (one accepting new channels and queueing them.. another doing the update work(invoked by the first))
// ++ if there is no update running (and not required) then call the done chan immediatly
func (r *reporef) updateRepositoryIfNeeded() (bool, error) {
	// update if required (not a commit (those don't change over time) and time past > 1 minute)
	if r.refType != refTypeCommit && time.Since(r.updateTime) > 1*time.Minute {
		return true, r.updateRepository()
	}
	return false, nil
}

func (r *reporef) updateRepository() error {

	// Lock the reporef so we're completely sure it is not being
	//   touched by another update goroutine (which should not exist anyway).
	r.updateLock.Lock()
	defer r.updateLock.Unlock()

	dataPath := gitDataPath + r.identifier

	// Make dirs (if not exists)
	err := os.MkdirAll(dataPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return err
	}

	//++ see if there is an existing git repo in the dataPath (dataPath+".git")
	// Try git clone
	gitCloneCmd := exec.Command("git", "clone", "git://"+r.originalRepoPath, "./")
	gitCloneCmd.Dir = dataPath
	fmt.Printf("---\nExecuting `git clone git://%s ./`\n", r.originalRepoPath)
	gitCloneOutput, err := gitCloneCmd.CombinedOutput()
	reporefCloneExists := strings.Contains(string(gitCloneOutput), "destination path '.' already exists and is not an empty directory")
	if err != nil && !reporefCloneExists {
		fmt.Printf("Error: %s\n", err)
		fmt.Printf("Output: %s\n", string(gitCloneOutput))

		// clean up (remove repo)
		errRemove := os.RemoveAll(dataPath)
		if errRemove != nil {
			fmt.Printf("Error cleaning up: %s\n", errRemove)
		}

		return err
	}
	fmt.Println("Done")

	if reporefCloneExists {
		// don't continue if this reporef has refType commit.
		// commits dont change over time.. update is never required.
		if r.refType == refTypeCommit {
			return errorWontUpdateCommitReporef
		}

		// we didn't make a new clone, so we have to make a pull from the upstream repository to update.
		gitPullCmd := exec.Command("git", "pull", "origin", r.ref)
		gitPullCmd.Dir = dataPath
		fmt.Printf("---\nExecuting `git pull origin %s`\n", r.ref)
		gitPullOutput, err := gitPullCmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			fmt.Printf("Output: %s\n", string(gitPullOutput))
			return err
		}
		fmt.Println("Done")
		//++ check what gitPull actually does.. (does it work?)
	} else {
		// do a git checkout to get the right branch or commit from local git repository.
		gitCheckoutCmd := exec.Command("git", "checkout", "-qf", r.ref)
		gitCheckoutCmd.Dir = dataPath
		fmt.Printf("---\nExecuting `git checkout -qf %s`\n", r.ref)
		gitCheckoutOutput, err := gitCheckoutCmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			fmt.Printf("Output: %s\n", string(gitCheckoutOutput))
			return err
		}
		fmt.Println("Done")
		//++ check what gitCheckout actually does..
		//++ cleanup if pathspec does not exist (ref does not exist)
	}

	// Change the HEAD file to point to the version we want..
	// branch: "ref: refs/heads/master" where master is the ref
	// commit: ref as commit hash
	fmt.Printf("---\nChanging HEAD and master file to point to %s %s\n", string(r.refType), r.ref)

	var commitHash string
	switch r.refType {
	case refTypeBranch:
		commitHashBytes, err := ioutil.ReadFile(dataPath + "/.git/refs/heads/" + r.ref)
		if err != nil {
			fmt.Printf("Error when reading /refs/heads/%s file for commit hash: %s\n", r.ref, err)
			//++ cleanup/remove reporef
			return err
		}
		commitHash = string(commitHashBytes) + "\n"
	case refTypeCommit:
		commitHash = r.ref + "\n"
	default:
		panic("unhandled refType")
	}

	// let HEAD file point to ref/heads/master
	headFile, err := os.Create(dataPath + "/.git/HEAD")
	if err != nil {
		fmt.Printf("Error when creating/truncating headFile: %s\n", err)
		//++ cleanup/remove reporef
		return err
	}
	headFile.WriteString("ref: refs/heads/master\n")

	masterFile, err := os.Create(dataPath + "/.git/refs/heads/master")
	if err != nil {
		fmt.Printf("Error when creating/truncating masterFile: %s\n", err)
		//++ cleanup/remove reporef
		return err
	}
	masterFile.WriteString(commitHash)
	fmt.Println("Done")

	gitUsiCmd := exec.Command("git", "update-server-info")
	gitUsiCmd.Dir = dataPath
	fmt.Printf("---\nExecuting `git update-server-info`\n")
	gitUsiOutput, err := gitUsiCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		fmt.Printf("Output: %s\n", gitUsiOutput)
		//++ cleanup/remove reporef
		return err
	}
	fmt.Println("Done")

	// Done
	r.updateTime = time.Now()
	return nil
}
