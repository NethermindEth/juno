package gateway

import "net/http"

func routes() http.Handler {
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

	// Wrap the ServeMux in middleware.
	return recoverPanic(logRequest(mux))
}
