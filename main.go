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

	// serve requests for /.git/ repo's
	gitDataHandler := http.FileServer(http.Dir(gitDataPath))

	// serve requests for /.git/ repo's
	publicResourcesHandler := http.StripPrefix("/public/", http.FileServer(http.Dir(publicResourcesPath)))

	indexHandler := func(w http.ResponseWriter, r *http.Request) {
		tmplIndex.Execute(w, &dataIndex{
			Header: &dataHeader{
				Title: "Link to a branch/commit.",
			},
		})
	}

	statsHandler := func(w http.ResponseWriter, r *http.Request) {
		tmplStats.Execute(w, &dataStats{
			Header: &dataHeader{
				Title: "Statistics",
			},
			TotalReporefs: 42,
		})
	}

	reporefHandler := func(w http.ResponseWriter, r *http.Request) {
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

		// Request is for the .git repository.
		if strings.HasPrefix(r.RequestURI, "/"+rr.identifier+"/.git/") {
			gitDataHandler.ServeHTTP(w, r)
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
			indexHandler(w, r)
			return
		}

		fields := strings.SplitN(r.RequestURI[1:], "/", 2)
		switch fields[0] {
		case "github.com":
			reporefHandler(w, r)
		case "public":
			publicResourcesHandler.ServeHTTP(w, r)
		case "about":
			http.Redirect(w, r, "https://github.com/GeertJohan/reporef", 307)
		case "stats":
			statsHandler(w, r)
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
