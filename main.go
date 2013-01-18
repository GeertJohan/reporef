package main

import (
	"html/template"
	"log"
	"net/http"
	"strings"
)

var tmplSimplePage *template.Template

type dataSimplePage struct {
	Identifier       string
	OriginalRepoPath string
	RefType          string
	Ref              string
}

func init() {
	var err error
	tmplSimplePage, err = template.New("simplePage").Parse(`
<html>
	<head>
		<meta charset="utf-8">
		<meta name="go-import" content="localhost.com/{{.Identifier}} git http://localhost.com/{{.Identifier}}/.git/">
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
}

func main() {

	// serve requests for the /git dir
	gitHttpHandler := http.FileServer(http.Dir(localDataPath))
	// http.HandleFunc("/git/", func(w http.ResponseWriter, r *http.Request) {
	// 	log.Printf("/git/ request: %s\n", r.RequestURI)
	// 	gitHttpHandler.ServeHTTP(w, r)
	// })

	// serve any request in the root (serve website with meta tag for go-get, redirecting to /git/)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/" {
			http.Redirect(w, r, "github.com/GeertJohan/reporef", 307)
		}
		log.Println("request on", r.RequestURI)
		rr, err := reporefFromRequestURI(r.RequestURI)
		if err != nil {
			log.Printf("Error: %s\n", err)
			return
		}

		done, err := rr.updateRepositoryIfNeeded()
		if err != nil {
			log.Printf("Error: %s\n", err)
			return
		}
		if done {
			log.Println("updated!")
		} else {
			log.Println("not updated")
		}

		if strings.HasPrefix(r.RequestURI, "/"+rr.identifier+"/.git/") {
			gitHttpHandler.ServeHTTP(w, r)
			return
		}

		pageData := &dataSimplePage{
			Identifier:       rr.identifier,
			OriginalRepoPath: rr.originalRepoPath,
			RefType:          string(rr.refType),
			Ref:              rr.ref,
		}

		tmplSimplePage.Execute(w, pageData)
	})

	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal(err)
	}
}
