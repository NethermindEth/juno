package rest

import (
	"net/http"

	"github.com/NethermindEth/juno/pkg/feeder"
)

var rest_handler RestHandler

func NewServer(rest_port string, feeder_gateway string, prefix string) *Server {
	rest_handler.RestFeeder = feeder.NewClient(feeder_gateway, "/feeder_gateway", nil)
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
