package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

const localDataPath = "/home/geertjohan/gitrefs/"

// gitProvider, used to make process exceptions per provider
type gitProvider string

var (
	gitProviderGithub = gitProvider("github.com")
)

// refType, used to identify how to handle a reporef
type refType string

var (
	refTypeCommit = refType("commit")
	refTypeBranch = refType("branch") // or tag

	regexpCommitHash = regexp.MustCompile("^([a-f0-9]{40})$")
)

var (
	errorUnknownOrUnsupportedProvider = errors.New("Unknown or unsupported provider.")
)

type reporef struct {
	identifier  string
	gitProvider gitProvider
	user        string
	repo        string
	repoPath    string
	ref         string
	refType     refType

	updateTime time.Time
	updateLock sync.Mutex
}

var (
	reporefByIdentifier     = make(map[string]*reporef)
	reporefByIdentifierLock sync.Mutex
)

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

	dataPath := localDataPath + r.identifier

	// Make dirs (if not exists)
	err := os.MkdirAll(dataPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return err
	}

	// Run git clone
	gitCloneCmd := exec.Command("git", "clone", "git://"+r.repoPath, "./")
	gitCloneCmd.Dir = dataPath
	gitCloneOutput, err := gitCloneCmd.CombinedOutput()
	gitCloneExists := strings.Contains(string(gitCloneOutput), "destination path '.' already exists and is not an empty directory")
	if err != nil && !gitCloneExists {
		log.Printf("Error: %s\n", err)
		log.Printf("Output: %s\n", string(gitCloneOutput))
		return err
	}

	if gitCloneExists {
		gitPullCmd := exec.Command("git", "pull")
		gitPullCmd.Dir = dataPath
		gitPullOutput, err := gitPullCmd.CombinedOutput()
		if err != nil {
			log.Printf("Error: %s\n", err)
			log.Printf("Output: %s\n", string(gitPullOutput))
			return err
		}
		//++ check what gitPull actually does..
	}

	gitCheckoutCmd := exec.Command("git", "checkout", "-qf", r.ref)
	gitCheckoutCmd.Dir = dataPath
	gitCheckoutOutput, err := gitCheckoutCmd.CombinedOutput()
	if err != nil {
		log.Printf("Error: %s\n", err)
		log.Printf("Output: %s\n", string(gitCheckoutOutput))
		return err
	}
	//++ check what gitCheckout actually does..
	//++ cleanup if pathspec does not exist (ref does not exist)

	log.Printf("%s\n", string(gitCloneOutput))

	// Done
	r.updateTime = time.Now()
	return nil
}

func reporefFromRequestURI(requestURI string) (*reporef, error) {
	r := &reporef{}

	// Remove eventual slash at the beginning of the URI
	requestURI = strings.TrimLeft(requestURI, "/")

	// Discard GET parameters from requestURI
	if questionMarkPos := strings.Index(requestURI, "?"); questionMarkPos > 0 {
		requestURI = requestURI[:questionMarkPos]
	}

	// Most unique way to identify this reporef is to include the repoPath and the ref.
	r.identifier = requestURI

	// See if this reporef already exists in the reporef map
	// The map is completely locked, and is unlocked until this function returns.
	// This means that only one reporef can be created or retrieved from the map at the same time.
	reporefByIdentifierLock.Lock()
	defer reporefByIdentifierLock.Unlock()
	if existingReporef, exists := reporefByIdentifier[r.identifier]; exists {
		return existingReporef, nil
	}
	reporefByIdentifier[r.identifier] = r

	// Split the requestURI into fields
	fields := strings.Split(requestURI, "/")

	// Obtain ref from last field. Store in r.ref. Remove from last field.
	lastField := fields[len(fields)-1]
	if equalsSignPos := strings.LastIndex(lastField, "@"); equalsSignPos > 0 {
		r.ref = lastField[equalsSignPos+1:]
		fields[len(fields)-1] = lastField[:equalsSignPos]
		if regexpCommitHash.MatchString(r.ref) {
			r.refType = refTypeCommit
		} else {
			r.refType = refTypeBranch
		}
	} else {
		r.ref = "master"
		r.refType = refTypeBranch
	}

	// Continueing steps differ for each git provider
	r.gitProvider = gitProvider(fields[0])
	switch r.gitProvider {
	case gitProviderGithub:
		r.user = fields[1]
		r.repo = fields[2]
		r.repoPath = strings.Join(fields[0:], "/")
		log.Println("This.. is... GITHUB!!!")
	default:
		return nil, errorUnknownOrUnsupportedProvider
	}

	//++ lock reporef map for write
	//++ check again that reporef does not exists ?? race condition here....

	// initial updateRepository
	r.updateRepository()

	// Done
	return r, nil
}
