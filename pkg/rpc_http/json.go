package rpcnew

import (
	"bytes"
	"encoding/json"
	"errors"
)

type RpcRequest struct {
	Id     interface{}     `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
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
	// Check jsonrpc version
	if req.Jsonrpc != JsonRpcVersion {
		return errors.New("jsonrpc version not match")
	}
	// Check id
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
	r.Method = req.Method
	r.Params = req.Params
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
