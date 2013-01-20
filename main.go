package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
)

var (
	tmplSimplePage *template.Template
	tmplGoGet      *template.Template
)

type dataTmplSimplePage struct {
	Identifier       string
	OriginalRepoPath string
	RefType          string
	Ref              string
}

type dataTmplGoGet struct {
	Identifier string
}

func init() {
	var err error
	tmplSimplePage, err = template.New("simplePage").Parse(`
<html>
	<head>
		<meta charset="utf-8">
		<title>RepoRef: {{.Identifier}}</title>
	</head>
	<body>
		Thank you for using {{.OriginalRepoPath}} at {{.RefType}} '{{.Ref}}'.<br/>
		The reporef service is experimental and under heavy development. Please contribute at github.com/GeertJohan/reporef.<br/>
	</body>
</html>
`)
	if err != nil {
		panic(err)
	}

	tmplGoGet, err = template.New("goGet").Parse(`
<html>
	<head>
		<meta charset="utf-8">
		<meta name="go-import" content="go.reporef.org/{{.Identifier}} git http://go.reporef.org/{{.Identifier}}/.git/">
		<meta HTTP-EQUIV="REFRESH" content="0; url=http://go.reporef.org/{{.Identifier}}">
		<title>RepoRef go-get: {{.Identifier}}</title>
	</head>
	<body>
		This is the go-get page for go.reporef.org/{{.Identifier}}
		You are being redirected to the human-readable page.
	</body>
</html>
`)
	if err != nil {
		panic(err)
	}
}

func main() {

	// serve requests for /.git/ repo's
	gitHttpHandler := http.FileServer(http.Dir(localDataPath))

	statsHandler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Not available yet.")
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
			gitHttpHandler.ServeHTTP(w, r)
			return
		}

		// go-get page with meta tag
		if r.FormValue("go-get") == "1" {
			pageData := &dataTmplGoGet{rr.identifier}
			tmplGoGet.Execute(w, pageData)
			return
		}

		// Information page
		pageData := &dataTmplSimplePage{
			Identifier:       rr.identifier,
			OriginalRepoPath: rr.originalRepoPath,
			RefType:          string(rr.refType),
			Ref:              rr.ref,
		}
		tmplSimplePage.Execute(w, pageData)
	}

	// Manual mux for all requests
	rootHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/" {
			// There's no index or home page yet. Redirect to reporef github project.
			http.Redirect(w, r, "https://github.com/GeertJohan/reporef", 307)
		}

		fields := strings.SplitN(r.RequestURI[1:], "/", 1)
		switch fields[0] {
		case "stats":
			statsHandler(w, r)
		case "github.com":
			reporefHandler(w, r)
		default:
			http.NotFound(w, r)
		}
	}

	// serve any request in the root (serve website with meta tag for go-get, redirecting to /git/)
	http.HandleFunc("/", rootHandler)

	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal(err)
	}
}
