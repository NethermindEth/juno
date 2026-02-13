package rpccore

import (
	"bytes"
	"encoding/json"
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

type LimitSlice[T any, L Limit] struct {
	Data []T `validate:"dive"`
}

func (l LimitSlice[T, L]) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.Data)
}

func (l *LimitSlice[T, L]) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))

	if err := expectDelim(decoder, '['); err != nil {
		return err
	}

	var limit L
	l.Data = []T{}
	for decoder.More() {
		if len(l.Data) >= limit.Limit() {
			return fmt.Errorf("expected max %d items", limit.Limit())
		}
		var value T
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		l.Data = append(l.Data, value)
	}

	return expectDelim(decoder, ']')
}

func expectDelim(decoder *json.Decoder, delim json.Delim) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token != delim {
		return fmt.Errorf("expected %s, got %s", delim, token)
	}
	return nil
}
