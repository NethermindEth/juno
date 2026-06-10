package rpcv9_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	rpc "github.com/NethermindEth/juno/rpc/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// See rpc/v10/wire_parity_test.go for rationale. Same JSON shape as v10/v9.

func TestMsgToL1_JSONShape_Stable_v9(t *testing.T) {
	in := rpc.MsgToL1{
		From:    felt.NewFromUint64[felt.Felt](0xabc),
		To:      eth.AddressFromString("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		Payload: []*felt.Felt{felt.NewFromUint64[felt.Felt](1), felt.NewFromUint64[felt.Felt](2)},
	}

	raw, err := json.Marshal(in)
	require.NoError(t, err)

	const want = `{` +
		`"from_address":"0xabc",` +
		`"to_address":"0xc662c410c0ecf747543f5ba90660f6abebd9c8c4",` +
		`"payload":["0x1","0x2"]` +
		`}`
	assert.JSONEq(t, want, string(raw))

	var out rpc.MsgToL1
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, in.To, out.To)
}

func TestMsgToL1_JSONShape_OmitsFromWhenNil_v9(t *testing.T) {
	in := rpc.MsgToL1{
		To:      eth.AddressFromString("0x0000000000000000000000000000000000000001"),
		Payload: []*felt.Felt{},
	}
	raw, err := json.Marshal(in)
	require.NoError(t, err)

	const want = `{"to_address":"0x0000000000000000000000000000000000000001","payload":[]}`
	assert.JSONEq(t, want, string(raw))
}

func TestMsgFromL1_JSONShape_Stable_v9(t *testing.T) {
	one := felt.NewFromUint64[felt.Felt](1)
	two := felt.NewFromUint64[felt.Felt](2)
	in := rpc.MsgFromL1{
		From:     eth.AddressFromString("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		To:       *felt.NewFromUint64[felt.Felt](0xdef),
		Payload:  []felt.Felt{*one, *two},
		Selector: *felt.NewFromUint64[felt.Felt](0x3),
	}

	raw, err := json.Marshal(in)
	require.NoError(t, err)

	const want = `{` +
		`"from_address":"0xc662c410c0ecf747543f5ba90660f6abebd9c8c4",` +
		`"to_address":"0xdef",` +
		`"payload":["0x1","0x2"],` +
		`"entry_point_selector":"0x3"` +
		`}`
	assert.JSONEq(t, want, string(raw))

	var out rpc.MsgFromL1
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, in.From, out.From)
}
