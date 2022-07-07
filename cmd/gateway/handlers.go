package main

import (
	"fmt"
	"net/http"
)

func handlerGetBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		clientErr(w, http.StatusMethodNotAllowed, "")
		return
	}

	// If not parameters are given, default to the latest block.
	if len(r.URL.Query()) == 0 {
		w.Write([]byte("Default to latest block."))
		return
	}

	// Exception list of expected parameters. A map is used to overcome
	// the lack of a set data structure in the standard library.
	allowed := map[string]bool{"blockHash": true, "blockNumber": true}
	for param := range r.URL.Query() {
		if !allowed[param] {
			message := fmt.Sprintf(
				`Query fields should be a subset of {'blockHash', 'blockNumber}; `+
					`got: {%q}.`,
				param)
			clientErr(w, http.StatusBadRequest, message)
			return
		}
	}

	if r.URL.Query().Has("blockHash") && r.URL.Query().Has("blockNumber") {
		// Requires one of.
		message := fmt.Sprintf(
			`Exactly one identifier - blockNumber or blockHash, have to be `+
				`specified; got: blockNumber=%s, blockHash=%s.`,
			r.URL.Query().Get("blockNumber"),
			r.URL.Query().Get("blockHash"))
		clientErr(w, http.StatusBadRequest, message)
		return
	}

	switch {
	case r.URL.Query().Has("blockHash"):
		// TODO: Get block by hash.
		fmt.Fprintf(w, "Get block by hash, %s.", r.URL.Query().Get("blockHash"))
	case r.URL.Query().Has("blockNumber"):
		// TODO: Get block by number.
		fmt.Fprintf(w, "Get block by number, %s.", r.URL.Query().Get("blockNumber"))
	}
}

func handlerNotFound(w http.ResponseWriter, r *http.Request) {
	clientErr(w, http.StatusNotFound, "")
}
