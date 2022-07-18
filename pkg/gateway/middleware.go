package gateway

import (
	"fmt"
	"net/http"
	"strings"

	. "github.com/NethermindEth/juno/internal/log"
)

// logRequest records the IP address, HTTP method, and URL accessed by
// the user.
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent CWE-117 log injection attack. See the following for
		// details https://cwe.mitre.org/data/definitions/117.html.
		uri := fmt.Sprintf("%s", r.URL.RequestURI())
		escURI := strings.Replace(strings.Replace(uri, "\n", "", -1), "\r", "", -1)
		Logger.With("protocol", r.Proto, "method", r.Method, "uri", escURI).Info("API request")

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
