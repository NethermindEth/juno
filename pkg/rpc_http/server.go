package rpcnew

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
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
}

func (s *httpRpc) AddService(name string, service interface{}) error {
	serv, err := NewRpcService(name, service)
	if err != nil {
		return err
	}
	s.services[name] = serv
	return nil
}

func (s *httpRpc) Do(request RpcRequest) (*RpcResponse, error) {
	serviceName := extractServiceName(request.Method)
	// Find the service
	service, ok := s.services[serviceName]
	if !ok {
		return nil, NewErrMethodNotFound(nil)
	}
	return service.Call(context.Background(), request)
}

func (s *httpRpc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	// Get the body
	rawData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseError(w, NewErrInternalError(err.Error()))
		return
	}

	var responseData interface{}
	w.Header().Add("Content-type", "application/json")

	decoder := json.NewDecoder(bytes.NewReader(rawData))
	if isBatch(rawData) {
		// Batch request
		var rawRequests []json.RawMessage
		err := decoder.Decode(&rawRequests)
		if err != nil {
			responseError(w, NewErrParseError(err.Error()))
		}
		responses := make([]*RpcResponse, len(rawRequests))
		for i, rawRequest := range rawRequests {
			var request RpcRequest
			err := json.Unmarshal(rawRequest, &request)
			if err != nil {
				responses[i] = NewRpcError(request.Id, NewErrParseError(err.Error()))
				continue
			}
			response, err := s.Do(request)
			if err != nil {
				responses[i] = NewRpcError(request.Id, err)
			} else {
				responses[i] = response
			}
		}
		responseData = responses
	} else {
		// Single request
		var request RpcRequest
		err := decoder.Decode(&request)
		if err != nil {
			responseError(w, NewErrParseError(err.Error()))
			return
		}
		response, err := s.Do(request)
		if err != nil {
			responseData = NewRpcError(request.Id, err)
		} else {
			responseData = response
		}
	}

	// Send response
	encoder := json.NewEncoder(w)
	err = encoder.Encode(&responseData)
	if err != nil {
		responseError(w, NewErrInternalError(err))
		return
	}
}

func isBatch(raw json.RawMessage) bool {
	for _, c := range raw {
		// skip insignificant whitespace (http://www.ietf.org/rfc/rfc4627.txt)
		if c == 0x20 || c == 0x09 || c == 0x0a || c == 0x0d {
			continue
		}
		return c == '['
	}
	return false
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
