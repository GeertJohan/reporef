package main

import (
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

const gitBase = "/home/geertjohan/gitrefs/"

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

// Updates the repository if the ref is a branch and the updateTime was a while ago.
// If the repository is up to date
// ++ request the update with by sending a chan on a chan. The sended chan is to be reacted upon with the 'did something' bool.
// ++ initial sender wait for the update (if any) to be done..
// ++ 'infodesk' with a queue of waiting channels.. use a double goroutine (one accepting new channels and queueing them.. another doing the update work(invoked by the first))
// ++ if there is no update running (and not required) then call the done chan immediatly
func (r *reporef) updateRepositoryIfNeeded() (bool, error) {
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
	log.Println("TODO: impl reporef.updateRepository()")

	// Make dirs
	err := os.MkdirAll(gitBase+r.repoPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return err
	}

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

	//++ see if this reporef already exists in the reporef map
	//++ lock the map for reading., unlock right after fail.

	// Split the requestURI into fields
	fields := strings.Split(requestURI, "/")

	// Obtain ref from last field. Store in r.ref. Remove from last field.
	lastField := fields[len(fields)-1]
	if equalsSignPos := strings.LastIndex(lastField, "@"); equalsSignPos > 0 {
		r.ref = lastField[equalsSignPos+1:]
		fields[len(fields)-1] = lastField[:equalsSignPos]
		//++ analyse r.ref and find it's refType (commit or branch)
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

	log.Printf("%#v\n", r)

	//++ lock reporef map for write
	//++ check again that reporef does not exists ?? race condition here....

	// Done
	return r, nil
}
