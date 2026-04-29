// Package jsonx is a drop-in replacement for the subset of encoding/json
// used across Juno's RPC path. It delegates to bytedance/sonic under the
// hood for performance. Swapping the underlying library later only
// requires editing this file.
package jsonx

import (
	"io"

	"github.com/bytedance/sonic"
)

var api = sonic.ConfigDefault

// Marshal encodes v as JSON.
func Marshal(v any) ([]byte, error) { return api.Marshal(v) }

// Unmarshal decodes the JSON-encoded data into v.
func Unmarshal(data []byte, v any) error { return api.Unmarshal(data, v) }

// UnmarshalString is like Unmarshal but takes the JSON as a string,
// avoiding a []byte→string copy when the caller already has a string.
func UnmarshalString(data string, v any) error {
	return sonic.UnmarshalString(data, v)
}

// Decoder mirrors sonic.Decoder (a superset of the stdlib decoder
// methods Juno consumes). Exposed as an interface so callers don't
// need to depend on the sonic package directly.
//
// Note: sonic's stream decoder yields top-level values, not tokens —
// there is no Token()/Delim equivalent. Callers that need to peek
// array/object delimiters must decode into json.RawMessage instead.
type Decoder interface {
	Decode(v any) error
	Buffered() io.Reader
	DisallowUnknownFields()
	More() bool
	UseNumber()
}

// NewDecoder returns a JSON decoder reading from r.
func NewDecoder(r io.Reader) Decoder { return api.NewDecoder(r) }
