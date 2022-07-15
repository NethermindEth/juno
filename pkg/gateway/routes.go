package gateway

import "net/http"

func routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", notFound)
	mux.HandleFunc("/v1/get_block", getBlock)
	mux.HandleFunc("/v1/get_block_hash_by_id", getBlockHashByID)
	mux.HandleFunc("/v1/get_block_id_by_hash", getBlockIDByHash)
	mux.HandleFunc("/v1/get_code", getCode)
	mux.HandleFunc("/v1/get_contract_addresses", getContractAddresses)
	mux.HandleFunc("/v1/get_storage_at", getStorageAt)
	mux.HandleFunc("/v1/get_transaction", getTransaction)
	mux.HandleFunc("/v1/get_transaction_hash_by_id", getTransactionHashByID)
	mux.HandleFunc("/v1/get_transaction_id_by_hash", getTransactionIDByHash)

	// Wrap the ServeMux in middleware.
	return recoverPanic(logRequest(mux))
}
