package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handlerNotFound)
	mux.HandleFunc("/v1/get_block", handlerGetBlock)

	log.Println("serving on http://localhost:4000")
	log.Fatal(http.ListenAndServe(":4000", mux))
}
