package gateway

import "net/http"

func (gw *gateway) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", gw.notFound)
	mux.HandleFunc("/v0/get_block", gw.getBlock)
	mux.HandleFunc("/v0/get_block_hash_by_id", gw.getBlockHashByID)
	mux.HandleFunc("/v0/get_block_id_by_hash", gw.getBlockIDByHash)

	// TODO: Implement these routes.
	/*
		mux.HandleFunc("/v0/get_code", getCode)
		mux.HandleFunc("/v0/get_contract_addresses", getContractAddresses)
		mux.HandleFunc("/v0/get_storage_at", getStorageAt)
		mux.HandleFunc("/v0/get_transaction", getTransaction)
		mux.HandleFunc("/v0/get_transaction_hash_by_id", getTransactionHashByID)
		mux.HandleFunc("/v0/get_transaction_id_by_hash", getTransactionIDByHash)
	*/

	// Wrap the ServeMux in middleware.
	return gw.logRequest(gw.panicRecovery(mux))
}
