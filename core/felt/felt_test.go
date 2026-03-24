package felt_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalJson(t *testing.T) {
	var with felt.Felt
	t.Run("with prefix 0x", func(t *testing.T) {
		assert.NoError(t, with.UnmarshalJSON([]byte("0x4437ab")))
	})

	t.Run("without prefix 0x", func(t *testing.T) {
		var without felt.Felt
		assert.NoError(t, without.UnmarshalJSON([]byte("4437ab")))
		assert.Equal(t, true, without.Equal(&with))
	})

	var failF felt.Felt

	fails := []string{
		"0x2000000000000000000000000000000000000000000000000000000000000000000",
		"0x800000000000011000000000000000000000000000000000000000000000001",
		"0xfb01012100000000000000000000000000000000000000000000000000000000",
	}

	for _, hex := range fails {
		t.Run(hex+" fails", func(t *testing.T) {
			assert.Error(t, failF.UnmarshalJSON([]byte(hex)))
		})
	}
}

func TestFeltCbor(t *testing.T) {
	val := felt.NewRandom[felt.Felt]()
	encoder.TestSymmetry(t, val)
}

func TestShortString(t *testing.T) {
	var f felt.Felt

	t.Run("less than 8 digits", func(t *testing.T) {
		_, err := f.SetString("0x1234567")
		require.NoError(t, err)
		assert.Equal(t, "0x1234567", f.ShortString())
	})

	t.Run("8 digits", func(t *testing.T) {
		_, err := f.SetString("0x12345678")
		require.NoError(t, err)
		assert.Equal(t, "0x12345678", f.ShortString())
	})

	t.Run("more than 8 digits", func(t *testing.T) {
		_, err := f.SetString("0x123456789")
		require.NoError(t, err)
		assert.Equal(t, "0x1234...6789", f.ShortString())
	})
}

// TestMarshalJSON_ValueInInterface reproduces a bug where felt types serialised
// as [4]uint64 limbs instead of hex strings. This happens because MarshalJSON
// uses a pointer receiver, but encoding/json cannot call pointer-receiver methods
// on non-addressable values (e.g. struct fields inside a value stored in `any`).
func TestMarshalJSON_ValueInInterface(t *testing.T) {
	f := felt.UnsafeFromString[felt.Felt]("0xdeadbeef")

	// Simulates how jsonrpc server stores handler results:
	//   response.Result = handlerReturnValue (stored as value in `any`)
	type rpcResponse struct {
		Result any `json:"result"`
	}

	tests := []struct {
		name  string
		input any
	}{
		{
			name: "Felt value field in struct value",
			input: struct {
				V felt.Felt `json:"v"`
			}{V: f},
		},
		{
			name: "TransactionHash value field in struct value",
			input: struct {
				V felt.TransactionHash `json:"v"`
			}{V: felt.TransactionHash(f)},
		},
		{
			name: "Hash value field in struct value",
			input: struct {
				V felt.Hash `json:"v"`
			}{V: felt.Hash(f)},
		},
		{
			name: "Address value field in struct value",
			input: struct {
				V felt.Address `json:"v"`
			}{V: felt.Address(f)},
		},
		{
			name: "ClassHash value field in struct value",
			input: struct {
				V felt.ClassHash `json:"v"`
			}{V: felt.ClassHash(f)},
		},
		{
			name: "pointer field works (control)",
			input: struct {
				V *felt.Felt `json:"v"`
			}{V: &f},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := rpcResponse{Result: tt.input}
			b, err := json.Marshal(resp)
			require.NoError(t, err)

			s := string(b)
			t.Logf("JSON output: %s", s)

			// Must contain "0xdeadbeef" as a hex string, not uint64 limbs
			assert.Contains(t, s, "0xdeadbeef",
				"expected hex string, got limbs — pointer-receiver MarshalJSON"+
					" not called on non-addressable value")
			assert.False(t, strings.Contains(s, "[") && strings.Contains(s, ","),
				"got JSON array (raw [4]uint64 limbs) instead of hex string")
		})
	}
}

func TestFeltMarshalAndUnmarshal(t *testing.T) {
	f := new(felt.Felt).SetBytes([]byte("somebytes"))

	fBytes := f.Marshal()

	f2 := new(felt.Felt)
	f2.Unmarshal(fBytes)

	assert.True(t, f2.Equal(f))
}
