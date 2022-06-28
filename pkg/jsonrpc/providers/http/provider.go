package httpprovider

import (
	"io/ioutil"
	"net/http"

	"github.com/NethermindEth/juno/pkg/jsonrpc"
)

type HttpProvider struct {
	http.Handler
	jsonrpcServer jsonrpc.Server
}

func NewHttpProvider(jsonrpcServer jsonrpc.Server) *HttpProvider {
	return &HttpProvider{
		jsonrpcServer: jsonrpcServer,
	}
}

func (p *HttpProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check request method
	if r.Method != http.MethodPost {
		// All the requests should be POST
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Check request content type
	if r.Header.Get("Content-Type") != "application/json" {
		// All the requests should be JSON
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	// Read the request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	response := p.jsonrpcServer.Call(body)
	_, err = w.Write(response)
	if err != nil {
		// TODO: log error
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
}
