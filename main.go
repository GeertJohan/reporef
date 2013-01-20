package main

import (
	"log"
	"net/http"
	"strings"
)

const (
	gitDataPath         = "/opt/reporef/gitdata/"
	publicResourcesPath = "./public/"
	templatesPath       = "./templates/"
)

func main() {

	//++ TODO: Verry important TODO!!!!
	//Split initial setup for a repo, and /updating/ a repo.
	// Update can fail, will just serve old data..
	// Initial setup should NEVER! fail and will have to call cleanupGitData()
	// This will make everything a lot nicer.
	// Also: less logging, only log when something goes wrong...

	// serve requests for /.git/ repo's
	publicResourcesHandler := http.StripPrefix("/public/", http.FileServer(http.Dir(publicResourcesPath)))

	indexHandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		tmplIndex.Execute(w, &dataIndex{
			Header: &dataHeader{
				Title: "Link to a branch/commit.",
			},
		})
	}

	statsHandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		tmplStats.Execute(w, &dataStats{
			Header: &dataHeader{
				Title: "Statistics",
			},
			TotalReporefs: 42,
		})
	}

	reporefHandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		// Get the rr for this request
		rr, err := reporefFromRequestURI(r.RequestURI)
		if err != nil {
			log.Printf("Error: %s\n", err)
			return
		}

		// Update if required.
		// TODO: concurrent update process
		_, err = rr.updateRepositoryIfNeeded()
		if err != nil {
			log.Printf("Error: %s\n", err)
			return
		}

		// Check if request is meant for the git repository.
		if rr.isGitHttpRequest(r.RequestURI) {
			rr.httpGitFileHandler.ServeHTTP(w, r)
			return
		}

		// go-get page with meta tag
		if r.FormValue("go-get") == "1" {
			pageData := &dataGoGet{rr.identifier}
			tmplGoGet.Execute(w, pageData)
			return
		}

		// Reporef page
		pageData := &dataReporef{
			Header: &dataHeader{
				Title: "RepoRef: " + rr.identifier,
			},
			Footer:           nil,
			Identifier:       rr.identifier,
			OriginalRepoPath: rr.originalRepoPath,
			RefType:          string(rr.refType),
			Ref:              rr.ref,
		}
		tmplReporef.Execute(w, pageData)
	}

	// Manual mux for all requests
	rootHandler := func(w http.ResponseWriter, r *http.Request) {
		// Alias for reporef.com
		if r.Host == "reporef.com" {
			http.Redirect(w, r, "http://reporef.org"+r.RequestURI, 302)
			return
		}

		// Index handler.
		if r.RequestURI == "/" {
			indexHandlerFunc(w, r)
			return
		}

		fields := strings.SplitN(r.RequestURI[1:], "/", 2)
		switch fields[0] {
		case "github.com":
			reporefHandlerFunc(w, r)
		case "public":
			publicResourcesHandler.ServeHTTP(w, r)
		case "about":
			http.Redirect(w, r, "https://github.com/GeertJohan/reporef", 307)
		case "stats":
			statsHandlerFunc(w, r)
		case "project":
			http.Redirect(w, r, "https://github.com/GeertJohan/reporef", 307)
		default:
			NotFound(w, r)
		}
	}

	// serve any request in the root (serve website with meta tag for go-get, redirecting to /git/)
	http.HandleFunc("/", rootHandler)

	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal(err)
	}
}
