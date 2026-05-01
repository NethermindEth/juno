package rpccore

import (
	"fmt"

	"github.com/NethermindEth/juno/utils/jsonx"
	"github.com/bytedance/sonic/ast"
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
	return jsonx.Marshal(l.Data)
}

// UnmarshalJSON walks the input array via sonic's lazy AST iterator and
// rejects as soon as the count exceeds L.Limit(), without decoding any
// subsequent elements.
func (l *LimitSlice[T, L]) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty input")
	}

	// (*ast.Node).UnmarshalJSON aliases data via sonic's internal
	// zero-copy []byte→string. The node is initially raw; first access
	// (via Values below) promotes it to a lazy array so iteration
	// scans one element at a time.
	var node ast.Node
	if err := node.UnmarshalJSON(data); err != nil {
		return err
	}

	iter, err := node.Values()
	if err != nil {
		return err
	}

	var limit L
	maxItems := limit.Limit()
	l.Data = []T{}
	var elem ast.Node
	for iter.Next(&elem) {
		if len(l.Data) >= maxItems {
			return fmt.Errorf("expected max %d items", maxItems)
		}
		raw, err := elem.Raw()
		if err != nil {
			return err
		}
		var v T
		if err := jsonx.UnmarshalString(raw, &v); err != nil {
			return err
		}
		l.Data = append(l.Data, v)
	}
	return nil
}
