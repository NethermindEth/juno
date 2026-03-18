package rpccore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

// LimitSliceHexOnly is like LimitSlice[T, L] but only accepts 0x-prefixed
// hexadecimal strings during JSON unmarshaling, rejecting decimal and bare-hex
type LimitSliceHexOnly[T any, L Limit] struct {
	Data []T `validate:"dive"`
}

func (h LimitSliceHexOnly[T, L]) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.Data)
}

func (h *LimitSliceHexOnly[T, L]) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))

	if err := expectDelim(decoder, '['); err != nil {
		return err
	}

	var limit L
	h.Data = []T{}
	for decoder.More() {
		if len(h.Data) >= limit.Limit() {
			return fmt.Errorf("expected max %d items", limit.Limit())
		}

		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return err
		}

		s := strings.Trim(string(raw), `"`)
		if !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
			return errors.New("calldata value must be a 0x-prefixed hex string")
		}

		var value T
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		h.Data = append(h.Data, value)
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
