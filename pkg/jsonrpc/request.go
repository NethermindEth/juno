package jsonrpc

import (
	"bytes"
	"encoding/json"
)

type rpcRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Id     any             `json:"id"`
}

func (r *rpcRequest) UnmarshalJSON(data []byte) error {
	var rawRequest struct {
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		Id      any             `json:"id"`
		Jsonrpc string          `json:"jsonrpc"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&rawRequest); err != nil {
		return err
	}
	if rawRequest.Jsonrpc != jsonrpcVersion {
		return ErrInvalidRequest
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
				return ErrInvalidRequest
			}
			r.Id = intId
		default:
			return ErrInvalidRequest
		}
	}
	return nil
}

func (r *rpcRequest) IsNotification() bool {
	return r.Id == nil
}
