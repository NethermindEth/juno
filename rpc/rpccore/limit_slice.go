package rpccore

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
)

type Limit interface {
	Limit() int
}

type (
	SimulationLimit       struct{}
	FunctionCalldataLimit struct{}
	SenderAddressLimit    struct{}
)

const (
	simulationLimit       = 5000
	functionCalldataLimit = 50000
	senderAddressLimit    = 5000
)

func (l SimulationLimit) Limit() int       { return simulationLimit }
func (l FunctionCalldataLimit) Limit() int { return functionCalldataLimit }
func (l SenderAddressLimit) Limit() int    { return senderAddressLimit }

// LimitSlice is a JSON array that rejects more than L items while it decodes.
type LimitSlice[T any, L Limit] struct {
	Data []T `validate:"dive"`
}

func (l LimitSlice[T, L]) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, l.Data)
}

func (l *LimitSlice[T, L]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if err := expectDelim(dec, '['); err != nil {
		return err
	}

	var limit L
	l.Data = []T{}
	for dec.PeekKind() != ']' {
		if len(l.Data) >= limit.Limit() {
			return fmt.Errorf("expected max %d items", limit.Limit())
		}
		var value T
		if err := json.UnmarshalDecode(dec, &value); err != nil {
			return err
		}
		l.Data = append(l.Data, value)
	}

	return expectDelim(dec, ']')
}

func expectDelim(dec *jsontext.Decoder, delim jsontext.Kind) error {
	token, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if token.Kind() != delim {
		return fmt.Errorf("expected %s, got %s", delim, token.Kind())
	}
	return nil
}
