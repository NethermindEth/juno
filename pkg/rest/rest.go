package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/NethermindEth/juno/pkg/feeder"
)

// GetBlock Returns Starknet Block
func (rh *RestHandler) GetBlock(w http.ResponseWriter, r *http.Request) {
	var (
		res *feeder.StarknetBlock
		err error
	)
	blockNumber, ok_blockNumber := r.URL.Query()["blockNumber"]
	_blockNumber := strings.Join(blockNumber, "")

	blockHash, ok_blockHash := r.URL.Query()["blockHash"]
	_blockHash := strings.Join(blockHash, "")

	if ok_blockNumber || ok_blockHash {
		res, err = rh.RestFeeder.GetBlock(_blockHash, _blockNumber)
		// test
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetBlock Request Failed: expected blockNumber or blockHash")
}

// GetCode returns CodeInfo using Block Identifier & Contract Address
func (rh *RestHandler) GetCode(w http.ResponseWriter, r *http.Request) {
	blockNumber, ok_blockNumber := r.URL.Query()["blockNumber"]
	var _blockNumber string
	if ok_blockNumber {
		_blockNumber = strings.Join(blockNumber, "")
	}
	blockHash, ok_blockHash := r.URL.Query()["blockHash"]
	var _blockHash string
	if ok_blockHash {
		_blockHash = strings.Join(blockHash, "")
	}

	contractAddress, ok_contractAddress := r.URL.Query()["contractAddress"]
	_contractAddress := strings.Join(contractAddress, "")

	if ok_contractAddress && (ok_blockHash || ok_blockNumber) {

		res, err := rh.RestFeeder.GetCode(_contractAddress, _blockHash, _blockNumber)
		// test
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}

	fmt.Fprintf(w, "GetCode Request Failed: expected (blockNumber or blockHash) and contractAddress")
}

// GetStorageAt returns StorageInfo
func (rh *RestHandler) GetStorageAt(w http.ResponseWriter, r *http.Request) {
	query_args := r.URL.Query()

	blockNumber, ok_blockNumber := query_args["blockNumber"]
	var _blockNumber string
	if ok_blockNumber {
		_blockNumber = strings.Join(blockNumber, "")
	}
	blockHash, ok_blockHash := r.URL.Query()["blockHash"]
	var _blockHash string
	if ok_blockHash {
		_blockHash = strings.Join(blockHash, "")
	}

	contractAddress, ok_contractAddress := r.URL.Query()["contractAddress"]
	var _contractAddress string
	if ok_contractAddress {
		_contractAddress = strings.Join(contractAddress, "")
	}

	key, ok_key := r.URL.Query()["key"]
	var _key string
	if ok_key {
		_key = strings.Join(key, "")
	}

	if (ok_blockHash || ok_blockNumber) && ok_contractAddress && ok_key {
		res, err := rh.RestFeeder.GetStorageAt(_key, _contractAddress, _blockNumber, _blockHash)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
	}
	fmt.Fprintf(w, "GetStorageAt Request Failed: expected (blockNumber or blockHash), contractAddress, and key")
}

// GetTransactionStatus returns Transaction Status
func (rh *RestHandler) GetTransactionStatus(w http.ResponseWriter, r *http.Request) {
	txHash, ok_txHash := r.URL.Query()["transactionHash"]
	_txHash := strings.Join(txHash, "")

	txId, ok_txId := r.URL.Query()["txId"]
	_txId := strings.Join(txId, "")

	if ok_txHash || ok_txId {
		res, err := rh.RestFeeder.GetTransactionStatus(_txHash, _txId)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetTransactionStatus failed: expected txId or transactionHash")
}
