package rest

import (
	"net/http"

	"github.com/NethermindEth/juno/pkg/feeder"
)

type RestHandler struct {
	GetBlock             func(http.ResponseWriter, *http.Request)
	GetStorageAt         func(http.ResponseWriter, *http.Request)
	GetTransactionStatus func(http.ResponseWriter, *http.Request)
	GetCode              func(http.ResponseWriter, *http.Request)
	RestFeeder           *feeder.Client
	//rest_server *http.Server
}
