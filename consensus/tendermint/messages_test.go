package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

type coreBlock struct {
	core.Block
}

func (c coreBlock) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(1)
}

var _ Hashable[felt.Felt] = coreBlock{}

// Todo: fix
func TestProposalMarshalUnmarshalCBOR(t *testing.T) {
	val := coreBlock{}
	var height height
	var round round
	var sender felt.Felt
	sender = *new(felt.Felt).SetUint64(1)
	original := Proposal[coreBlock, felt.Felt, felt.Felt]{
		MessageHeader: MessageHeader[felt.Felt]{
			Height: height,
			Round:  round,
			Sender: sender,
		},
		ValidRound: round,
		Value:      &val,
	}

	// Marshal
	encoded, err := original.MarshalCBOR()
	require.NoError(t, err, "MarshalCBOR failed")

	// Unmarshal
	var decoded Proposal[coreBlock, felt.Felt, felt.Felt]
	err = decoded.UnmarshalCBOR(encoded)
	require.NoError(t, err, "UnmarshalCBOR failed")
	require.Equal(t, original.Height, decoded.Height, "Height mismatch")
	require.Equal(t, original.Round, decoded.Round, "Round mismatch")
	require.Equal(t, original.Sender, decoded.Sender, "Sender mismatch")
	require.Equal(t, original.ValidRound, decoded.ValidRound, "ValidRound mismatch")
	require.NotNil(t, decoded.Value, "Value is nil after decode")
	require.Equal(t, *original.Value, *decoded.Value, "Value mismatch")
}
