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
		// TODO: Return the latest block.
		w.Write([]byte("Default to latest block."))
		return
	}

	// Slight difference here is that the feeder will collect all
	// parameters and specify which are incorrect in the error message
	// whereas this implementation will send an error at the first
	// occurrence of an invalid parameter without checking the rest.
	allowed := map[string]bool{"blockHash": true, "blockNumber": true}
	for param := range r.URL.Query() {
		if !allowed[param] {
			message := fmt.Sprintf(
				`Query fields should be a subset of {'blockHash', 'blockNumber'}; `+
					`got: {%q}.`,
				param)
			clientErr(w, http.StatusBadRequest, message)
			return
		}
	}

	// Requires one of.
	if r.URL.Query().Has("blockHash") && r.URL.Query().Has("blockNumber") {
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
		// TODO: Fetch block from database using its hash.
		fmt.Fprintf(w, "Get block by hash: %s.", r.URL.Query().Get("blockHash"))
	case r.URL.Query().Has("blockNumber"):
		// TODO: Fetch block from database using its number.
		fmt.Fprintf(w, "Get block by number: %s.", r.URL.Query().Get("blockNumber"))
	}
}

func handlerGetBlockHashByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		clientErr(w, http.StatusMethodNotAllowed, "")
		return
	}

	if len(r.URL.Query()) == 0 {
		clientErr(w, http.StatusBadRequest, "Key not found: 'blockId'")
		return
	}

	if r.URL.Query().Has("blockId") {
		// TODO: Fetch block hash from database using its ID.
		fmt.Fprintf(w, "Get block hash by id: %s.", r.URL.Query().Get("blockId"))
		return
	}
	clientErr(w, http.StatusBadRequest, "Key not found: 'blockId'")
}

func handlerGetBlockIDByHash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		clientErr(w, http.StatusMethodNotAllowed, "")
		return
	}

	if len(r.URL.Query()) == 0 {
		clientErr(w, http.StatusBadRequest, "Block hash must be given.")
		return
	}

	if r.URL.Query().Has("blockHash") {
		// TODO: Fetch block id from database using its hash.
		fmt.Fprintf(w, "Get block id by hash: %s.", r.URL.Query().Get("blockHash"))
		return
	}
	clientErr(w, http.StatusBadRequest, "Block hash must be given.")
}

func handlerGetCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		clientErr(w, http.StatusMethodNotAllowed, "")
		return
	}

	// Slight difference here is that the feeder will collect all
	// parameters and specify which are incorrect in the error message
	// whereas this implementation will send an error at the first
	// occurrence of an invalid parameter without checking the rest.
	allowed := map[string]bool{
		"blockHash": true, "blockNumber": true, "contractAddress": true,
	}
	for param := range r.URL.Query() {
		if !allowed[param] {
			message := fmt.Sprintf(
				`Query fields should be a subset of {'blockHash', 'blockNumber', `+
					`'contractAddress'}; got: {%q}.`,
				param)
			clientErr(w, http.StatusBadRequest, message)
			return
		}
	}

	// Requires one of.
	if r.URL.Query().Has("blockHash") && r.URL.Query().Has("blockNumber") {
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
		// TODO: Fetch code from database relative to the given block hash.
		fmt.Fprintf(
			w, "Get code relative to block hash: %s.", r.URL.Query().Get("blockHash"))
	case r.URL.Query().Has("blockNumber"):
		// TODO: Fetch code from database relative to the given block
		// number.
		fmt.Fprintf(
			w, "Get code relative to block number: %s.", r.URL.Query().Get("blockHash"))
	}
}

func handlerGetContractAddresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		clientErr(w, http.StatusMethodNotAllowed, "")
		return
	}

	// TODO: Return the address of the StarkNet and GPS Statement Verifier
	// contracts on Ethereum.
	w.Write([]byte("Get the contract address of StarkNet and the GpsStatementVerifier on Ethereum."))
}

func handlerGetStorageAt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		clientErr(w, http.StatusMethodNotAllowed, "")
		return
	}

	switch query := r.URL.Query(); {
	case len(query) == 0, !query.Has("key"):
		clientErr(w, http.StatusBadRequest, "Key not found: 'key'")
		return
	case !query.Has("contractAddress"):
		clientErr(w, http.StatusBadRequest, "Key not found: 'contractAddress'")
		return
	}

	// Slight difference here is that the feeder will collect all
	// parameters and specify which are incorrect in the error message
	// whereas this implementation will send an error at the first
	// occurrence of an invalid parameter without checking the rest.
	allowed := map[string]bool{
		"blockHash":       true,
		"blockNumber":     true,
		"contractAddress": true,
		"key":             true,
	}
	for param := range r.URL.Query() {
		if !allowed[param] {
			message := fmt.Sprintf(
				`Query fields should be a subset of {'blockHash', 'blockNumber', `+
					`'contractAddress', 'key'}; got: {%q}.`,
				param)
			clientErr(w, http.StatusBadRequest, message)
			return
		}
	}

	// Requires one of.
	if r.URL.Query().Has("blockHash") && r.URL.Query().Has("blockNumber") {
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
		// TODO: Fetch the value of the storage slot at the given key
		// relative to the given block specified by block hash.
		fmt.Fprintf(
			w,
			"Get value of storage slot %s at contract %s relative to block (hash) %s.",
			r.URL.Query().Get("key"),
			r.URL.Query().Get("contractAddress"),
			r.URL.Query().Get("blockHash"),
		)
	case r.URL.Query().Has("blockNumber"):
		// TODO: Fetch the value of the storage slot at the given key
		// relative to the given block specified by block number.
		fmt.Fprintf(
			w,
			"Get value of storage slot %s at contract %s relative to block (number) %s.",
			r.URL.Query().Get("key"),
			r.URL.Query().Get("contractAddress"),
			r.URL.Query().Get("blockNumber"),
		)
	}
}

func handlerGetTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		clientErr(w, http.StatusMethodNotAllowed, "")
		return
	}

	if len(r.URL.Query()) == 0 {
		message := `Transaction hash should be a hexadecimal string starting ` +
			`with 0x; got None.`
		clientErr(w, http.StatusBadRequest, message)
		return
	}

	// Slight difference here is that the feeder will collect all
	// parameters and specify which are incorrect in the error message
	// whereas this implementation will send an error at the first
	// occurrence of an invalid parameter without checking the rest.
	for param := range r.URL.Query() {
		if param != "transactionHash" {
			message := fmt.Sprintf(
				`Query fields should be a subset of {'transactionHash'}; got: {%q}.`,
				param)
			clientErr(w, http.StatusBadRequest, message)
			return
		}
	}

	if r.URL.Query().Has("transactionHash") {
		// TODO: Fetch transaction from database using its hash.
		fmt.Fprintf(
			w, "Get transaction by hash : %s.", r.URL.Query().Get("transactionHash"))
		return
	}

	// XXX: Never reached?
	panic("Reached code marked as unreachable.")
}

func handlerGetTransactionHashByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		clientErr(w, http.StatusMethodNotAllowed, "")
		return
	}

	if len(r.URL.Query()) == 0 {
		clientErr(w, http.StatusBadRequest, "Key not found: 'transactionId'")
		return
	}

	if r.URL.Query().Has("transactionId") {
		// TODO: Fetch transaction from database using its ID.
		fmt.Fprintf(
			w, "Get transaction by id: %s.", r.URL.Query().Get("transactionId"))
		return
	}
	clientErr(w, http.StatusBadRequest, "Key not found: 'transactionId'")
}

func handlerGetTransactionIDByHash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		clientErr(w, http.StatusMethodNotAllowed, "")
		return
	}

	if len(r.URL.Query()) == 0 {
		message := `Transaction hash should be a hexadecimal string starting ` +
			`with 0x; got None.`
		clientErr(w, http.StatusBadRequest, message)
		return
	}

	// Slight difference here is that the feeder will collect all
	// parameters and specify which are incorrect in the error message
	// whereas this implementation will send an error at the first
	// occurrence of an invalid parameter without checking the rest.
	for param := range r.URL.Query() {
		if param != "transactionHash" {
			message := fmt.Sprintf(
				`Query fields should be a subset of {'transactionHash'}; got: {%q}.`,
				param)
			clientErr(w, http.StatusBadRequest, message)
			return
		}
	}

	if r.URL.Query().Has("transactionHash") {
		// TODO: Fetch transaction id from database using its hash.
		fmt.Fprintf(
			w, "Get transaction id by hash: %s.", r.URL.Query().Get("transactionHash"))
		return
	}

	// XXX: Never reached?
	panic("Reached code marked as unreachable.")
}

func handlerNotFound(w http.ResponseWriter, r *http.Request) {
	clientErr(w, http.StatusNotFound, "")
}
