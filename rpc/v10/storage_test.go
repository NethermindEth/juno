package rpcv10_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStorageResponseFlags_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		json          string
		expected      rpc.StorageAtResponseFlags
		expectedError string
	}{
		{
			name:     "empty array",
			json:     `[]`,
			expected: rpc.StorageAtResponseFlags{IncludeLastUpdateBlock: false},
		},
		{
			name:     "with INCLUDE_LAST_UPDATE_BLOCK",
			json:     `["INCLUDE_LAST_UPDATE_BLOCK"]`,
			expected: rpc.StorageAtResponseFlags{IncludeLastUpdateBlock: true},
		},
		{
			name:          "unknown flag",
			json:          `["UNKNOWN_FLAG"]`,
			expectedError: "unknown flag: UNKNOWN_FLAG",
		},
		{
			name:          "invalid json",
			json:          `not_json`,
			expectedError: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var flags rpc.StorageAtResponseFlags
			err := json.Unmarshal([]byte(tt.json), &flags)

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, flags)
			}
		})
	}
}

func TestStorageAt(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, log)

	targetAddress := felt.FromUint64[felt.Felt](1234)
	targetSlot := felt.FromUint64[felt.Felt](5678)

	mockState := mocks.NewMockStateReader(mockCtrl)
	expectedStorage := felt.FromUint64[felt.Felt](1)

	t.Run("no flags", func(t *testing.T) {
		noFlags := rpc.StorageAtResponseFlags{}

		t.Run("empty blockchain", func(t *testing.T) {
			mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

			blockID := rpc.BlockIDLatest()
			_, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})

		t.Run("non-existent block hash", func(t *testing.T) {
			mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

			blockID := rpc.BlockIDFromHash(&felt.Zero)
			_, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})

		t.Run("non-existent block number", func(t *testing.T) {
			mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

			blockID := rpc.BlockIDFromNumber(0)
			_, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})

		t.Run("non-existent contract", func(t *testing.T) {
			mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, db.ErrKeyNotFound)

			blockID := rpc.BlockIDLatest()
			_, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
		})

		t.Run("non-existent key", func(t *testing.T) {
			mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(felt.Zero, nil)

			blockID := rpc.BlockIDLatest()
			result, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, rpcErr)
			validateStorageAtJSON(t, result, noFlags.IncludeLastUpdateBlock)
		})

		t.Run("internal error while retrieving key", func(t *testing.T) {
			internalErr := errors.New("some internal error")
			mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).
				Return(felt.Felt{}, internalErr)

			blockID := rpc.BlockIDLatest()
			_, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			assert.Equal(t, rpccore.ErrInternal.CloneWithData(internalErr), rpcErr)
		})

		t.Run("blockID - latest", func(t *testing.T) {
			mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(expectedStorage, nil)

			blockID := rpc.BlockIDLatest()
			result, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, rpcErr)
			validateStorageAtJSON(t, result, noFlags.IncludeLastUpdateBlock)
		})

		t.Run("blockID - hash", func(t *testing.T) {
			mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(expectedStorage, nil)

			blockID := rpc.BlockIDFromHash(&felt.Zero)
			result, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, rpcErr)
			assert.Equal(t, expectedStorage, result.Value)
		})

		t.Run("blockID - number", func(t *testing.T) {
			mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(expectedStorage, nil)

			blockID := rpc.BlockIDFromNumber(0)
			result, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, rpcErr)
			assert.Equal(t, expectedStorage, result.Value)
		})

		t.Run("blockID - pre_confirmed", func(t *testing.T) {
			preConfirmedStateDiff := core.EmptyStateDiff()
			preConfirmedStateDiff.
				StorageDiffs[targetAddress] = map[felt.Felt]*felt.Felt{targetSlot: &expectedStorage}
			preConfirmedStateDiff.
				DeployedContracts[targetAddress] = felt.NewFromUint64[felt.Felt](123456789)

			preConfirmed := core.PreConfirmed{
				Block: &core.Block{
					Header: &core.Header{
						Number: 2,
					},
				},
				StateUpdate: &core.StateUpdate{
					StateDiff: &preConfirmedStateDiff,
				},
			}
			mockSyncReader.EXPECT().PendingData().Return(&preConfirmed, nil)
			mockReader.EXPECT().StateAtBlockNumber(preConfirmed.Block.Number-1).
				Return(mockState, nopCloser, nil)
			preConfirmedID := rpc.BlockIDPreConfirmed()
			result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &preConfirmedID, noFlags)
			require.Nil(t, rpcErr)
			validateStorageAtJSON(t, result, noFlags.IncludeLastUpdateBlock)
		})

		t.Run("blockID - l1_accepted", func(t *testing.T) {
			l1HeadBlockNumber := uint64(10)
			mockReader.EXPECT().L1Head().Return(
				core.L1Head{
					BlockNumber: l1HeadBlockNumber,
					BlockHash:   &felt.Zero,
					StateRoot:   &felt.Zero,
				},
				nil,
			)
			mockReader.EXPECT().StateAtBlockNumber(l1HeadBlockNumber).Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&felt.Zero).Return(felt.Zero, nil)
			mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

			blockID := rpc.BlockIDL1Accepted()
			result, rpcErr := handler.StorageAt(&felt.Zero, &felt.Zero, &blockID, noFlags)
			require.Nil(t, rpcErr)
			validateStorageAtJSON(t, result, noFlags.IncludeLastUpdateBlock)
		})
	})

	t.Run("with IncludeLastUpdateBlock flag", func(t *testing.T) {
		flags := rpc.StorageAtResponseFlags{IncludeLastUpdateBlock: true}
		historyPrefix := db.ContractStorageHistoryKey(&targetAddress, &targetSlot)

		t.Run("blockID - number", func(t *testing.T) {
			blockNumber := uint64(5)
			lastUpdateBlockNum := uint64(3)

			mockReader.EXPECT().StateAtBlockNumber(blockNumber).Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(expectedStorage, nil)
			mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(
				&core.Header{Number: blockNumber}, nil,
			)
			mockReader.EXPECT().HistoryBlockNumber(historyPrefix, blockNumber).
				Return(lastUpdateBlockNum, true, nil)

			blockID := rpc.BlockIDFromNumber(blockNumber)
			result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &blockID, flags)
			require.Nil(t, rpcErr)
			validateStorageAtJSON(t, result, flags.IncludeLastUpdateBlock)
		})

		t.Run("blockID - latest", func(t *testing.T) {
			latestBlockNumber := uint64(7)
			lastUpdateBlockNum := uint64(2)

			mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(expectedStorage, nil)
			mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: latestBlockNumber}, nil)
			mockReader.EXPECT().HistoryBlockNumber(historyPrefix, latestBlockNumber).
				Return(lastUpdateBlockNum, true, nil)

			blockID := rpc.BlockIDLatest()
			result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &blockID, flags)
			require.Nil(t, rpcErr)
			validateStorageAtJSON(t, result, flags.IncludeLastUpdateBlock)
		})

		t.Run("blockID - hash", func(t *testing.T) {
			blockHash := felt.FromUint64[felt.Felt](42)
			blockNumber := uint64(6)
			lastUpdateBlockNum := uint64(1)

			mockReader.EXPECT().StateAtBlockHash(&blockHash).Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(expectedStorage, nil)
			mockReader.EXPECT().BlockHeaderByHash(&blockHash).Return(
				&core.Header{Number: blockNumber}, nil,
			)
			mockReader.EXPECT().HistoryBlockNumber(historyPrefix, blockNumber).
				Return(lastUpdateBlockNum, true, nil)

			blockID := rpc.BlockIDFromHash(&blockHash)
			result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &blockID, flags)
			require.Nil(t, rpcErr)
			validateStorageAtJSON(t, result, flags.IncludeLastUpdateBlock)
		})

		t.Run("blockID - l1_accepted", func(t *testing.T) {
			l1HeadBlockNumber := uint64(10)
			lastUpdateBlockNum := uint64(8)

			// L1Head is called twice: once in stateByBlockID and once in blockHeaderByID.
			mockReader.EXPECT().L1Head().Return(
				core.L1Head{
					BlockNumber: l1HeadBlockNumber,
					BlockHash:   &felt.Zero,
					StateRoot:   &felt.Zero,
				},
				nil,
			).Times(2)
			mockReader.EXPECT().StateAtBlockNumber(l1HeadBlockNumber).Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(expectedStorage, nil)
			mockReader.EXPECT().BlockHeaderByNumber(l1HeadBlockNumber).Return(
				&core.Header{Number: l1HeadBlockNumber}, nil,
			)
			mockReader.EXPECT().HistoryBlockNumber(historyPrefix, l1HeadBlockNumber).
				Return(lastUpdateBlockNum, true, nil)

			blockID := rpc.BlockIDL1Accepted()
			result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &blockID, flags)
			require.Nil(t, rpcErr)
			validateStorageAtJSON(t, result, flags.IncludeLastUpdateBlock)
		})

		t.Run("blockID - pre_confirmed", func(t *testing.T) {
			preConfirmedBlockNumber := uint64(3)
			lastUpdateBlockNum := uint64(1)

			preConfirmedStateDiff := core.EmptyStateDiff()
			preConfirmedStateDiff.
				StorageDiffs[targetAddress] = map[felt.Felt]*felt.Felt{targetSlot: &expectedStorage}
			preConfirmedStateDiff.
				DeployedContracts[targetAddress] = felt.NewFromUint64[felt.Felt](123456789)

			preConfirmed := core.PreConfirmed{
				Block: &core.Block{
					Header: &core.Header{
						Number: preConfirmedBlockNumber,
					},
				},
				StateUpdate: &core.StateUpdate{
					StateDiff: &preConfirmedStateDiff,
				},
			}

			// PendingData is called twice: once in PendingState (via stateByBlockID)
			// and once in blockHeaderByID.
			mockSyncReader.EXPECT().PendingData().Return(&preConfirmed, nil).Times(2)
			mockReader.EXPECT().StateAtBlockNumber(preConfirmedBlockNumber-1).
				Return(mockState, nopCloser, nil)
			mockReader.EXPECT().HistoryBlockNumber(historyPrefix, preConfirmedBlockNumber).
				Return(lastUpdateBlockNum, true, nil)

			preConfirmedID := rpc.BlockIDPreConfirmed()
			result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &preConfirmedID, flags)
			require.Nil(t, rpcErr)
			validateStorageAtJSON(t, result, flags.IncludeLastUpdateBlock)
		})

		t.Run("no history entry (storage never updated)", func(t *testing.T) {
			blockNumber := uint64(4)

			mockReader.EXPECT().StateAtBlockNumber(blockNumber).Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(expectedStorage, nil)
			mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(
				&core.Header{Number: blockNumber}, nil,
			)
			mockReader.EXPECT().HistoryBlockNumber(historyPrefix, blockNumber).
				Return(uint64(0), false, nil)

			blockID := rpc.BlockIDFromNumber(blockNumber)
			result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &blockID, flags)
			require.Nil(t, rpcErr)
			validateStorageAtJSON(t, result, flags.IncludeLastUpdateBlock)
		})
	})

	t.Run("error: internal error with data", func(t *testing.T) {
		blockNumber := uint64(3)
		dbErr := errors.New("db error")

		mockReader.EXPECT().StateAtBlockNumber(blockNumber).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, dbErr)

		blockID := rpc.BlockIDFromNumber(blockNumber)
		_, rpcErr := handler.StorageAt(
			&targetAddress, &targetSlot, &blockID, rpc.StorageAtResponseFlags{},
		)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(dbErr), rpcErr)
	})

	t.Run("error: block not found", func(t *testing.T) {
		blockNumber := uint64(99)

		mockReader.EXPECT().StateAtBlockNumber(blockNumber).Return(nil, nil, db.ErrKeyNotFound)

		blockID := rpc.BlockIDFromNumber(blockNumber)
		_, rpcErr := handler.StorageAt(
			&targetAddress, &targetSlot, &blockID, rpc.StorageAtResponseFlags{},
		)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("error: contract not found", func(t *testing.T) {
		blockNumber := uint64(99)

		mockReader.EXPECT().StateAtBlockNumber(blockNumber).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, db.ErrKeyNotFound)

		blockID := rpc.BlockIDFromNumber(blockNumber)
		_, rpcErr := handler.StorageAt(
			&targetAddress, &targetSlot, &blockID, rpc.StorageAtResponseFlags{},
		)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})
}

func validateStorageAtJSON(
	t *testing.T,
	result *rpc.StorageAtResponse,
	includeLastUpdateBlock bool,
) {
	data, err := json.Marshal(result)
	require.NoError(t, err)

	if includeLastUpdateBlock {
		dataMap := make(map[string]any)
		err = json.Unmarshal(data, &dataMap)
		require.NoError(t, err)

		assert.Equal(t, result.Value.String(), dataMap["value"])
		assert.EqualValues(t, result.LastUpdateBlock, dataMap["last_update_block"])
		assert.Len(t, dataMap, 2)
	} else {
		assert.JSONEq(t, strconv.Quote(result.Value.String()), string(data))
	}

	var newResult rpc.StorageAtResponse
	err = json.Unmarshal(data, &newResult)
	require.NoError(t, err)
	assert.Exactly(t, *result, newResult)
}
