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
	t.Run("with prefix 0x", func(t *testing.T) {
		var f felt.Felt
		assert.NoError(t, f.UnmarshalJSON([]byte(`"0x4437ab"`)))
	})

	var failF felt.Felt

	fails := []string{
		"4437ab",
		"123456",
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

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name   string
		hex    string
		expect string
	}{
		{"zero", "0x0", `"0x0"`},
		{"one", "0x1", `"0x1"`},
		{"single digit", "0xf", `"0xf"`},
		{"two digits", "0xff", `"0xff"`},
		{"leading nibble < 0x10", "0xa", `"0xa"`},
		{"leading nibble 0x0a boundary", "0xa0", `"0xa0"`},
		{"small value", "0xdeadbeef", `"0xdeadbeef"`},
		{
			"typical address", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
			`"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"`,
		},
		{
			"max 252-bit value", "0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			`"0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"`,
		},
		{"power of two", "0x100", `"0x100"`},
		{"0x10 boundary", "0x10", `"0x10"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := felt.UnsafeFromString[felt.Felt](tc.hex)
			got, err := f.MarshalJSON()
			require.NoError(t, err)
			assert.Equal(t, tc.expect, string(got))
		})
	}
}

func TestUnmarshalJSON(t *testing.T) {
	t.Run("valid values", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
			hex   string
		}{
			{"zero", `"0x0"`, "0x0"},
			{"zero with leading zeros", `"0x0000"`, "0x0"},
			{"one", `"0x1"`, "0x1"},
			{"small", `"0xdeadbeef"`, "0xdeadbeef"},
			{"uppercase X prefix", `"0Xdeadbeef"`, "0xdeadbeef"},
			{"mixed case hex digits", `"0xDeAdBeEf"`, "0xdeadbeef"},
			{
				"64 hex digits with leading zero (padded address)",
				`"0x041d5da3fe4b9a6c0aef883cfec0d45436b51128e1adae80340a941b5f905db6"`,
				"0x41d5da3fe4b9a6c0aef883cfec0d45436b51128e1adae80340a941b5f905db6",
			},
			{
				"max valid felt (P-1)",
				`"0x800000000000011000000000000000000000000000000000000000000000000"`,
				"0x800000000000011000000000000000000000000000000000000000000000000",
			},
			{"odd number of hex digits", `"0xabc"`, "0xabc"},
			{"single digit", `"0x5"`, "0x5"},
			{"leading zero nibble value", `"0x0a"`, "0xa"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				var f felt.Felt
				err := f.UnmarshalJSON([]byte(tc.input))
				require.NoError(t, err)
				assert.Equal(t, tc.hex, f.String())
			})
		}
	})

	t.Run("invalid values", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
		}{
			{"no quotes", `0xdeadbeef`},
			{"no prefix", `"deadbeef"`},
			{"no hex prefix", `"4437ab"`},
			{"empty hex", `"0x"`},
			{"only quotes", `""`},
			{
				"65 hex digits (exceeds 32 bytes)",
				`"0x10000000000000000000000000000000000000000000000000000000000000000"`,
			},
			{
				"field modulus P (invalid canonical)",
				`"0x0800000000000011000000000000000000000000000000000000000000000001"`,
			},
			{
				"above modulus with leading zero",
				`"0x0fb01012100000000000000000000000000000000000000000000000000000000"`,
			},
			{"invalid hex character", `"0xdeadgbeef"`},
			{"spaces in hex", `"0xdead beef"`},
			{"negative", `"-0x1"`},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				var f felt.Felt
				assert.Error(t, f.UnmarshalJSON([]byte(tc.input)))
			})
		}
	})
}

func TestJSONRoundTrip(t *testing.T) {
	values := []string{
		"0x0",
		"0x1",
		"0xa",
		"0xf",
		"0x10",
		"0xff",
		"0x100",
		"0xdeadbeef",
		"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
		"0x800000000000011000000000000000000000000000000000000000000000000",
	}

	for _, hex := range values {
		t.Run(hex, func(t *testing.T) {
			original := felt.UnsafeFromString[felt.Felt](hex)

			marshalled, err := json.Marshal(&original)
			require.NoError(t, err)

			var decoded felt.Felt
			require.NoError(t, json.Unmarshal(marshalled, &decoded))

			assert.True(t, original.Equal(&decoded),
				"round-trip failed: %s → %s → %s", hex, string(marshalled), decoded.String())
		})
	}
}

func TestJSONRoundTripRandom(t *testing.T) {
	for range 1000 {
		original := felt.NewRandom[felt.Felt]()

		marshalled, err := json.Marshal(&original)
		require.NoError(t, err)

		var decoded felt.Felt
		require.NoError(t, json.Unmarshal(marshalled, &decoded))

		assert.True(t, original.Equal(&decoded),
			"round-trip failed for random felt: marshalled=%s", string(marshalled))
	}
}
