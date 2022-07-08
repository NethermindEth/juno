package main

import (
	"fmt"
	"net/http"
	"strings"
)

// logRequest records the IP address, HTTP method, and URL accessed by
// the user.
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent CWE-117 log injection attack.
		// https://cwe.mitre.org/data/definitions/117.html.
		uri := fmt.Sprintf("%s", r.URL.RequestURI())
		escaped := strings.Replace(strings.Replace(uri, "\n", "", -1), "\r", "", -1)
		logInfo.Printf("%s %s %s %s", r.RemoteAddr, r.Proto, r.Method, escaped)
		next.ServeHTTP(w, r)
	})
}

// recoverPanic wraps the server in middleware that will ensure that a
// connection is closed in case a panic occurs.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				serverErr(w, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
