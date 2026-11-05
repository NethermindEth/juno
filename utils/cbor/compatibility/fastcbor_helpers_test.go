package cbor_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// compare reads data into a T with fastcbor and with fxamacker. Where fastcbor reads, fxamacker
// must read the same value. Where fastcbor returns an error, which leaves T zero, it is free to
// refuse and the caller reads with fxamacker. It reports whether fastcbor refused what fxamacker
// reads.
func compare[T any](t *testing.T, data []byte) (refused bool) {
	t.Helper()
	var want, got T
	wantErr := fxamacker.Unmarshal(data, &want)
	gotErr := fastcbor.Unmarshal(data, &got)
	if gotErr != nil {
		require.Zero(t, got, "a failed read left a value behind")
		return wantErr == nil
	}
	require.NoError(t, wantErr, "fastcbor reads what fxamacker rejects")
	require.Equal(t, want, got)
	return false
}

// readByFastcbor is compare that also requires that fastcbor read the value.
func readByFastcbor[T any](t *testing.T, data []byte) {
	t.Helper()
	require.False(t, compare[T](t, data), "fastcbor refused what fxamacker reads")
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := fxamacker.Marshal(v)
	require.NoError(t, err)
	return data
}
