package rpcnew

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
)

type RpcRequest struct {
	Id      interface{}     `json:"id"`
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (r *RpcRequest) UnmarshalJSON(data []byte) error {
	type rawRequest struct {
		Id      interface{}     `json:"id"`
		Jsonrpc string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var req rawRequest
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	switch x := req.Id.(type) {
	case json.Number:
		id, err := x.Int64()
		if err != nil {
			return err
		}
		r.Id = id
	case string:
		r.Id = x
	default:
		return errors.New("invalid id type")
	}
	r.Jsonrpc = req.Jsonrpc
	r.Method = req.Method
	r.Params = req.Params
	return nil
}

func (r *RpcRequest) Validate() error {
	if r.Jsonrpc != JsonRpcVersion {
		return NewErrInvalidRequest("invalid jsonrpc field")
	}
	if err := r.validateId(); err != nil {
		return err
	}
	if r.Method == "" {
		return NewErrInvalidRequest("empty method field")
	}

	return nil
}

func (r *RpcRequest) validateId() error {
	if reflect.ValueOf(r.Id).IsValid() {
		switch r.Id.(type) {
		case int64, string:
			break
		default:
			return NewErrInvalidRequest("invalid id type")
		}
	}
	return nil
}

func (r RpcRequest) String() string {
	buff := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buff)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(&r)
	return buff.String()
}

type RpcResponse struct {
	Id      interface{} `json:"id"`
	Jsonrpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   error       `json:"error,omitempty"`
}

func NewRpcResponse(id interface{}, result interface{}) *RpcResponse {
	return &RpcResponse{
		Id:      id,
		Jsonrpc: JsonRpcVersion,
		Result:  result,
	}
}

func NewRpcError(id interface{}, err error) *RpcResponse {
	return &RpcResponse{
		Id:      id,
		Jsonrpc: JsonRpcVersion,
		Error:   err,
	}
}
