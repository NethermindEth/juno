package eth_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hashCorpus returns a deterministic sample of hashes for parity tests
// against go-ethereum's common.Hash. The geth imports in this file are
// removed in PR5.
func hashCorpus(t *testing.T) []eth.Hash {
	t.Helper()
	// LogMessageToL2 event signature hash — used by rpc/v*/l1.go.
	const logMsgSigHex = "0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b"
	logMsgSig := eth.HashFromString(logMsgSigHex)
	raws := bytePatterns(eth.HashLength, 3, 4, logMsgSig.Bytes())
	out := make([]eth.Hash, len(raws))
	for i, b := range raws {
		out[i] = eth.HashFromBytes(b)
	}
	return out
}

func TestHash_HashFromString_GethParity(t *testing.T) {
	cases := []string{
		"0x0000000000000000000000000000000000000000000000000000000000000000",
		"0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b",
		"0xDB80DD488ACF86D17C747445B0EABB5D57C541D3BD7B6B87AF987858E5066B2B", // upper case
		"db80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b",   // no prefix
		"0x1",                           // short, left-padded
		"0xabcd",                        // short, left-padded
		"",                              // empty
		"0x" + strings.Repeat("ff", 40), // too long, left-cropped
		"0xZZ",                          // invalid hex - geth silently returns zero
		"not-hex",                       // non-hex without prefix
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			ours := eth.HashFromString(in)
			geth := gethcommon.HexToHash(in)
			assert.Equal(t, geth.Bytes(), ours.Bytes())
		})
	}
}

func TestHash_HashFromBytes_GethParity(t *testing.T) {
	cases := [][]byte{
		nil,
		{},
		{0x01},
		bytes.Repeat([]byte{0xab}, 32),
		bytes.Repeat([]byte{0xcd}, 40), // too long, left-cropped
	}
	for _, in := range cases {
		ours := eth.HashFromBytes(in)
		geth := gethcommon.BytesToHash(in)
		assert.Equal(t, geth.Bytes(), ours.Bytes())
	}
}

func TestHash_Bytes_GethParity(t *testing.T) {
	for _, raw := range hashCorpus(t) {
		ours := eth.HashFromBytes(raw[:])
		geth := gethcommon.BytesToHash(raw[:])
		assert.Equal(t, geth.Bytes(), ours.Bytes())
		assert.Len(t, ours.Bytes(), eth.HashLength)
	}
}

func TestHash_Hex_GethParity(t *testing.T) {
	for _, raw := range hashCorpus(t) {
		ours := eth.HashFromBytes(raw[:])
		geth := gethcommon.BytesToHash(raw[:])
		// geth's Hash.Hex() is lowercase 0x-hex (unlike Address.Hex which is EIP-55).
		assert.Equal(t, geth.Hex(), ours.Hex())
	}
}

func TestHash_MarshalJSON_GethParity(t *testing.T) {
	for _, raw := range hashCorpus(t) {
		ours := eth.HashFromBytes(raw[:])
		geth := gethcommon.BytesToHash(raw[:])

		oJSON, err := json.Marshal(ours)
		require.NoError(t, err)
		gJSON, err := json.Marshal(geth)
		require.NoError(t, err)

		assert.Equal(t, gJSON, oJSON,
			"hash JSON mismatch for %x: geth=%s ours=%s", raw, gJSON, oJSON)
	}
}

func TestHash_UnmarshalJSON_RoundTrip(t *testing.T) {
	for _, raw := range hashCorpus(t) {
		ours := eth.HashFromBytes(raw[:])
		geth := gethcommon.BytesToHash(raw[:])

		gJSON, err := json.Marshal(geth)
		require.NoError(t, err)
		oJSON, err := json.Marshal(ours)
		require.NoError(t, err)

		// Our impl decodes geth output.
		var fromGeth eth.Hash
		require.NoError(t, json.Unmarshal(gJSON, &fromGeth))
		assert.Equal(t, ours, fromGeth)

		// Geth decodes our output.
		var fromOurs gethcommon.Hash
		require.NoError(t, json.Unmarshal(oJSON, &fromOurs))
		assert.Equal(t, geth, fromOurs)
	}
}

func TestHash_UnmarshalJSON_AcceptsUpperHex(t *testing.T) {
	upper := []byte(`"0xDB80DD488ACF86D17C747445B0EABB5D57C541D3BD7B6B87AF987858E5066B2B"`)
	var ours eth.Hash
	var geth gethcommon.Hash
	require.NoError(t, json.Unmarshal(upper, &ours))
	require.NoError(t, json.Unmarshal(upper, &geth))
	assert.Equal(t, geth.Bytes(), ours.Bytes())
}

func TestHash_UnmarshalJSON_Errors(t *testing.T) {
	cases := []string{
		`"db80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b"`,     // missing prefix
		`"0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2"`,    // short
		`"0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2bff"`, // long
		`"0xZZ80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b"`,   // non-hex
		`null`,
		`0`,
		``,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			var ours eth.Hash
			var geth gethcommon.Hash
			oErr := json.Unmarshal([]byte(in), &ours)
			gErr := json.Unmarshal([]byte(in), &geth)
			require.Error(t, gErr, "geth should reject %q", in)
			require.Error(t, oErr, "ours should reject %q", in)
		})
	}
}

func TestHash_Cmp_GethParity(t *testing.T) {
	corpus := hashCorpus(t)
	for i := range corpus {
		for j := range corpus {
			ours := eth.HashFromBytes(corpus[i][:]).Cmp(eth.HashFromBytes(corpus[j][:]))
			geth := gethcommon.BytesToHash(corpus[i][:]).Cmp(gethcommon.BytesToHash(corpus[j][:]))
			assert.Equal(t, geth, ours, "Cmp(%d, %d)", i, j)
		}
	}
}

// TestHash_UnmarshalJSON_OracleParity feeds a wide input matrix to both
// our and geth's UnmarshalJSON; any divergence in accept-vs-reject is a
// regression. Whenever both accept, the decoded bytes must also match.
func TestHash_UnmarshalJSON_OracleParity(t *testing.T) {
	const validHex = "db80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b" // 64 chars
	inputs := []string{
		// Valid: lower / upper / mixed hex, 0x and 0X prefix.
		`"0x` + validHex + `"`,
		`"0X` + validHex + `"`,
		`"0x` + strings.ToUpper(validHex) + `"`,
		`"0X` + strings.ToUpper(validHex) + `"`,
		`"0xDB80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b"`,
		`"0x0000000000000000000000000000000000000000000000000000000000000000"`,
		`"0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"`,

		// Invalid.
		`"` + validHex + `"`,         // no prefix
		`"0x"`,                       // just prefix
		`"0x` + validHex[:63] + `"`,  // odd length, short
		`"0x` + validHex[:62] + `"`,  // even length, short
		`"0x` + validHex + `ab"`,     // too long
		`"0xZZ` + validHex[2:] + `"`, // non-hex
		`""`,
		`null`,
		`123`,
		`true`,
		`[]`,
		`{}`,
		``,
		`"`,
	}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			var ours eth.Hash
			var geth gethcommon.Hash
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

func TestHash_SetBytes_Cropping(t *testing.T) {
	long := bytes.Repeat([]byte{0xab}, 40)
	var ours eth.Hash
	var geth gethcommon.Hash
	ours.SetBytes(long)
	geth.SetBytes(long)
	assert.Equal(t, geth.Bytes(), ours.Bytes())
}
