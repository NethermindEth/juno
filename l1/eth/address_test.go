package eth_test

import (
	"bytes"
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addressCorpus returns a deterministic, broad sample of 20-byte values for
// parity tests against go-ethereum's common.Address. The geth package is still
// in go.mod through PR1-PR4; these imports are removed in PR5.
func addressCorpus(t *testing.T) [][20]byte {
	t.Helper()
	var zero, allFF, allAA, ascending [20]byte
	for i := range allFF {
		allFF[i] = 0xff
		allAA[i] = 0xaa
		ascending[i] = byte(i)
	}
	out := [][20]byte{zero, allFF, allAA, ascending}

	// real mainnet Starknet core contract address.
	starknetCore := gethcommon.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4")
	out = append(out, starknetCore)

	rng := rand.New(rand.NewPCG(1, 2))
	for range 32 {
		var a [20]byte
		for j := range a {
			a[j] = byte(rng.Uint32())
		}
		out = append(out, a)
	}
	return out
}

func TestAddress_HexToAddress_GethParity(t *testing.T) {
	cases := []string{
		"0x0000000000000000000000000000000000000000",
		"0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4",
		"0xC662C410C0ECF747543F5BA90660F6ABEBD9C8C4", // upper case
		"c662c410c0ecf747543f5ba90660f6abebd9c8c4",   // no prefix
		"0x1",                           // short -> left padded
		"0xabcd",                        // short -> left padded
		"",                              // empty
		"0x" + strings.Repeat("ff", 25), // too long -> left cropped
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			ours := eth.HexToAddress(in)
			geth := gethcommon.HexToAddress(in)
			assert.Equal(t, geth.Bytes(), ours.Bytes())
		})
	}
}

func TestAddress_BytesToAddress_GethParity(t *testing.T) {
	cases := [][]byte{
		nil,
		{},
		{0x01},
		{0x01, 0x02, 0x03},
		bytes.Repeat([]byte{0xab}, 20),
		bytes.Repeat([]byte{0xcd}, 25), // too long, left cropped
	}
	for _, in := range cases {
		ours := eth.BytesToAddress(in)
		geth := gethcommon.BytesToAddress(in)
		assert.Equal(t, geth.Bytes(), ours.Bytes())
	}
}

func TestAddress_Bytes_GethParity(t *testing.T) {
	for _, raw := range addressCorpus(t) {
		ours := eth.BytesToAddress(raw[:])
		geth := gethcommon.BytesToAddress(raw[:])
		assert.Equal(t, geth.Bytes(), ours.Bytes())
		assert.Len(t, ours.Bytes(), eth.AddressLength)
	}
}

func TestAddress_MarshalJSON_GethParity(t *testing.T) {
	for _, raw := range addressCorpus(t) {
		ours := eth.BytesToAddress(raw[:])
		geth := gethcommon.BytesToAddress(raw[:])

		oJSON, err := json.Marshal(ours)
		require.NoError(t, err)
		gJSON, err := json.Marshal(geth)
		require.NoError(t, err)

		// Byte-for-byte identical to geth's JSON output.
		assert.Equal(t, gJSON, oJSON,
			"address JSON mismatch for %x: geth=%s ours=%s", raw, gJSON, oJSON)
	}
}

func TestAddress_UnmarshalJSON_RoundTrip(t *testing.T) {
	for _, raw := range addressCorpus(t) {
		ours := eth.BytesToAddress(raw[:])
		geth := gethcommon.BytesToAddress(raw[:])

		gJSON, err := json.Marshal(geth)
		require.NoError(t, err)
		oJSON, err := json.Marshal(ours)
		require.NoError(t, err)

		// Our impl decodes geth output.
		var fromGeth eth.Address
		require.NoError(t, json.Unmarshal(gJSON, &fromGeth))
		assert.Equal(t, ours, fromGeth)

		// Geth decodes our output.
		var fromOurs gethcommon.Address
		require.NoError(t, json.Unmarshal(oJSON, &fromOurs))
		assert.Equal(t, geth, fromOurs)
	}
}

func TestAddress_UnmarshalJSON_AcceptsUpperHex(t *testing.T) {
	upper := []byte(`"0xC662C410C0ECF747543F5BA90660F6ABEBD9C8C4"`)
	var ours eth.Address
	var geth gethcommon.Address
	require.NoError(t, json.Unmarshal(upper, &ours))
	require.NoError(t, json.Unmarshal(upper, &geth))
	assert.Equal(t, geth.Bytes(), ours.Bytes())
}

func TestAddress_UnmarshalJSON_Errors(t *testing.T) {
	cases := []string{
		`"c662c410c0ecf747543f5ba90660f6abebd9c8c4"`,    // missing 0x prefix
		`"0xc662c410c0ecf747543f5ba90660f6abebd9c8"`,    // too short
		`"0xc662c410c0ecf747543f5ba90660f6abebd9c8c4f"`, // too long
		`"0xZZZZZZ410c0ecf747543f5ba90660f6abebd9c8c4"`, // non-hex
		`null`, // not a string
		`0`,    // not a string
		``,     // empty
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			var ours eth.Address
			var geth gethcommon.Address
			oErr := json.Unmarshal([]byte(in), &ours)
			gErr := json.Unmarshal([]byte(in), &geth)
			// Both must reject (we don't care that the error message matches,
			// only that error vs no-error agrees).
			require.Error(t, gErr, "geth should reject %q", in)
			require.Error(t, oErr, "ours should reject %q", in)
		})
	}
}

// TestAddress_UnmarshalJSON_OracleParity feeds a wide matrix of inputs to both
// our and geth's UnmarshalJSON; any divergence in accept-vs-reject is a
// regression. Whenever both accept, the decoded bytes must also match.
func TestAddress_UnmarshalJSON_OracleParity(t *testing.T) {
	const validHex = "c662c410c0ecf747543f5ba90660f6abebd9c8c4" // 40 chars
	inputs := []string{
		// Valid: lower / upper / mixed hex, 0x and 0X prefix.
		`"0x` + validHex + `"`,
		`"0X` + validHex + `"`,
		`"0x` + strings.ToUpper(validHex) + `"`,
		`"0X` + strings.ToUpper(validHex) + `"`,
		`"0xC662c410C0ECf747543f5bA90660f6ABeBD9C8c4"`,
		`"0x0000000000000000000000000000000000000000"`,
		`"0xffffffffffffffffffffffffffffffffffffffff"`,

		// Invalid: missing prefix / wrong length / non-hex / non-string.
		`"` + validHex + `"`,                  // no prefix
		`"` + strings.ToUpper(validHex) + `"`, // no prefix, upper
		`"0x"`,                                // just prefix
		`"0x` + validHex[:39] + `"`,           // odd length, short
		`"0x` + validHex[:38] + `"`,           // even length, short
		`"0x` + validHex + `ab"`,              // too long
		`"0xZZ` + validHex[2:] + `"`,          // non-hex char
		`""`,                                  // empty string
		`null`,                                // JSON null
		`123`,                                 // JSON number
		`true`,                                // JSON bool
		`[]`,                                  // JSON array
		`{}`,                                  // JSON object
		``,                                    // empty input
		`"`,                                   // single quote
	}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			var ours eth.Address
			var geth gethcommon.Address
			oErr := json.Unmarshal([]byte(in), &ours)
			gErr := json.Unmarshal([]byte(in), &geth)

			if (gErr == nil) != (oErr == nil) {
				t.Fatalf("accept-reject divergence for %q: geth err=%v, ours err=%v", in, gErr, oErr)
			}
			if gErr == nil {
				assert.Equal(t, geth.Bytes(), ours.Bytes(), "decoded bytes differ for %q", in)
			}
		})
	}
}

func TestAddress_SetBytes_Cropping(t *testing.T) {
	// SetBytes left-crops oversized input — match geth.
	long := bytes.Repeat([]byte{0xab}, 25)
	var ours eth.Address
	var geth gethcommon.Address
	ours.SetBytes(long)
	geth.SetBytes(long)
	assert.Equal(t, geth.Bytes(), ours.Bytes())
}
