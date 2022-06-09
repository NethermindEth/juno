package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/NethermindEth/juno/pkg/feeder"
)

//Returns Starknet Block
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
		//test
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "Get Block failed: blockNumber or blockHash not present")
}

//GetCode returns CodeInfo using Block Identifier & Contract Address
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
		//test
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}

	fmt.Fprintf(w, "GetCode Request Failed: invalid inputs")
}

//GetStorageAt returns StorageInfo
func (rh *RestHandler) GetStorageAt(w http.ResponseWriter, r *http.Request) {
	query_args := r.URL.Query()
	res, err := rh.RestFeeder.GetStorageAt(strings.Join(query_args["key"], ""), strings.Join(query_args["contractAddress"], ""), strings.Join(query_args["blockNumber"], ""), strings.Join(query_args["blockHash"], ""))
	//test
	if err != nil {
		w.WriteHeader(http.StatusBadRequest) // 400 http status code
		fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
		return
	}
	json.NewEncoder(w).Encode(res)
}

// GetTransactionStatus returns Transaction Status
func (rh *RestHandler) GetTransactionStatus(w http.ResponseWriter, r *http.Request) {

	txHash, ok_txHash := r.URL.Query()["transactionHash"]
	_txHash := strings.Join(txHash, "")

	txId, ok_txId := r.URL.Query()["txId"]
	_txId := strings.Join(txId, "")

	if ok_txHash || ok_txId {
		res, err := rh.RestFeeder.GetTransactionStatus(_txHash, _txId)
		//test
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) // 400 http status code
			fmt.Fprintf(w, "Invalid request body error:%s", err.Error())
			return
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	//test
	fmt.Fprintf(w, "Transaction Status failed: invalid input")
}
