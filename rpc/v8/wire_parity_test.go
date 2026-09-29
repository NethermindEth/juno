package rpcv8_test

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// See rpc/v10/wire_parity_test.go for rationale. Same JSON shape as v10/v9.

func TestMsgToL1_JSONShape_Stable_v8(t *testing.T) {
	in := rpc.MsgToL1{
		From:    felt.NewFromUint64[felt.Felt](0xabc),
		To:      felt.UnsafeFromString[felt.Felt]("0xc662c410c0ecf747543f5ba90660f6abebd9c8c4"),
		Payload: []felt.Felt{felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2)},
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

func TestMsgToL1_JSONShape_OmitsFromWhenNil_v8(t *testing.T) {
	// MsgToL1.From has omitempty; a nil pointer must not emit the key.
	in := rpc.MsgToL1{
		To:      felt.FromUint64[felt.Felt](1),
		Payload: []felt.Felt{},
	}
	raw, err := json.Marshal(in)
	require.NoError(t, err)

	const want = `{"to_address":"0x1","payload":[]}`
	assert.JSONEq(t, want, string(raw))
}

// MSG_TO_L1.to_address is a FELT in the spec, not an ETH_ADDRESS, so it must not carry
// zero padding. Sepolia blocks 32 and 48 carry short ones: 0x1 and a 39-digit address.
func TestMsgToL1_ToAddress_IsFelt_v8(t *testing.T) {
	feltPattern := regexp.MustCompile(`^0x(0|[a-fA-F1-9][a-fA-F0-9]{0,62})$`)

	// Same conversion AdaptReceipt applies to core's 20-byte eth.Address.
	leadingZeroAddr := eth.AddressFromString("0x0c5CC7CD1d4E6feeBAC176BaEc70Ff59F5C3BAa5")
	in := rpc.MsgToL1{
		To:      felt.FromBytes[felt.Felt](leadingZeroAddr.Bytes()),
		Payload: []felt.Felt{},
	}
	raw, err := json.Marshal(in)
	require.NoError(t, err)

	const want = `{"to_address":"0xc5cc7cd1d4e6feebac176baec70ff59f5c3baa5","payload":[]}`
	assert.JSONEq(t, want, string(raw))

	var got struct {
		To string `json:"to_address"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Regexp(t, feltPattern, got.To)

	var out rpc.MsgToL1
	require.NoError(t, json.Unmarshal([]byte(`{"to_address":"0x1","payload":[]}`), &out))
	assert.Equal(t, felt.FromUint64[felt.Felt](1), out.To)
}

func TestMsgFromL1_JSONShape_Stable_v8(t *testing.T) {
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
