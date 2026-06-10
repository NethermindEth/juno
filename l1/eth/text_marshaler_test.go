package eth_test

import (
	"encoding"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the parity claim: eth.Hash / eth.Address must implement
// encoding.TextMarshaler / encoding.TextUnmarshaler the same way geth does,
// so that downstream consumers (json map keys, viper/mapstructure decoders)
// behave identically.

// --- Interface implementation -------------------------------------------

func TestHash_ImplementsTextCodec(t *testing.T) {
	var h eth.Hash
	var _ encoding.TextMarshaler = h
	var _ encoding.TextUnmarshaler = &h
}

func TestAddress_ImplementsTextCodec(t *testing.T) {
	var a eth.Address
	var _ encoding.TextMarshaler = a
	var _ encoding.TextUnmarshaler = &a
}

// --- MarshalText / UnmarshalText geth parity ----------------------------

func TestHash_MarshalText_GethParity(t *testing.T) {
	for _, raw := range hashCorpus(t) {
		ours := eth.HashFromBytes(raw[:])
		geth := gethcommon.BytesToHash(raw[:])

		oTxt, err := ours.MarshalText()
		require.NoError(t, err)
		gTxt, err := geth.MarshalText()
		require.NoError(t, err)

		assert.Equal(t, gTxt, oTxt)
	}
}

func TestAddress_MarshalText_GethParity(t *testing.T) {
	for _, raw := range addressCorpus(t) {
		ours := eth.AddressFromBytes(raw[:])
		geth := gethcommon.BytesToAddress(raw[:])

		oTxt, err := ours.MarshalText()
		require.NoError(t, err)
		gTxt, err := geth.MarshalText()
		require.NoError(t, err)

		assert.Equal(t, gTxt, oTxt)
	}
}

func TestHash_UnmarshalText_RoundTrip(t *testing.T) {
	for _, raw := range hashCorpus(t) {
		geth := gethcommon.BytesToHash(raw[:])
		txt, err := geth.MarshalText()
		require.NoError(t, err)

		var ours eth.Hash
		require.NoError(t, ours.UnmarshalText(txt))
		assert.Equal(t, geth.Bytes(), ours.Bytes())
	}
}

func TestAddress_UnmarshalText_RoundTrip(t *testing.T) {
	for _, raw := range addressCorpus(t) {
		geth := gethcommon.BytesToAddress(raw[:])
		txt, err := geth.MarshalText()
		require.NoError(t, err)

		var ours eth.Address
		require.NoError(t, ours.UnmarshalText(txt))
		assert.Equal(t, geth.Bytes(), ours.Bytes())
	}
}

// --- Concrete failure modes from the PR review note ---------------------

// JSON map-key marshal: encoding/json requires the key type to implement
// encoding.TextMarshaler. With only MarshalJSON, json.Marshal of a map
// keyed by Address/Hash fails outright. Geth supports it.
func TestHash_MapKey_JSONMarshal(t *testing.T) {
	m := map[eth.Hash]int{
		eth.HashFromBytes([]byte{0xab, 0xcd}): 1,
	}
	out, err := json.Marshal(m)
	require.NoError(t, err, "json.Marshal of map keyed by eth.Hash must work")

	gm := map[gethcommon.Hash]int{
		gethcommon.BytesToHash([]byte{0xab, 0xcd}): 1,
	}
	gOut, err := json.Marshal(gm)
	require.NoError(t, err)

	assert.Equal(t, gOut, out, "map[Hash]int JSON output must match geth")
}

func TestAddress_MapKey_JSONMarshal(t *testing.T) {
	m := map[eth.Address]int{
		eth.AddressFromBytes([]byte{0xab, 0xcd}): 1,
	}
	out, err := json.Marshal(m)
	require.NoError(t, err, "json.Marshal of map keyed by eth.Address must work")

	gm := map[gethcommon.Address]int{
		gethcommon.BytesToAddress([]byte{0xab, 0xcd}): 1,
	}
	gOut, err := json.Marshal(gm)
	require.NoError(t, err)

	assert.Equal(t, gOut, out, "map[Address]int JSON output must match geth")
}

// mapstructure (used by viper for YAML/TOML/env config decoding) routes
// string -> struct-field conversion through TextUnmarshallerHookFunc, which
// requires encoding.TextUnmarshaler on the target type.
func TestAddress_MapstructureDecode(t *testing.T) {
	type cfg struct {
		CoreContract eth.Address `mapstructure:"core_contract"`
	}
	in := map[string]any{
		"core_contract": "0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4",
	}

	var out cfg
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.TextUnmarshallerHookFunc(),
		Result:     &out,
	})
	require.NoError(t, err)
	require.NoError(t, dec.Decode(in))

	want := gethcommon.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4")
	assert.Equal(t, want.Bytes(), out.CoreContract.Bytes())
}

func TestHash_MapstructureDecode(t *testing.T) {
	type cfg struct {
		Topic eth.Hash `mapstructure:"topic"`
	}
	in := map[string]any{
		"topic": "0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b",
	}

	var out cfg
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.TextUnmarshallerHookFunc(),
		Result:     &out,
	})
	require.NoError(t, err)
	require.NoError(t, dec.Decode(in))

	want := gethcommon.HexToHash("0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b")
	assert.Equal(t, want.Bytes(), out.Topic.Bytes())
}
