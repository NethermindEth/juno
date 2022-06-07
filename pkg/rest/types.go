package rest

import (
	//"net/http"

	"github.com/NethermindEth/juno/pkg/feeder"
)

type RestHandler struct {
	RestFeeder *feeder.Client
	//rest_server *http.Server
}
