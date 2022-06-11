package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetBlock Returns Starknet Block
func (rh *RestHandler) GetBlock(w http.ResponseWriter, r *http.Request) {
	blockNumber, ok_blockNumber := r.URL.Query()["blockNumber"]
	blockHash, ok_blockHash := r.URL.Query()["blockHash"]

	if ok_blockNumber || ok_blockHash {
		res, err := rh.RestFeeder.GetBlock(strings.Join(blockHash, ""), strings.Join(blockNumber, ""))
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
	blockHash, ok_blockHash := r.URL.Query()["blockHash"]
	contractAddress, ok_contractAddress := r.URL.Query()["contractAddress"]

	if ok_contractAddress && (ok_blockHash || ok_blockNumber) {

		res, err := rh.RestFeeder.GetCode(strings.Join(contractAddress, ""), strings.Join(blockHash, ""), strings.Join(blockNumber, ""))
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
	blockNumber, ok_blockNumber := r.URL.Query()["blockNumber"]
	blockHash, ok_blockHash := r.URL.Query()["blockHash"]
	contractAddress, ok_contractAddress := r.URL.Query()["contractAddress"]
	key, ok_key := r.URL.Query()["key"]

	if (ok_blockHash || ok_blockNumber) && ok_contractAddress && ok_key {
		res, err := rh.RestFeeder.GetStorageAt(strings.Join(key, ""), strings.Join(contractAddress, ""), strings.Join(blockNumber, ""), strings.Join(blockHash, ""))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetStorageAt Request Failed: expected blockIdentifier, contractAddress, and key")
}

// GetTransactionStatus returns Transaction Status
func (rh *RestHandler) GetTransactionStatus(w http.ResponseWriter, r *http.Request) {
	txHash, ok_txHash := r.URL.Query()["transactionHash"]
	txId, ok_txId := r.URL.Query()["txId"]

	if ok_txHash || ok_txId {
		res, err := rh.RestFeeder.GetTransactionStatus(strings.Join(txHash, ""), strings.Join(txId, ""))
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

// GetTransactionReceipt Returns Transaction Receipt
func (rh *RestHandler) GetTransactionReceipt(w http.ResponseWriter, r *http.Request) {
	txHash, ok_txHash := r.URL.Query()["transactionHash"]
	txId, ok_txId := r.URL.Query()["txId"]

	if ok_txHash || ok_txId {
		res, err := rh.RestFeeder.GetTransactionReceipt(strings.Join(txHash, ""), strings.Join(txId, ""))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetTransactionReceipt failed: expected txId or transactionHash")
}

// GetTransactionReceipt Returns Transaction Receipt
func (rh *RestHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	txHash, ok_txHash := r.URL.Query()["transactionHash"]
	txId, ok_txId := r.URL.Query()["txId"]

	if ok_txHash || ok_txId {
		res, err := rh.RestFeeder.GetTransaction(strings.Join(txHash, ""), strings.Join(txId, ""))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetTransaction failed: expected txId or transactionHash")
}

// GetFullContract returns full contract: code and ABI
func (rh *RestHandler) GetFullContract(w http.ResponseWriter, r *http.Request) {
	blockHash, ok_blockHash := r.URL.Query()["blockHash"]
	blockNumber, ok_blockNumber := r.URL.Query()["blockNumber"]
	contractAddress, ok_contractAddress := r.URL.Query()["contractAddress"]

	if (ok_blockHash || ok_blockNumber) && ok_contractAddress {
		res, err := rh.RestFeeder.GetFullContract(strings.Join(contractAddress, ""), strings.Join(blockHash, ""), strings.Join(blockNumber, ""))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetFullContract failed: expected contractAddress and Block Identifier")
}
