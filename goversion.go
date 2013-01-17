package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rr, err := reporefFromRequestURI(r.RequestURI)
		if err != nil {
			log.Printf("Error %s\n", err)
			return
		}

		if r.Header.Get("go-get") == "1" {
			done, err := rr.updateRepositoryIfNeeded()
			if err != nil {
				log.Printf("Error: %s\n", err)
				return
			}
			if done {
				log.Println("done!")
			}
		} else {
			fmt.Fprint(w, "Thank you for requesting ")
		}
	})

	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal(err)
	}
}
