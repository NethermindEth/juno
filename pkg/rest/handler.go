package rest

import (
	"net/http"

	"github.com/NethermindEth/juno/pkg/feeder"
)

var handler RestHandler

// // handlerGetBlock returns Starknet Block using handler.feederGateway
// func handlerGetBlock(w http.ResponseWriter, r *http.Request) {
// 	handler.GetBlock(w, r)
// }

// // handlerGetCode returns CodeInfo using Block Identifier & Contract Address
// func handlerGetCode(w http.ResponseWriter, r *http.Request) {
// 	handler.GetCode(w, r)
// }

// // handlerGetStorageAt returns StorageInfo
// func handlerGetStorageAt(w http.ResponseWriter, r *http.Request) {
// 	handler.GetStorageAt(w, r)
// }

// // handlerGetTransactionStatus returns Transaction Status
// func handlerGetTransactionStatus(w http.ResponseWriter, r *http.Request) {
// 	handler.GetTransactionStatus(w, r)
// }

func NewServer(rest_port string, feeder_gateway string) *Server {

	handler.RestFeeder = feeder.NewClient(feeder_gateway, "/feeder_gateway", nil)
	m := http.NewServeMux()

	m.HandleFunc("/get_block", handler.GetBlock)
	m.HandleFunc("/get_code", handler.GetCode)
	m.HandleFunc("/get_storage_at", handler.GetStorageAt)
	m.HandleFunc("/get_transaction_status", handler.GetTransactionStatus)

	return &Server{server: http.Server{Addr: rest_port, Handler: m}}
}
