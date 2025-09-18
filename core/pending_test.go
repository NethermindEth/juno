package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestPendingValidate(t *testing.T) {
	pending0 := &core.Pending{
		Block: &core.Block{
			Header: &core.Header{
				ParentHash: &felt.Zero,
				Number:     0,
			},
		},
	}

	require.True(t, pending0.Validate(nil))

	// Pending becomes head
	head0 := &core.Block{
		Header: &core.Header{
			ParentHash: &felt.Zero,
			Hash:       &felt.One,
			Number:     0,
		},
	}

	require.False(t, pending0.Validate(head0.Header))

	// Pending for head
	pending1 := &core.Pending{
		Block: &core.Block{
			Header: &core.Header{
				ParentHash: &felt.One,
				Number:     1,
			},
		},
	}

	require.True(t, pending1.Validate(head0.Header))
}

func TestPreConfirmedValidate(t *testing.T) {
	// Genesis case with nil parent
	preConfirmed0 := &core.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: 0,
			},
		},
	}

	require.True(t, preConfirmed0.Validate(nil))

	// PreConfirmed becomes head
	head0 := &core.Block{
		Header: &core.Header{
			Number:     0,
			ParentHash: &felt.Zero,
			Hash:       &felt.One,
		},
	}

	require.False(t, preConfirmed0.Validate(head0.Header))

	// PreConfirmed for head
	preConfirmed1 := &core.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: 1,
			},
		},
	}

	require.True(t, preConfirmed1.Validate(head0.Header))
}
