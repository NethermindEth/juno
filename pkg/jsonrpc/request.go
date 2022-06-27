package jsonrpc

import (
	"bytes"
	"encoding/json"
)

type rpcRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Id     interface{}     `json:"id"`
}

func (r *rpcRequest) UnmarshalJSON(data []byte) error {
	var rawRequest struct {
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		Id      interface{}     `json:"id"`
		Jsonrpc string          `json:"jsonrpc"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&rawRequest); err != nil {
		if _, ok := err.(*json.UnmarshalTypeError); ok {
			return errInvalidRequest
		}
		return errParseError
	}
	if rawRequest.Jsonrpc != jsonrpc {
		return errInvalidRequest
	}
	r.Method = rawRequest.Method
	r.Params = rawRequest.Params
	if rawRequest.Id != nil {
		switch id := rawRequest.Id.(type) {
		case string:
			r.Id = id
		case json.Number:
			intId, err := id.Int64()
			if err != nil {
				return err
			}
			r.Id = intId
		default:
			return errInvalidRequest
		}
	}
	return nil
}

func (r *rpcRequest) IsNotification() bool {
	return r.Id == nil
}
