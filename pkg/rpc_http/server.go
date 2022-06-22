package rpcnew

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"strings"
)

var JsonRpcVersion = "2.0"

type HTTPRpc interface {
	http.Handler
	AddService(name string, service interface{}) error
	Do(request RpcRequest) (*RpcResponse, error)
}

func NewHTTPRpc() (HTTPRpc, error) {
	return &httpRpc{
		services: make(map[string]*RpcService),
	}, nil
}

type httpRpc struct {
	services map[string]*RpcService
	mu       sync.Mutex
}

func (s *httpRpc) AddService(name string, service interface{}) error {
	serv, err := NewRpcService(name, service)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[name] = serv
	return nil
}

func (s *httpRpc) Do(request RpcRequest) (*RpcResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, err
	}
	serviceName := extractServiceName(request.Method)
	// Find the service
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[serviceName]
	if !ok {
		return nil, NewErrMethodNotFound(request.Method)
	}
	return service.Call(context.Background(), request)
}

func (s *httpRpc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Check Content-type header?
	// Decode request body as a RpcRequest
	// TODO: Check if request is a batch of requests
	w.Header().Add("Content-type", "application/json")
	decoder := json.NewDecoder(r.Body)
	request := new(RpcRequest)
	err := decoder.Decode(request)
	if err != nil {
		responseError(w, NewErrParseError(err.Error()))
		return
	}

	fmt.Printf("RPC request:\n%s", *request)
	// Call the RPC
	response, rpcErr := s.Do(*request)
	if rpcErr != nil {
		responseError(w, rpcErr)
		return
	}

	// Send response
	encoder := json.NewEncoder(w)
	err = encoder.Encode(response)
	if err != nil {
		responseError(w, NewErrInternalError(err))
		return
	}
	// w.WriteHeader(http.StatusOK)
}

func responseError(w http.ResponseWriter, rpcErr error) {
	encoder := json.NewEncoder(w)
	err := encoder.Encode(&RpcResponse{
		Id:      nil,
		Jsonrpc: JsonRpcVersion,
		Error:   rpcErr,
	})
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func extractServiceName(method string) string {
	splits := strings.SplitN(method, "_", 2)
	if len(splits) == 1 {
		return ""
	}
	return splits[0]
}
