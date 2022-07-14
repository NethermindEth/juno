package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/types"
)

const (
	blockNotFound             = "StarknetErrorCode.BLOCK_NOT_FOUND"
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
		notImplementedErr(w)
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

		block, err := services.BlockService.GetBlockByHash(types.HexToBlockHash(hash))
		if err != nil {
			msg := fmt.Sprintf("Block hash %s does not exist.", hash)
			clientErr(w, http.StatusNotFound, blockNotFound, msg)
			return
		}

		res, err := json.MarshalIndent(&block, "", "  ")
		if err != nil {
			serverErr(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(res)
	case r.URL.Query().Has("blockNumber"):
		num, err := strconv.Atoi(r.URL.Query().Get("blockNumber"))
		if err != nil {
			// Another departure from the feeder gateway in terms of the error
			// message which seems to be the result of Python's failure to
			// interpret string characters as integers.
			clientErr(w, http.StatusBadRequest, malformedRequest, "Unexpected input")
			return
		}

		block, err := services.BlockService.GetBlockByNumber(uint64(num))
		if err != nil {
			msg := fmt.Sprintf("Block number %d was not found.", num)
			clientErr(w, http.StatusNotFound, blockNotFound, msg)
			return
		}

		res, err := json.MarshalIndent(block, "", "  ")
		if err != nil {
			serverErr(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(res)
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

		// The behaviour here is a bit confusing. The JSON-RPC specification
		// describes the block_id as one of the following:
		//
		//   1. block_hash.
		//   2. block_number.
		//   3. "latest" xor "pending".
		//
		// but the feeder gateway only seems to accept an id which
		// represents a block number.
		//     Furthermore, on Goerli, the block hash returned may reference
		// a block that has been reverted for example query [1] which when
		// its output is used as an input in get_block, [2], returns a
		// reverted block.
		//
		// - [1] https://alpha4.starknet.io/feeder_gateway/get_block_hash_by_id?blockId=1
		// - [2] https://alpha4.starknet.io/feeder_gateway/get_block?blockHash=0xb679258302fe1c8c771ceff6842a39b7f59b3f20a8c9e247648b0dac4106bf
		//
		// This implementation will just return the hash of the block
		// accepted on Ethereum queried by number.

		block, err := services.BlockService.GetBlockByNumber(uint64(id))
		if err != nil {
			msg := fmt.Sprintf("Block id %d was not found.", id)
			clientErr(w, http.StatusNotFound, blockNotFound, msg)
			return
		}

		fmt.Fprintf(w, "%q", block.BlockHash.Hex())
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

		block, err := services.BlockService.GetBlockByHash(types.HexToBlockHash(hash))
		if err != nil {
			msg := fmt.Sprintf("Block hash %s was not found.", hash)
			clientErr(w, http.StatusNotFound, blockNotFound, msg)
			return
		}

		fmt.Fprintf(w, "%d", block.BlockNumber)
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

	// FIX: hex.DecodeString returns an error if the string does not have
	// an even amount of characters. In the test cases for
	// services.StateService.GetCode is error is explicitly ignored so its
	// not clear how to compose the query.
	/*
		addr := strings.TrimPrefix(r.URL.Query().Get("contractAddress"), "0x")
		encoded, err := hex.DecodeString(addr)
		if err != nil {
			serverErr(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		code, err := services.StateService.GetCode(encoded)
		if err != nil {
			// The feeder gateway returns an empty JSON struct instead of a
			// not found error when the code does not lie in the database.
			code = &state.Code{}
		}

		res, err := json.MarshalIndent(code, "", "  ")
		if err != nil {
			serverErr(w, err)
			return
		}

		w.Write(res)
	*/
}

func handlerGetContractAddresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		clientErr(w, http.StatusMethodNotAllowed, "", "")
		return
	}

	// TODO: Return the address of the StarkNet and GPS Statement Verifier
	// contracts on Ethereum.
	notImplementedErr(w)
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
			w,
			http.StatusBadRequest,
			malformedRequest,
			"Key not found: 'contractAddress'",
		)
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

		// XXX: See comment in case below for issues relating to retrieving
		// the contract storage.

		// TODO: Fetch the value of the storage slot at the given key
		// relative to the given block specified by block hash.
		notImplementedErr(w)
	case r.URL.Query().Has("blockNumber"):
		_, err := strconv.Atoi(r.URL.Query().Get("blockNumber"))
		if err != nil {
			// Another departure from the feeder gateway in terms of the error
			// message which seems to be the result of Python's failure to
			// interpret string characters as integers.
			clientErr(w, http.StatusBadRequest, malformedRequest, "Unexpected input")
			return
		}

		// FIX: services.StateService.GetStorage does not seem to be
		// returning anything at all. Not even an error.
		/*
			addr := r.URL.Query().Get("contractAddress")
			storage, err := services.StateService.GetStorage(addr, uint64(num))
			if err != nil {
				// Implies empty storage slot.
				w.Write([]byte("0x0"))
				return
			}

			raw := fmt.Sprintf("%v", storage)
			w.Write([]byte(raw))
			return
		*/

		// TODO: Fetch the value of the storage slot at the given key
		// relative to the given block specified by block number.
		notImplementedErr(w)
	}
	// TODO: Default to getting slot relative to latest block.
	notImplementedErr(w)
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

		w.Header().Set("Content-Type", "application/json")

		trimmed := strings.TrimPrefix(hash, "0x")
		tx, err := services.TransactionService.GetTransaction(types.HexToTransactionHash(trimmed))
		if err != nil {
			w.Write([]byte(`{ "status": "NOT_RECEIVED" }`))
			return
		}

		res, err := json.MarshalIndent(tx, "", "  ")
		if err != nil {
			serverErr(w, err)
			return
		}

		w.Write(res)
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
		_, err := strconv.Atoi(r.URL.Query().Get("transactionId"))
		if err != nil {
			// Another departure from the feeder gateway in terms of the error
			// message which seems to be the result of Python's failure to
			// interpret string characters as integers.
			clientErr(w, http.StatusBadRequest, malformedRequest, "Unexpected input")
			return
		}

		// TODO: Fetch transaction from database using its ID. Currently
		// this information does not seem to be stored locally.
		notImplementedErr(w)
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
		// Currently this information does not appear to be stored locally.
		notImplementedErr(w)
		return
	}

	panic("Reached code marked as unreachable.")
}

func handlerNotFound(w http.ResponseWriter, r *http.Request) {
	clientErr(w, http.StatusNotFound, "", "")
}
