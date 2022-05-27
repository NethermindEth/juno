package rest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/gorilla/mux"
)

var feederClient *feeder.Client

//Returns Starknet Block
func getBlock(w http.ResponseWriter, r *http.Request) {
	var (
		res *feeder.StarknetBlock
		err error
	)
	blockNumber, ok_blockNumber := r.URL.Query()["blockNumber"]
	_blockNumber := strings.Join(blockNumber, "")

	blockHash, ok_blockHash := r.URL.Query()["blockHash"]
	_blockHash := strings.Join(blockHash, "")

	if ok_blockNumber || ok_blockHash {
		res, err = feederClient.GetBlock(_blockHash, _blockNumber)
		if err != nil {
			log.Fatalln(err)
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Fprintf(w, "Transaction Status failed: invalid input")
}

//returns CodeInfo
func getCode(w http.ResponseWriter, r *http.Request) {

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

//returns StorageInfo
func getStorageAt(w http.ResponseWriter, r *http.Request) {
	query_args := r.URL.Query()
	fmt.Fprintf(w, "before")
	fmt.Fprintf(w, "start")
	res, err := feederClient.GetStorageAt(strings.Join(query_args["key"], ""), strings.Join(query_args["contractAddress"], ""), strings.Join(query_args["blockNumber"], ""), strings.Join(query_args["blockHash"], ""))
	if err != nil {
		log.Fatalln(err)
	}
	json.NewEncoder(w).Encode(res)
}

//Returns Transaction Status
func getTransactionStatus(w http.ResponseWriter, r *http.Request) {

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

func NewServer(port string) {

	router := mux.NewRouter()
	router.UseEncodedPath()

	//globally defined feed_gateway_client
	//config.Runtime.Starknet.FeederGateway
	feederClient = feeder.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway", nil)

	//get_block endpoint
	router.HandleFunc("/get_block", getBlock).Methods("GET")
	//get_code endpoint
	router.HandleFunc("/get_code", getCode).Methods("GET")
	//get_storage endpoint
	router.HandleFunc("/get_storage_at", getStorageAt).Methods("GET")
	//get_transaction endpoint
	router.HandleFunc("/get_transaction_status", getTransactionStatus).Methods("GET")
	//port :8100
	log.Fatal(http.ListenAndServe(port, router))
}
