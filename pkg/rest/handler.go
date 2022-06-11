package rest

import (
	"net/http"

	"github.com/NethermindEth/juno/pkg/feeder"
)

var rest_handler RestHandler

func NewServer(rest_port string, feeder_gateway string) *Server {
	rest_handler.RestFeeder = feeder.NewClient(feeder_gateway, "/feeder_gateway", nil)
	m := http.NewServeMux()

	m.HandleFunc("/get_block", rest_handler.GetBlock)
	m.HandleFunc("/get_code", rest_handler.GetCode)
	m.HandleFunc("/get_storage_at", rest_handler.GetStorageAt)
	m.HandleFunc("/get_transaction_status", rest_handler.GetTransactionStatus)
	m.HandleFunc("/get_transaction_receipt", rest_handler.GetTransactionReceipt)
	m.HandleFunc("/get_transaction", rest_handler.GetTransaction)

	return &Server{server: http.Server{Addr: rest_port, Handler: m}}
}
