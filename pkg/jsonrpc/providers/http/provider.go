package httpprovider

import (
	"io/ioutil"
	"net/http"

	"github.com/NethermindEth/juno/pkg/jsonrpc"
)

type HttpProvider struct {
	http.Handler
	jsonrpcServer *jsonrpc.JsonRpc
	cors          bool
	corsOrigins   string
}

func NewHttpProvider(jsonrpcServer *jsonrpc.JsonRpc) *HttpProvider {
	return &HttpProvider{
		jsonrpcServer: jsonrpcServer,
	}
}

func (p *HttpProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.cors {
		w.Header().Set("Access-Control-Allow-Origin", p.corsOrigins)
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	// Check request method
	if r.Method != http.MethodPost {
		// All the requests should be POST
		w.Header().Set("Allow", http.MethodPost)
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
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (p *HttpProvider) EnableCors(origins string) {
	p.cors = true
	p.corsOrigins = origins
}
