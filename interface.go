package main

import (
	"html/template"
	"net/http"
)

var (
	tmplIndex    = template.Must(template.ParseFiles(templatesPath+"index.html", templatesPath+"header.html", templatesPath+"footer.html"))
	tmplReporef  = template.Must(template.ParseFiles(templatesPath+"reporef.html", templatesPath+"header.html", templatesPath+"footer.html"))
	tmplStats    = template.Must(template.ParseFiles(templatesPath+"stats.html", templatesPath+"header.html", templatesPath+"footer.html"))
	tmplNotFound = template.Must(template.ParseFiles(templatesPath+"notFound.html", templatesPath+"header.html", templatesPath+"footer.html"))

	// This is now the 404 page..
	// TODO: Fix the NotFound template to be one page, no external loads.. see TODO in the NotFound function.
	tmpl404 = template.Must(template.ParseFiles(templatesPath + "404.html"))

	// Simple template with go-get meta tag
	tmplGoGet = template.Must(template.ParseFiles(templatesPath + "goGet.html"))
)

type dataHeader struct {
	Title string
}

type dataFooter struct {
}

type dataIndex struct {
	Header *dataHeader
	Footer *dataFooter
	Text   string
}

type dataReporef struct {
	Header           *dataHeader
	Footer           *dataFooter
	Identifier       string
	OriginalRepoPath string
	RefType          string
	Ref              string
}

type dataStats struct {
	Header        *dataHeader
	Footer        *dataFooter
	TotalReporefs int
}

type dataNotFound struct {
	Header    *dataHeader
	Footer    *dataFooter
	ExtraInfo string
}

type dataGoGet struct {
	Identifier string
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	// // Set status header to 404 not found
	// w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// w.WriteHeader(http.StatusNotFound)

	// // Write 404 template
	// tmplNotFound.Execute(w, &dataNotFound{
	// 	Header: &dataHeader{
	// 		Title: "404 Page not found.",
	// 	},
	// 	ExtraInfo: "Thats all we know...",
	// })
	//
	// TODO: 404 that looks like the rest of the site.
	//       Make it one page though: https://github.com/styleguide/templates/2.0

	tmpl404.Execute(w, nil)
}
