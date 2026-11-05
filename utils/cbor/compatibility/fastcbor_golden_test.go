// This test pins the fastcbor package to the stored bytes of the records the node reads with it.
// Each record is read with fastcbor and must give the value the other codecs read from the same
// bytes. To cover another type, add it to goldenCases and to fastcborDecoders.

package cbor_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/stretchr/testify/require"
)

// decodeFastcbor reads into a T. A type cannot be named from a reflect.Type, so each type the node
// reads with fastcbor has a decoder below.
func decodeFastcbor[T any](data []byte, dst any) error {
	return fastcbor.Unmarshal(data, dst.(*T))
}

// decodeTransactionFastcbor reads as the node does, into a core.Transaction whose concrete type
// the tag in the stored bytes names, and hands that value on.
func decodeTransactionFastcbor(data []byte, dst any) error {
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

var fastcborDecoders = map[reflect.Type]func([]byte, any) error{
	reflect.TypeFor[core.DeclareTransaction]():       decodeTransactionFastcbor,
	reflect.TypeFor[core.DeployTransaction]():        decodeTransactionFastcbor,
	reflect.TypeFor[core.InvokeTransaction]():        decodeTransactionFastcbor,
	reflect.TypeFor[core.L1HandlerTransaction]():     decodeTransactionFastcbor,
	reflect.TypeFor[core.DeployAccountTransaction](): decodeTransactionFastcbor,
	reflect.TypeFor[core.Header]():                   decodeFastcbor[core.Header],
	reflect.TypeFor[core.StateUpdate]():              decodeFastcbor[core.StateUpdate],
	reflect.TypeFor[core.TransactionReceipt]():       decodeFastcbor[core.TransactionReceipt],
	reflect.TypeFor[core.DeclaredClassDefinition]():  decodeFastcbor[core.DeclaredClassDefinition],
	reflect.TypeFor[core.BlockTransactionsIndexes](): decodeFastcbor[core.BlockTransactionsIndexes],
}

// TestFastcborGoldenBytes reads the stored bytes of the records the node reads with the fastcbor
// package, and requires the values the other codecs read from them. fastcbor must read every one
// itself.
func TestFastcborGoldenBytes(t *testing.T) {
	var cases []goldenCase
	for _, c := range goldenCases() {
		if _, ok := fastcborDecoders[reflect.TypeOf(c.value)]; ok {
			cases = append(cases, c)
		}
	}
	require.NotEmpty(t, cases)

	assertDecodesFromGolden(t, func(data []byte, dst any) error {
		return fastcborDecoders[reflect.TypeOf(dst).Elem()](data, dst)
	}, cases)
}
