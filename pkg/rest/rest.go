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

// GetStateUpdate returns state update achieved in specified block
func (rh *RestHandler) GetStateUpdate(w http.ResponseWriter, r *http.Request) {
	blockHash, ok_blockHash := r.URL.Query()["blockHash"]
	blockNumber, ok_blockNumber := r.URL.Query()["blockNumber"]

	if ok_blockHash || ok_blockNumber {
		res, err := rh.RestFeeder.GetStateUpdate(strings.Join(blockHash, ""), strings.Join(blockNumber, ""))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetStateUpdate failed: expected Block Identifier")
}

// GetContractAddresses returns starknet contract addresses
func (rh *RestHandler) GetContractAddresses(w http.ResponseWriter, r *http.Request) {
	res, err := rh.RestFeeder.GetContractAddresses()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest) // 400 http status code
		fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
		return
	}
	json.NewEncoder(w).Encode(res)
}

// GetBlockHashById returns block hash given blockId
func (rh *RestHandler) GetBlockHashById(w http.ResponseWriter, r *http.Request) {
	blockId, ok_blockId := r.URL.Query()["blockId"]

	if ok_blockId {
		res, err := rh.RestFeeder.GetBlockHashById(strings.Join(blockId, ""))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetBlockHashById failed: expected blockId")
}

// GetBlockIdByHash returns blockId given block hash
func (rh *RestHandler) GetBlockIDByHash(w http.ResponseWriter, r *http.Request) {
	blockHash, ok_blockHash := r.URL.Query()["blockHash"]

	if ok_blockHash {
		res, err := rh.RestFeeder.GetBlockIDByHash(strings.Join(blockHash, ""))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetBlockIDByHash failed: expected blockHash")
}

// GetTransactionHashById returns transaction hash given transactionId
func (rh *RestHandler) GetTransactionHashByID(w http.ResponseWriter, r *http.Request) {
	transactionId, ok_transactionId := r.URL.Query()["transactionId"]

	if ok_transactionId {
		res, err := rh.RestFeeder.GetTransactionHashByID(strings.Join(transactionId, ""))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetTransactionHashByID failed: expected transactionId")
}

// GetTransactionIDByHash returns transactionId given transaction hash
func (rh *RestHandler) GetTransactionIDByHash(w http.ResponseWriter, r *http.Request) {
	transactionHash, ok_transactionHash := r.URL.Query()["transactionHash"]

	if ok_transactionHash {
		res, err := rh.RestFeeder.GetTransactionIDByHash(strings.Join(transactionHash, ""))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "GetTransactionIDByHash failed: expected transactionHash")
}
