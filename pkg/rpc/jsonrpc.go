package rpc

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/goccy/go-json"
)

const (
	// Version is JSON-RPC 2.0.
	Version = "2.0"

	batchRequestKey  = '['
	contentTypeKey   = "Content-Type"
	contentTypeValue = "application/json"
)

type (
	// Request represents a JSON-RPC request received by the server.
	Request struct {
		Version string           `json:"jsonrpc"`
		Method  string           `json:"method"`
		Params  *json.RawMessage `json:"params"`
		ID      *json.RawMessage `json:"id"`
	}

	// Response represents a JSON-RPC response returned by the server.
	Response struct {
		Version string           `json:"jsonrpc"`
		Result  interface{}      `json:"result,omitempty"`
		Error   *Error           `json:"error,omitempty"`
		ID      *json.RawMessage `json:"id,omitempty"`
	}
)

// ParseRequest parses a HTTP request to JSON-RPC request.
func ParseRequest(r *http.Request) ([]*Request, bool, *Error) {
	var reqErr *Error

	if !strings.HasPrefix(r.Header.Get(contentTypeKey), contentTypeValue) {
		// notest
		return nil, false, ErrInvalidRequest()
	}

	buf := bytes.NewBuffer(make([]byte, 0, r.ContentLength))
	if _, err := buf.ReadFrom(r.Body); err != nil {
		// notest
		return nil, false, ErrInvalidRequest()
	}
	defer func(r *http.Request) {
		err := r.Body.Close()
		if err != nil {
			// notest
			reqErr = ErrInternal()
		}
	}(r)

	if buf.Len() == 0 {
		// notest
		return nil, false, ErrInvalidRequest()
	}

	f, _, err := buf.ReadRune()
	if err != nil {
		// notest
		return nil, false, ErrInvalidRequest()
	}
	if err := buf.UnreadRune(); err != nil {
		// notest
		return nil, false, ErrInvalidRequest()
	}

	var rs []*Request
	if f != batchRequestKey {
		var req *Request
		if err := json.NewDecoder(buf).Decode(&req); err != nil {
			return nil, false, ErrParse()
		}
		return append(rs, req), false, nil
	}

	if err := json.NewDecoder(buf).Decode(&rs); err != nil {
		// notest
		return nil, false, ErrParse()
	}

	return rs, true, reqErr
}

// NewResponse generates a JSON-RPC response.
func NewResponse(r *Request) *Response {
	return &Response{Version: r.Version, ID: r.ID}
}

// SendResponse writes JSON-RPC response.
func SendResponse(w http.ResponseWriter, resp []*Response, batch bool) error {
	w.Header().Set(contentTypeKey, contentTypeValue)
	if batch || len(resp) > 1 {
		return json.NewEncoder(w).Encode(resp)
	} else if len(resp) == 1 {
		return json.NewEncoder(w).Encode(resp[0])
	}
	// notest
	return nil
}
