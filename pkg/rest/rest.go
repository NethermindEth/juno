package rest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/NethermindEth/juno/pkg/feeder"
)

var feederClient *feeder.Client

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
		if err != nil {
			log.Fatalln(err)
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "Get Block failed: blockNumber or blockHash not present")
}

//GetCode returns CodeInfo using Block Identifier & Contract Address
func GetCode(w http.ResponseWriter, r *http.Request) {

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

		res, err := feederClient.GetCode(_contractAddress, _blockHash, _blockNumber)
		if err != nil {
			log.Fatalln(err)
		}
		json.NewEncoder(w).Encode(res)
		return
	}

	fmt.Fprintf(w, "GetCode Request Failed: invalid inputs")
}

//GetStorageAt returns StorageInfo
func GetStorageAt(w http.ResponseWriter, r *http.Request) {
	query_args := r.URL.Query()
	res, err := feederClient.GetStorageAt(strings.Join(query_args["key"], ""), strings.Join(query_args["contractAddress"], ""), strings.Join(query_args["blockNumber"], ""), strings.Join(query_args["blockHash"], ""))
	if err != nil {
		log.Fatalln(err)
	}
	json.NewEncoder(w).Encode(res)
}

// GetTransactionStatus returns Transaction Status
func GetTransactionStatus(w http.ResponseWriter, r *http.Request) {

	txHash, ok_txHash := r.URL.Query()["transactionHash"]
	_txHash := strings.Join(txHash, "")

	txId, ok_txId := r.URL.Query()["txId"]
	_txId := strings.Join(txId, "")

	if ok_txHash || ok_txId {
		res, err := feederClient.GetTransactionStatus(_txHash, _txId)
		if err != nil {
			log.Fatalln(err)
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "Transaction Status failed: invalid input")
}

func NewServer(rest_port string, feeder_gateway string) *Server {
	m := http.NewServeMux()

	//m.HandleFunc("/get_block", GetBlock)
	m.HandleFunc("/get_code", GetCode)
	m.HandleFunc("/get_storage_at", GetStorageAt)
	m.HandleFunc("/get_transaction_status", GetTransactionStatus)
	//m.HandleFunc("/get_full_contract", getTransactionStatus)

	feederClient = feeder.NewClient(feeder_gateway, "/feeder_gateway", nil)

	return &Server{server: http.Server{Addr: rest_port, Handler: m}}
}
