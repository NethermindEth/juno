package main

import (
	"flag"
	"log"
	"net/http"
	"os"
)

var (
	logErr  = log.New(os.Stderr, "ERROR ", log.LstdFlags|log.Lshortfile)
	logInfo = log.New(os.Stdout, "INFO ", log.LstdFlags)
)

func main() {
	addr := flag.String("addr", ":4000", "local network address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlerNotFound)
	mux.HandleFunc("/v1/get_block", handlerGetBlock)
	mux.HandleFunc("/v1/get_block_hash_by_id", handlerGetBlockHashByID)
	mux.HandleFunc("/v1/get_block_id_by_hash", handlerGetBlockIDByHash)
	mux.HandleFunc("/v1/get_code", handlerGetCode)
	mux.HandleFunc("/v1/get_contract_addresses", handlerGetContractAddresses)
	mux.HandleFunc("/v1/get_storage_at", handlerGetStorageAt)
	mux.HandleFunc("/v1/get_transaction", handlerGetTransaction)
	mux.HandleFunc("/v1/get_transaction_hash_by_id", handlerGetTransactionHashByID)
	mux.HandleFunc("/v1/get_transaction_id_by_hash", handlerGetTransactionIDByHash)

	srv := &http.Server{
		Addr:     *addr,
		ErrorLog: logErr,
		Handler:  mux,
	}

	logInfo.Printf("serving on http://localhost%s", *addr)
	logErr.Fatal(srv.ListenAndServe())
}
