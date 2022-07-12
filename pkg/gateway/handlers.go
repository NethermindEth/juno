package gateway

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
)

const (
	malformedRequest          = "StarkErrorCode.MALFORMED_REQUEST"
	outOfRangeBlockHash       = "StarknetErrorCode.OUT_OF_RANGE_BLOCK_HASH"
	outOfRangeContractAddr    = "StarknetErrorCode.OUT_OF_RANGE_CONTRACT_ADDRESS"
	outOfRangeTransactionHash = "StarknetErrorCode.OUT_OF_RANGE_TRANSACTION_HASH"
)

func handlerGetBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		clientErr(w, http.StatusMethodNotAllowed, "", "")
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
			msg := fmt.Sprintf(
				`Query fields should be a subset of {'blockHash', 'blockNumber'}; `+
					`got: {%q}.`,
				param)
			clientErr(w, http.StatusBadRequest, malformedRequest, msg)
			return
		}
	}

	// Requires one of.
	if r.URL.Query().Has("blockHash") && r.URL.Query().Has("blockNumber") {
		msg := fmt.Sprintf(
			`Exactly one identifier - blockNumber or blockHash, have to be `+
				`specified; got: blockNumber=%s, blockHash=%s.`,
			r.URL.Query().Get("blockNumber"),
			r.URL.Query().Get("blockHash"))
		clientErr(w, http.StatusBadRequest, malformedRequest, msg)
		return
	}

	switch {
	case r.URL.Query().Has("blockHash"):
		hash := r.URL.Query().Get("blockHash")
		if err := isValid(hash); err != nil {
			if errors.Is(err, errInvalidHex) {
				msg := fmt.Sprintf(
					"Block hash should be a hexadecimal string starting with 0x, or "+
						"'null'; got: %s.", hash)
				clientErr(w, http.StatusBadRequest, malformedRequest, msg)
				return
			}
			// Only other error returned by isValid is an out of range felt.
			msg := fmt.Sprintf("Block hash %s is out of range", hash)
			clientErr(w, http.StatusBadRequest, outOfRangeBlockHash, msg)
			return
		}

		// TODO: Fetch block from database using its hash.
		fmt.Fprintf(w, "Get block by hash: %s.", hash)
	case r.URL.Query().Has("blockNumber"):
		num, err := strconv.Atoi(r.URL.Query().Get("blockNumber"))
		if err != nil {
			// Another departure from the feeder gateway in terms of the error
			// message which seems to be the result of Python's failure to
			// interpret string characters as integers.
			clientErr(w, http.StatusBadRequest, malformedRequest, "Unexpected input")
			return
		}

		// TODO: Fetch block from database using its number.
		fmt.Fprintf(w, "Get block by number: %d.", num)
	}
}

func handlerGetBlockHashByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		clientErr(w, http.StatusMethodNotAllowed, "", "")
		return
	}

	if len(r.URL.Query()) == 0 {
		clientErr(w, http.StatusBadRequest, malformedRequest, "Key not found: 'blockId'")
		return
	}

	if r.URL.Query().Has("blockId") {
		id, err := strconv.Atoi(r.URL.Query().Get("blockId"))
		if err != nil {
			// Another departure from the feeder gateway in terms of the error
			// message which seems to be the result of Python's failure to
			// interpret string characters as integers.
			clientErr(w, http.StatusBadRequest, malformedRequest, "Unexpected input")
			return
		}

		// TODO: Fetch block hash from database using its ID.
		fmt.Fprintf(w, "Get block hash by id: %d.", id)
		return
	}
	clientErr(w, http.StatusBadRequest, malformedRequest, "Key not found: 'blockId'")
}

func handlerGetBlockIDByHash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		clientErr(w, http.StatusMethodNotAllowed, "", "")
		return
	}

	if len(r.URL.Query()) == 0 {
		clientErr(w, http.StatusBadRequest, malformedRequest, "Block hash must be given.")
		return
	}

	if r.URL.Query().Has("blockHash") {
		hash := r.URL.Query().Get("blockHash")
		if err := isValid(hash); err != nil {
			if errors.Is(err, errInvalidHex) {
				msg := fmt.Sprintf(
					"Block hash should be a hexadecimal string starting with 0x, or "+
						"'null'; got: %s.", hash)
				clientErr(w, http.StatusBadRequest, malformedRequest, msg)
				return
			}
			// Only other error returned by isValid is an out of range felt.
			msg := fmt.Sprintf("Block hash %s is out of range", hash)
			clientErr(w, http.StatusBadRequest, outOfRangeBlockHash, msg)
			return
		}

		// TODO: Fetch block id from database using its hash.
		fmt.Fprintf(w, "Get block id by hash: %s.", hash)
		return
	}
	clientErr(w, http.StatusBadRequest, malformedRequest, "Block hash must be given.")
}

func handlerGetCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		clientErr(w, http.StatusMethodNotAllowed, "", "")
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
			msg := fmt.Sprintf(
				`Query fields should be a subset of {'blockHash', 'blockNumber', `+
					`'contractAddress'}; got: {%q}.`,
				param)
			clientErr(w, http.StatusBadRequest, malformedRequest, msg)
			return
		}
	}

	// Requires one of.
	if r.URL.Query().Has("blockHash") && r.URL.Query().Has("blockNumber") {
		msg := fmt.Sprintf(
			`Exactly one identifier - blockNumber or blockHash, have to be `+
				`specified; got: blockNumber=%s, blockHash=%s.`,
			r.URL.Query().Get("blockNumber"),
			r.URL.Query().Get("blockHash"))
		clientErr(w, http.StatusBadRequest, malformedRequest, msg)
		return
	}

	if r.URL.Query().Has("contractAddress") {
		// Validate contract address.
		addr := r.URL.Query().Get("contractAddress")
		if err := isValid(addr); err != nil {
			// Difference is that the feeder gateway returns a JSON with empty
			// fields for in invalid hexadecimal string input instead of an
			// error as below.
			if errors.Is(err, errInvalidHex) {
				msg := fmt.Sprintf(
					"Contract address should be a hexadecimal string starting with 0x, or "+
						"'null'; got: %s.", addr)
				clientErr(w, http.StatusBadRequest, malformedRequest, msg)
				return
			}
			// Only other error returned by isValid is an out of range felt.
			msg := fmt.Sprintf("Invalid contract address: %s is out of range.", addr)
			clientErr(w, http.StatusBadRequest, outOfRangeContractAddr, msg)
			return
		}
	} else {
		msg := "Key not found: 'contractAddress'"
		clientErr(w, http.StatusBadRequest, malformedRequest, msg)
		return
	}

	switch {
	case r.URL.Query().Has("blockHash"):
		hash := r.URL.Query().Get("blockHash")
		if err := isValid(hash); err != nil {
			if errors.Is(err, errInvalidHex) {
				msg := fmt.Sprintf(
					"Block hash should be a hexadecimal string starting with 0x, or "+
						"'null'; got: %s.", hash)
				clientErr(w, http.StatusBadRequest, malformedRequest, msg)
				return
			}
			// Only other error returned by isValid is an out of range felt.
			msg := fmt.Sprintf("Block hash %s is out of range", hash)
			clientErr(w, http.StatusBadRequest, outOfRangeBlockHash, msg)
			return
		}

		// TODO: Fetch code from database relative to the given block hash.
		fmt.Fprintf(w, "Get code relative to block hash: %s.", hash)
	case r.URL.Query().Has("blockNumber"):
		num, err := strconv.Atoi(r.URL.Query().Get("blockNumber"))
		if err != nil {
			// Another departure from the feeder gateway in terms of the error
			// message which seems to be the result of Python's failure to
			// interpret string characters as integers.
			clientErr(w, http.StatusBadRequest, malformedRequest, "Unexpected input")
			return
		}

		// TODO: Fetch code from database relative to the given block
		// number.
		fmt.Fprintf(w, "Get code relative to block number: %d.", num)
	}
}

func handlerGetContractAddresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		clientErr(w, http.StatusMethodNotAllowed, "", "")
		return
	}

	// TODO: Return the address of the StarkNet and GPS Statement Verifier
	// contracts on Ethereum.
	w.Write([]byte("Get the contract address of StarkNet and the GpsStatementVerifier on Ethereum."))
}

func handlerGetStorageAt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		clientErr(w, http.StatusMethodNotAllowed, "", "")
		return
	}

	switch query := r.URL.Query(); {
	case len(query) == 0, !query.Has("key"):
		clientErr(w, http.StatusBadRequest, malformedRequest, "Key not found: 'key'")
		return
	case !query.Has("contractAddress"):
		clientErr(
			w, http.StatusBadRequest, malformedRequest, "Key not found: 'contractAddress'")
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
			msg := fmt.Sprintf(
				`Query fields should be a subset of {'blockHash', 'blockNumber', `+
					`'contractAddress', 'key'}; got: {%q}.`,
				param)
			clientErr(w, http.StatusBadRequest, malformedRequest, msg)
			return
		}
	}

	// Requires one of.
	if r.URL.Query().Has("blockHash") && r.URL.Query().Has("blockNumber") {
		msg := fmt.Sprintf(
			`Exactly one identifier - blockNumber or blockHash, have to be `+
				`specified; got: blockNumber=%s, blockHash=%s.`,
			r.URL.Query().Get("blockNumber"),
			r.URL.Query().Get("blockHash"))
		clientErr(w, http.StatusBadRequest, malformedRequest, msg)
		return
	}

	if r.URL.Query().Has("contractAddress") {
		// Validate contract address.
		addr := r.URL.Query().Get("contractAddress")
		if err := isValid(addr); err != nil {
			// Difference is that the feeder gateway returns a JSON with empty
			// fields for in invalid hexadecimal string input instead of an
			// error as below.
			if errors.Is(err, errInvalidHex) {
				msg := fmt.Sprintf(
					"Contract address should be a hexadecimal string starting with 0x, or "+
						"'null'; got: %s.", addr)
				clientErr(w, http.StatusBadRequest, malformedRequest, msg)
				return
			}
			// Only other error returned by isValid is an out of range felt.
			msg := fmt.Sprintf("Invalid contract address: %s is out of range.", addr)
			clientErr(w, http.StatusBadRequest, outOfRangeContractAddr, msg)
			return
		}
	} else {
		msg := "Key not found: 'contractAddress'"
		clientErr(w, http.StatusBadRequest, malformedRequest, msg)
		return
	}

	if r.URL.Query().Has("key") {
		key := r.URL.Query().Get("key")
		_, ok := new(big.Int).SetString(key, 10)
		if !ok {
			msg := fmt.Sprintf("invalid literal for int() with base 10 : '%s'", key)
			clientErr(w, http.StatusBadRequest, malformedRequest, msg)
			return
		}
	} else {
		msg := "Key not found: 'key'"
		clientErr(w, http.StatusBadRequest, malformedRequest, msg)
		return
	}

	switch {
	case r.URL.Query().Has("blockHash"):
		hash := r.URL.Query().Get("blockHash")
		if err := isValid(hash); err != nil {
			if errors.Is(err, errInvalidHex) {
				msg := fmt.Sprintf(
					"Block hash should be a hexadecimal string starting with 0x, or "+
						"'null'; got: %s.", hash)
				clientErr(w, http.StatusBadRequest, malformedRequest, msg)
				return
			}
			// Only other error returned by isValid is an out of range felt.
			msg := fmt.Sprintf("Block hash %s is out of range", hash)
			clientErr(w, http.StatusBadRequest, outOfRangeBlockHash, msg)
			return
		}

		// TODO: Fetch the value of the storage slot at the given key
		// relative to the given block specified by block hash.
		fmt.Fprintf(
			w,
			"Get value of storage slot %s at contract %s relative to block (hash) %s.",
			r.URL.Query().Get("key"),
			r.URL.Query().Get("contractAddress"),
			hash,
		)
	case r.URL.Query().Has("blockNumber"):
		num, err := strconv.Atoi(r.URL.Query().Get("blockNumber"))
		if err != nil {
			// Another departure from the feeder gateway in terms of the error
			// message which seems to be the result of Python's failure to
			// interpret string characters as integers.
			clientErr(w, http.StatusBadRequest, malformedRequest, "Unexpected input")
			return
		}

		// TODO: Fetch the value of the storage slot at the given key
		// relative to the given block specified by block number.
		fmt.Fprintf(
			w,
			"Get value of storage slot %s at contract %s relative to block (number) %d.",
			r.URL.Query().Get("key"),
			r.URL.Query().Get("contractAddress"),
			num,
		)
	}
}

func handlerGetTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		clientErr(w, http.StatusMethodNotAllowed, "", "")
		return
	}

	if len(r.URL.Query()) == 0 {
		msg := "Transaction hash should be a hexadecimal string starting with " +
			"0x; got None."
		clientErr(w, http.StatusBadRequest, malformedRequest, msg)
		return
	}

	// Slight difference here is that the feeder will collect all
	// parameters and specify which are incorrect in the error message
	// whereas this implementation will send an error at the first
	// occurrence of an invalid parameter without checking the rest.
	for param := range r.URL.Query() {
		if param != "transactionHash" {
			msg := fmt.Sprintf(
				`Query fields should be a subset of {'transactionHash'}; got: {%q}.`,
				param)
			clientErr(w, http.StatusBadRequest, malformedRequest, msg)
			return
		}
	}

	if r.URL.Query().Has("transactionHash") {
		hash := r.URL.Query().Get("transactionHash")
		if err := isValid(hash); err != nil {
			if errors.Is(err, errInvalidHex) {
				msg := fmt.Sprintf(
					"Transaction hash should be a hexadecimal string starting with 0x, "+
						"or 'null'; got: %s.", hash)
				clientErr(w, http.StatusBadRequest, malformedRequest, msg)
				return
			}
			// Only other error returned by isValid is an out of range felt.
			msg := fmt.Sprintf("Transaction hash %s is out of range", hash)
			clientErr(w, http.StatusBadRequest, outOfRangeTransactionHash, msg)
			return
		}

		// TODO: Fetch transaction from database using its hash.
		fmt.Fprintf(w, "Get transaction by hash : %s.", hash)
		return
	}

	panic("Reached code marked as unreachable.")
}

func handlerGetTransactionHashByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		clientErr(w, http.StatusMethodNotAllowed, "", "")
		return
	}

	if len(r.URL.Query()) == 0 {
		clientErr(
			w, http.StatusBadRequest, malformedRequest, "Key not found: 'transactionId'")
		return
	}

	if r.URL.Query().Has("transactionId") {
		id, err := strconv.Atoi(r.URL.Query().Get("transactionId"))
		if err != nil {
			// Another departure from the feeder gateway in terms of the error
			// message which seems to be the result of Python's failure to
			// interpret string characters as integers.
			clientErr(w, http.StatusBadRequest, malformedRequest, "Unexpected input")
			return
		}

		// TODO: Fetch transaction from database using its ID.
		fmt.Fprintf(w, "Get transaction by id: %d.", id)
		return
	}
	clientErr(w, http.StatusBadRequest, malformedRequest, "Key not found: 'transactionId'")
}

func handlerGetTransactionIDByHash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		clientErr(w, http.StatusMethodNotAllowed, "", "")
		return
	}

	if len(r.URL.Query()) == 0 {
		msg := "Transaction hash should be a hexadecimal string starting with " +
			"0x; got None."
		clientErr(w, http.StatusBadRequest, malformedRequest, msg)
		return
	}

	// Slight difference here is that the feeder will collect all
	// parameters and specify which are incorrect in the error message
	// whereas this implementation will send an error at the first
	// occurrence of an invalid parameter without checking the rest.
	for param := range r.URL.Query() {
		if param != "transactionHash" {
			msg := fmt.Sprintf(
				"Query fields should be a subset of {'transactionHash'}; got: {%q}.",
				param)
			clientErr(w, http.StatusBadRequest, malformedRequest, msg)
			return
		}
	}

	if r.URL.Query().Has("transactionHash") {
		hash := r.URL.Query().Get("transactionHash")
		if err := isValid(hash); err != nil {
			if errors.Is(err, errInvalidHex) {
				msg := fmt.Sprintf(
					"Transaction hash should be a hexadecimal string starting with 0x, "+
						"or 'null'; got: %s.", hash)
				clientErr(w, http.StatusBadRequest, malformedRequest, msg)
				return
			}
			// Only other error returned by isValid is an out of range felt.
			msg := fmt.Sprintf("Transaction hash %s is out of range", hash)
			clientErr(w, http.StatusBadRequest, outOfRangeTransactionHash, msg)
			return
		}

		// TODO: Fetch transaction id from database using its hash.
		fmt.Fprintf(w, "Get transaction id by hash: %s.", hash)
		return
	}

	panic("Reached code marked as unreachable.")
}

func handlerNotFound(w http.ResponseWriter, r *http.Request) {
	clientErr(w, http.StatusNotFound, "", "")
}
