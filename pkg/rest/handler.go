package rest

import (
	"net/http"

	"github.com/NethermindEth/juno/pkg/feeder"
)

// rest_handler object is used to route calls from the Rest Server
var rest_handler RestHandler

// NewServer creates a REST API server with the listed endpoints
func NewServer(rest_port string, feederClient *feeder.Client) *Server {
	rest_handler.RestFeeder = feederClient
	prefix := feederClient.BaseAPI
	m := http.NewServeMux()

	m.HandleFunc(prefix+"/get_block", rest_handler.GetBlock)
	m.HandleFunc(prefix+"/get_code", rest_handler.GetCode)
	m.HandleFunc(prefix+"/get_storage_at", rest_handler.GetStorageAt)
	m.HandleFunc(prefix+"/get_transaction_status", rest_handler.GetTransactionStatus)
	m.HandleFunc(prefix+"/get_transaction_receipt", rest_handler.GetTransactionReceipt)
	m.HandleFunc(prefix+"/get_transaction", rest_handler.GetTransaction)
	m.HandleFunc(prefix+"/get_full_contract", rest_handler.GetFullContract)
	m.HandleFunc(prefix+"/get_state_update", rest_handler.GetStateUpdate)
	m.HandleFunc(prefix+"/get_contract_addresses", rest_handler.GetContractAddresses)
	m.HandleFunc(prefix+"/get_block_hash_by_id", rest_handler.GetBlockHashById)
	m.HandleFunc(prefix+"/get_block_id_by_hash", rest_handler.GetBlockIDByHash)
	m.HandleFunc(prefix+"/get_transaction_hash_by_id", rest_handler.GetTransactionHashByID)
	m.HandleFunc(prefix+"/get_transaction_id_by_hash", rest_handler.GetTransactionIDByHash)

	return &Server{server: http.Server{Addr: rest_port, Handler: m}}
}
