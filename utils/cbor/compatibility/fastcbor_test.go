package cbor_test

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fallbacks counts the reads the fastcbor package hands to fxamacker.
type fallbacks struct{ n atomic.Int64 }

func (f *fallbacks) Warn(string, ...zap.Field) { f.n.Add(1) }
func (*fallbacks) Debug(string, ...zap.Field)  {}
func (*fallbacks) Info(string, ...zap.Field)   {}
func (*fallbacks) Error(string, ...zap.Field)  {}
func (*fallbacks) Trace(string, ...zap.Field)  {}

// decodeWithFast reads into a T. The fastcbor package is generic and a type cannot be named from a
// reflect.Type, so each type the node reads with it has a decoder below.
func decodeWithFast[T any](data []byte, dst any) error {
	return fastcbor.Unmarshal(data, dst.(*T))
}

// decodeTransactionWithFast reads as the node does, into a core.Transaction, whose concrete type
// the tag in the stored bytes names, and hands that value on.
func decodeTransactionWithFast(data []byte, dst any) error {
	var transaction core.Transaction
	if err := fastcbor.Unmarshal(data, &transaction); err != nil {
		return err
	}
	// The interface holds a pointer to the concrete type.
	value := reflect.ValueOf(transaction)
	want := reflect.TypeOf(dst).Elem()
	if value.Kind() != reflect.Pointer || value.Elem().Type() != want {
		return fmt.Errorf("read a %v, want a pointer to a %v", value.Type(), want)
	}
	reflect.ValueOf(dst).Elem().Set(value.Elem())
	return nil
}

var fastDecoders = map[reflect.Type]func([]byte, any) error{
	reflect.TypeFor[core.DeclareTransaction]():       decodeTransactionWithFast,
	reflect.TypeFor[core.DeployTransaction]():        decodeTransactionWithFast,
	reflect.TypeFor[core.InvokeTransaction]():        decodeTransactionWithFast,
	reflect.TypeFor[core.L1HandlerTransaction]():     decodeTransactionWithFast,
	reflect.TypeFor[core.DeployAccountTransaction](): decodeTransactionWithFast,
	reflect.TypeFor[core.Header]():                   decodeWithFast[core.Header],
	reflect.TypeFor[core.StateUpdate]():              decodeWithFast[core.StateUpdate],
	reflect.TypeFor[core.TransactionReceipt]():       decodeWithFast[core.TransactionReceipt],
	reflect.TypeFor[core.DeclaredClassDefinition]():  decodeWithFast[core.DeclaredClassDefinition],
	reflect.TypeFor[core.BlockTransactionsIndexes](): decodeWithFast[core.BlockTransactionsIndexes],
}

// TestFastcborGoldenBytes reads the stored bytes of the records the node reads with the fastcbor
// package, and requires the values the other codecs read from them, with no read left to
// fxamacker.
func TestFastcborGoldenBytes(t *testing.T) {
	var cases []goldenCase
	for _, c := range goldenCases() {
		if _, ok := fastDecoders[reflect.TypeOf(c.value)]; ok {
			cases = append(cases, c)
		}
	}
	require.NotEmpty(t, cases)

	counter := new(fallbacks)
	fastcbor.SetLogger(counter)
	assertDecodesFromGolden(t, func(data []byte, dst any) error {
		return fastDecoders[reflect.TypeOf(dst).Elem()](data, dst)
	}, cases)
	require.Zero(t, counter.n.Load(), "a read was left to fxamacker")
}
