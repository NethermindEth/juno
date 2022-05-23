package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/gorilla/mux"
)

//writes all http.Responses to output in JSON format
// func responseToJSON(w http.ResponseWriter, resp *http.Response) {
// 	body, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		log.Fatalln(err)
// 	}
// 	//Convert the body to type string
// 	sb := string(body)
// 	json.NewEncoder(w).Encode(sb)
// }

//cannot unmarshal array into Go struct field StarknetBlock.transactions of type feeder.TxnSpecificInfo
func getBlock(w http.ResponseWriter, r *http.Request) {

	var (
		res feeder.StarknetBlock
		err error
	)

	feederClient := feeder.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway", nil)

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

//return block with optional arguments for blockhash or blocknumber. Returns latest block if not specified.
// func getBlock(w http.ResponseWriter, r *http.Request) {
// 	var get_block_url string
// 	blockNumber, ok_blockNumber := r.URL.Query()["blockNumber"]
// 	blockHash, ok_blockHash := r.URL.Query()["blockHash"]
// 	//return latest block if block_hash and block_number not specified
// 	if !ok_blockNumber && !ok_blockHash {
// 		fmt.Println(`latest block`)
// 		get_block_url = "https://alpha-mainnet.starknet.io/feeder_gateway/get_block"
// 		//return block by block_hash if block_hash is specified
// 	} else if ok_blockHash {
// 		blockHash := strings.Join(blockHash, "")
// 		get_block_url = fmt.Sprintf("https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockHash=%s", blockHash)
// 		//return block by block_number if only block_number is specified
// 	} else if ok_blockNumber {
// 		blockNumber := strings.Join(blockNumber, "")
// 		get_block_url = fmt.Sprintf("https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=%s", blockNumber)
// 	}
// 	resp, err := http.Get(get_block_url)
// 	if err != nil {
// 		log.Fatalln(err)
// 	}
// 	responseToJSON(w, resp)
// }

//(feederClient *feeder.Client)
func getCode(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintf(w, "Code request handled")
	//globally defined feed_gateway_client
	feederClient := feeder.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway", nil)

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
		//sb := string(strings.Join(res, ""))
		json.NewEncoder(w).Encode(res)
		return
	}

	fmt.Fprintf(w, "GetCode Request Failed: invalid inputs")
}

func getStorageAt(w http.ResponseWriter, r *http.Request) {
	query_args := r.URL.Query()
	fmt.Fprintf(w, "before")
	feederClient := feeder.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway", nil)
	fmt.Fprintf(w, "start")
	res, err := feederClient.GetStorageAt(strings.Join(query_args["key"], ""), strings.Join(query_args["contractAddress"], ""), strings.Join(query_args["blockNumber"], ""), strings.Join(query_args["blockHash"], ""))
	if err != nil {
		log.Fatalln(err)
	}
	//fmt.Fprintf(w, res)
	json.NewEncoder(w).Encode(res)
}

//works
func getTransactionStatus(w http.ResponseWriter, r *http.Request) {
	feederClient := feeder.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway", nil)

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

// func requestNotFound(w http.ResponseWriter, r *http.Request) {
// 	w.WriteHeader(http.StatusNotFound)
// 	w.Write([]byte("request not found"))
// }

func main() {

	router := mux.NewRouter()
	router.UseEncodedPath()

	//globally defined feed_gateway_client
	//feederClient := feeder.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway", nil)

	//get_block endpoint
	router.HandleFunc("/get_block", getBlock).Methods("GET")
	//get_code endpoint
	router.HandleFunc("/get_code", getCode).Methods("GET")
	//get_storage endpoint
	router.HandleFunc("/get_storage_at", getStorageAt).Methods("GET")
	//get_transaction endpoint
	router.HandleFunc("/juno/get_transaction_status", getTransactionStatus).Methods("GET")
	//port :8100
	log.Fatal(http.ListenAndServe(":8100", router))
}
