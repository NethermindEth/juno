package main

import (
	"flag"
	"log"
	"net/http"
	"os"
)

var (
	logErr  = log.New(os.Stderr, "ERROR ", log.LstdFlags|log.Lshortfile)
	logInfo = log.New(os.Stdout, "INFO ", log.LstdFlags)
)

func main() {
	addr := flag.String("addr", ":4000", "local network address")
	flag.Parse()

	srv := &http.Server{
		Addr:     *addr,
		ErrorLog: logErr,
		Handler:  routes(),
	}

	logInfo.Printf("serving on http://localhost%s", *addr)
	logErr.Fatal(srv.ListenAndServe())
}
