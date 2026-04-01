package sync_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/sync"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestResolvePreConfirmedBaseState(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockStateReader := mocks.NewMockStateReader(mockCtrl)
	t.Run("PreConfirmedBlock without pre-latest", func(t *testing.T) {
		// Create a pre-confirmed block without pre-latest
		preConfirmed := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: 5,
				},
			},
		}

		// Test that it uses the parent block number
		// Test that it uses the parent hash directly
		mockReader.EXPECT().StateAtBlockNumber(preConfirmed.Block.Number-1).Return(
			mockStateReader,
			func() error { return nil },
			nil,
		)
		stateReader, closer, err := sync.ResolvePreConfirmedBaseState(preConfirmed, mockReader)
		require.NoError(t, err)
		require.NotNil(t, stateReader)
		require.NotNil(t, closer)
		require.NoError(t, closer())
	})

	t.Run("PreConfirmedBlock with pre-latest", func(t *testing.T) {
		// Create a pre-confirmed block with pre-latest
		preLatest := &core.PreLatest{
			Block: &core.Block{
				Header: &core.Header{
					ParentHash: &[]felt.Felt{felt.FromUint64[felt.Felt](0x456)}[0],
				},
			},
		}
		preConfirmed := &core.PreConfirmed{
			PreLatest: preLatest,
		}

		// Test that it uses the pre-latest parent hash
		mockReader.EXPECT().StateAtBlockHash(preLatest.Block.ParentHash).Return(
			mockStateReader,
			func() error { return nil },
			nil,
		)
		stateReader, closer, err := sync.ResolvePreConfirmedBaseState(preConfirmed, mockReader)
		require.NoError(t, err)
		require.NotNil(t, stateReader)
		require.NotNil(t, closer)
		require.NoError(t, closer())
	})

	t.Run("PreConfirmedBlock genesis block", func(t *testing.T) {
		// Create a pre-confirmed block for genesis (number 0)
		preConfirmed := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: 0,
				},
			},
		}

		// Test that it uses zero hash for genesis
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(
			mockStateReader,
			func() error { return nil },
			nil,
		)
		stateReader, closer, err := sync.ResolvePreConfirmedBaseState(preConfirmed, mockReader)
		require.NoError(t, err)
		require.NotNil(t, stateReader)
		require.NotNil(t, closer)
		require.NoError(t, closer())
	})
}
