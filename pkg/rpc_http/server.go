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
	err := request.Validate()
	if err != nil {
		return nil, err
	}
	serviceName := extractServiceName(request.Method)
	// Find the service
	service, ok := s.services[serviceName]
	if !ok {
		return nil, NewErrMethodNotFound(request.Method)
	}
	return service.Call(context.Background(), request)
}

func (s *httpRpc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Check Content-type header?

	// Get the body
	rawData, err := getJsonBody(r)
	if err != nil {
		responseError(w, err)
		return
	}

	var responseData interface{}
	w.Header().Add("Content-type", "application/json")

	decoder := json.NewDecoder(bytes.NewReader(rawData))
	if isBatch(rawData) {
		// Batch request
		var requests []RpcRequest
		err := decoder.Decode(&requests)
		if err != nil {
			responseError(w, NewErrParseError(err.Error()))
		}
		responses := make([]RpcResponse, len(requests))
		for i, request := range requests {
			response, err := s.Do(request)
			if err != nil {
				responseError(w, err)
				return
			}
			responses[i] = *response
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
			responseError(w, err)
			return
		}
		responseData = response
	}

	// Send response
	encoder := json.NewEncoder(w)
	err = encoder.Encode(&responseData)
	if err != nil {
		responseError(w, NewErrInternalError(err))
		return
	}
}

func getJsonBody(r *http.Request) (json.RawMessage, error) {
	bodyRawData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, NewErrParseError(err.Error())
	}
	message := json.RawMessage{}
	err = message.UnmarshalJSON(bodyRawData)
	if err != nil {
		return nil, NewErrParseError(err.Error())
	}
	return message, nil
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
