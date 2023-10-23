package syncl1_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/l1data"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/syncl1"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestFetcher(t *testing.T) {
	encodedDiffs := getEncodedDiffs(t)
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	for i, expectedDiff := range encodedDiffs {
		height := uint64(i)
		updateFetcher := mocks.NewMockStateUpdateLogFetcher(ctrl)
		expectedLog := &l1data.LogStateUpdate{
			GlobalRoot:  new(big.Int).SetUint64(height + 1),
			BlockNumber: new(big.Int).SetUint64(height),
			Raw: types.Log{
				BlockNumber: height,
				TxIndex:     uint(i),
			},
		}
		updateFetcher.EXPECT().
			StateUpdateLogs(gomock.Any(), height, height).
			Return([]*l1data.LogStateUpdate{expectedLog}, nil).
			Times(1)
		diffFetcher := mocks.NewMockStateDiffFetcher(ctrl)
		diffFetcher.EXPECT().
			StateDiff(gomock.Any(), uint(i), height).
			Return(expectedDiff, nil).
			Times(1)

		require.NoError(t, syncl1.NewIterator(updateFetcher, diffFetcher, func(encodedDiff []*big.Int, log *l1data.LogStateUpdate) error {
			t.Helper()
			require.Equal(t, expectedDiff, encodedDiff)
			require.Equal(t, expectedLog, log)
			return nil
		}).Run(ctx, height, height, height))
	}
}
