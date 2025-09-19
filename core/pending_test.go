package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/types/felt"
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
	t.Run("Without PreLatest", func(t *testing.T) {
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
	})

	t.Run("With Prelatest", func(t *testing.T) {
		head0 := &core.Block{
			Header: &core.Header{
				Number:     0,
				ParentHash: &felt.Zero,
				Hash:       &felt.One,
			},
		}

		preLatest1 := core.PreLatest{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: &felt.One,
					Number:     1,
				},
			},
		}
		// Genesis case with nil parent
		preConfirmed2 := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: 2,
				},
			},
		}

		preConfirmed2.WithPreLatest(&preLatest1)
		require.True(t, preConfirmed2.Validate(head0.Header))

		// Prelatest becomes latest preLatest nullified
		preConfirmed2.WithPreLatest(nil)
		head1 := preLatest1.Block
		require.True(t, preConfirmed2.Validate(head1.Header))

		// PreConfirmed becomes head, preconfirmed not upto date
		head2 := preConfirmed2.Block
		require.False(t, preConfirmed2.Validate(head2.Header))
	})
}
