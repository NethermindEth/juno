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
		expected      rpc.StorageResponseFlags
		expectedError string
	}{
		{
			name:     "empty array",
			json:     `[]`,
			expected: rpc.StorageResponseFlags{IncludeLastUpdateBlock: false},
		},
		{
			name:     "with INCLUDE_LAST_UPDATE_BLOCK",
			json:     `["INCLUDE_LAST_UPDATE_BLOCK"]`,
			expected: rpc.StorageResponseFlags{IncludeLastUpdateBlock: true},
		},
		{
			name:          "unknown flag",
			json:          `["UNKNOWN_FLAG"]`,
			expectedError: "unknown storage response flag: UNKNOWN_FLAG",
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
			var flags rpc.StorageResponseFlags
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

func TestStorageAtResult_MarshalJSON(t *testing.T) {
	t.Parallel()

	value := "0xdeadbeef"
	valueFelt := felt.UnsafeFromString[felt.Felt](value)

	t.Run("includeLastUpdateBlock == false", func(t *testing.T) {
		t.Parallel()
		result := rpc.StorageAtResult{Value: valueFelt, LastUpdateBlock: 1234}
		result.IncludeLastUpdateBlock(new(false))

		data, err := json.Marshal(&result)
		require.NoError(t, err)

		assert.JSONEq(t, strconv.Quote(value), string(data))
	})

	t.Run("includeLastUpdateBlock == true", func(t *testing.T) {
		t.Parallel()
		result := rpc.StorageAtResult{Value: valueFelt, LastUpdateBlock: 0}
		result.IncludeLastUpdateBlock(new(true))

		data, err := json.Marshal(&result)
		require.NoError(t, err)

		expected := `{"value":"` + value + `","last_update_block":0}`
		assert.JSONEq(t, expected, string(data))
	})

	t.Run("marshal then unmarshal plain felt round-trips", func(t *testing.T) {
		t.Parallel()
		original := rpc.StorageAtResult{Value: valueFelt}

		data, err := json.Marshal(&original)
		require.NoError(t, err)

		var restored rpc.StorageAtResult
		require.NoError(t, json.Unmarshal(data, &restored))

		assert.Equal(t, valueFelt, restored.Value)
		assert.False(t, restored.IncludeLastUpdateBlock(nil))
		assert.Zero(t, restored.LastUpdateBlock)
	})

	t.Run("marshal then unmarshal object round-trips", func(t *testing.T) {
		t.Parallel()
		original := rpc.StorageAtResult{Value: valueFelt, LastUpdateBlock: 7}
		original.IncludeLastUpdateBlock(new(true))

		data, err := json.Marshal(&original)
		require.NoError(t, err)

		var restored rpc.StorageAtResult
		require.NoError(t, json.Unmarshal(data, &restored))

		assert.Equal(t, valueFelt, restored.Value)
		assert.Equal(t, original.LastUpdateBlock, restored.LastUpdateBlock)
		assert.True(t, restored.IncludeLastUpdateBlock(nil))
	})
}

func TestStorageAtResult_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	value := "0xdeadbeef"
	valueFelt := felt.UnsafeFromString[felt.Felt](value)

	t.Run("plain felt string", func(t *testing.T) {
		t.Parallel()
		var result rpc.StorageAtResult
		err := json.Unmarshal([]byte(strconv.Quote(value)), &result)
		require.NoError(t, err)

		assert.Equal(t, valueFelt, result.Value)
		assert.Zero(t, result.LastUpdateBlock)
		assert.False(t, result.IncludeLastUpdateBlock(nil))
	})

	t.Run("object with value and last_update_block", func(t *testing.T) {
		t.Parallel()
		var result rpc.StorageAtResult
		err := json.Unmarshal([]byte(`{"value":"`+value+`","last_update_block":5}`), &result)
		require.NoError(t, err)

		assert.Equal(t, valueFelt, result.Value)
		assert.Equal(t, uint64(5), result.LastUpdateBlock)
		assert.True(t, result.IncludeLastUpdateBlock(nil))
	})

	t.Run("object with zero values", func(t *testing.T) {
		t.Parallel()
		var result rpc.StorageAtResult
		err := json.Unmarshal([]byte(`{"value":"0x0","last_update_block":0}`), &result)
		require.NoError(t, err)

		assert.Equal(t, felt.Zero, result.Value)
		assert.Equal(t, uint64(0), result.LastUpdateBlock)
		assert.True(t, result.IncludeLastUpdateBlock(nil))
	})

	t.Run("plain felt zero", func(t *testing.T) {
		t.Parallel()
		var result rpc.StorageAtResult
		err := json.Unmarshal([]byte(`"0x0"`), &result)
		require.NoError(t, err)

		assert.Equal(t, felt.Zero, result.Value)
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()
		var result rpc.StorageAtResult
		err := json.Unmarshal([]byte(`not_valid`), &result)
		require.Error(t, err)
	})
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
	expectedStorage := felt.NewFromUint64[felt.Felt](1)

	t.Run("no flags", func(t *testing.T) {
		noFlags := rpc.StorageResponseFlags{}

		t.Run("empty blockchain", func(t *testing.T) {
			mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

			blockID := blockIDLatest(t)
			storageValue, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, storageValue)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})

		t.Run("non-existent block hash", func(t *testing.T) {
			mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

			blockID := blockIDHash(t, &felt.Zero)
			storageValue, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, storageValue)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})

		t.Run("non-existent block number", func(t *testing.T) {
			mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

			blockID := blockIDNumber(t, 0)
			storageValue, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, storageValue)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})

		t.Run("non-existent contract", func(t *testing.T) {
			mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, db.ErrKeyNotFound)

			blockID := blockIDLatest(t)
			storageValue, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, storageValue)
			assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
		})

		t.Run("non-existent key", func(t *testing.T) {
			mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(felt.Zero, nil)

			blockID := blockIDLatest(t)
			storageValue, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			assert.Equal(t, &felt.Zero, storageValue)
			require.Nil(t, rpcErr)
		})

		t.Run("internal error while retrieving key", func(t *testing.T) {
			mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).
				Return(felt.Felt{}, errors.New("some internal error"))

			blockID := blockIDLatest(t)
			storageValue, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			assert.Nil(t, storageValue)
			assert.Equal(t, rpccore.ErrInternal, rpcErr)
		})

		t.Run("blockID - latest", func(t *testing.T) {
			mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

			blockID := blockIDLatest(t)
			storageValue, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, rpcErr)
			assert.Equal(t, expectedStorage, storageValue)
		})

		t.Run("blockID - hash", func(t *testing.T) {
			mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

			blockID := blockIDHash(t, &felt.Zero)
			storageValue, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, rpcErr)
			assert.Equal(t, expectedStorage, storageValue)
		})

		t.Run("blockID - number", func(t *testing.T) {
			mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
			mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
			mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

			blockID := blockIDNumber(t, 0)
			storageValue, rpcErr := handler.StorageAt(
				&targetAddress,
				&targetSlot,
				&blockID,
				noFlags,
			)
			require.Nil(t, rpcErr)
			assert.Equal(t, expectedStorage, storageValue)
		})

		t.Run("blockID - pre_confirmed", func(t *testing.T) {
			preConfirmedStateDiff := core.EmptyStateDiff()
			preConfirmedStateDiff.
				StorageDiffs[targetAddress] = map[felt.Felt]*felt.Felt{targetSlot: expectedStorage}
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
			preConfirmedID := blockIDPreConfirmed(t)
			storageValue, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &preConfirmedID, noFlags)
			require.Nil(t, rpcErr)
			assert.Equal(t, expectedStorage, storageValue)
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
			mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(*expectedStorage, nil)

			blockID := blockIDL1Accepted(t)
			storageValue, rpcErr := handler.StorageAt(&felt.Zero, &felt.Zero, &blockID, noFlags)
			require.Nil(t, rpcErr)
			assert.Equal(t, expectedStorage, storageValue)
		})
	})

	// t.Run("with IncludeLastUpdateBlock flag", func(t *testing.T) {
	// 	t.Run("finds last update block", func(t *testing.T) {
	// 		blockNumber := uint64(5)
	// 		lastUpdateBlockNum := uint64(3)

	// 		mockReader.EXPECT().StateAtBlockNumber(blockNumber).Return(mockState, nopCloser, nil)
	// 		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
	// 		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

	// 		emptyStateDiff := core.EmptyStateDiff()
	// 		mockReader.EXPECT().StateUpdateByNumber(blockNumber).Return(
	// 			&core.StateUpdate{StateDiff: &emptyStateDiff}, nil,
	// 		)
	// 		mockReader.EXPECT().StateUpdateByNumber(uint64(4)).Return(
	// 			&core.StateUpdate{StateDiff: &emptyStateDiff}, nil,
	// 		)

	// 		updateStateDiff := core.EmptyStateDiff()
	// 		updateStateDiff.StorageDiffs[targetAddress] = map[felt.Felt]*felt.Felt{
	// 			targetSlot: expectedStorage,
	// 		}
	// 		mockReader.EXPECT().StateUpdateByNumber(lastUpdateBlockNum).Return(
	// 			&core.StateUpdate{StateDiff: &updateStateDiff}, nil,
	// 		)

	// 		blockID := blockIDNumber(t, blockNumber)
	// 		result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &blockID, flags)
	// 		require.Nil(t, rpcErr)

	// 		storageResult, ok := result.(*rpc.StorageResult)
	// 		require.True(t, ok)
	// 		assert.Equal(t, expectedStorage, storageResult.Value)
	// 		assert.Equal(t, lastUpdateBlockNum, storageResult.LastUpdateBlock)
	// 	})

	// 	t.Run("last update at genesis block", func(t *testing.T) {
	// 		blockNumber := uint64(2)

	// 		mockReader.EXPECT().StateAtBlockNumber(blockNumber).Return(mockState, nopCloser, nil)
	// 		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
	// 		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

	// 		emptyStateDiff := core.EmptyStateDiff()
	// 		mockReader.EXPECT().StateUpdateByNumber(blockNumber).Return(
	// 			&core.StateUpdate{StateDiff: &emptyStateDiff}, nil,
	// 		)
	// 		mockReader.EXPECT().StateUpdateByNumber(uint64(1)).Return(
	// 			&core.StateUpdate{StateDiff: &emptyStateDiff}, nil,
	// 		)

	// 		genesisStateDiff := core.EmptyStateDiff()
	// 		genesisStateDiff.StorageDiffs[targetAddress] = map[felt.Felt]*felt.Felt{
	// 			targetSlot: expectedStorage,
	// 		}
	// 		mockReader.EXPECT().StateUpdateByNumber(uint64(0)).Return(
	// 			&core.StateUpdate{StateDiff: &genesisStateDiff}, nil,
	// 		)

	// 		blockID := blockIDNumber(t, blockNumber)
	// 		result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &blockID, flags)
	// 		require.Nil(t, rpcErr)

	// 		storageResult, ok := result.(*rpc.StorageResult)
	// 		require.True(t, ok)
	// 		assert.Equal(t, expectedStorage, storageResult.Value)
	// 		assert.Equal(t, uint64(0), storageResult.LastUpdateBlock)
	// 	})

	// 	t.Run("without flag returns plain felt", func(t *testing.T) {
	// 		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
	// 		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
	// 		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

	// 		blockID := blockIDLatest(t)
	// 		noFlags := rpc.StorageResponseFlags{}
	// 		result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &blockID, noFlags)
	// 		require.Nil(t, rpcErr)

	// 		feltResult, ok := result.(*felt.Felt)
	// 		require.True(t, ok)
	// 		assert.Equal(t, expectedStorage, feltResult)
	// 	})

	// 	t.Run("with flag and l1_accepted block", func(t *testing.T) {
	// 		l1HeadBlockNumber := uint64(10)
	// 		lastUpdateBlockNum := uint64(8)

	// 		mockReader.EXPECT().L1Head().Return(
	// 			core.L1Head{
	// 				BlockNumber: l1HeadBlockNumber,
	// 				BlockHash:   &felt.Zero,
	// 				StateRoot:   &felt.Zero,
	// 			},
	// 			nil,
	// 		).Times(2)
	// 		mockReader.EXPECT().StateAtBlockNumber(l1HeadBlockNumber).Return(mockState, nopCloser, nil)
	// 		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
	// 		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)

	// 		emptyStateDiff := core.EmptyStateDiff()
	// 		mockReader.EXPECT().StateUpdateByNumber(l1HeadBlockNumber).Return(
	// 			&core.StateUpdate{StateDiff: &emptyStateDiff}, nil,
	// 		)
	// 		mockReader.EXPECT().StateUpdateByNumber(uint64(9)).Return(
	// 			&core.StateUpdate{StateDiff: &emptyStateDiff}, nil,
	// 		)

	// 		updateStateDiff := core.EmptyStateDiff()
	// 		updateStateDiff.StorageDiffs[targetAddress] = map[felt.Felt]*felt.Felt{
	// 			targetSlot: expectedStorage,
	// 		}
	// 		mockReader.EXPECT().StateUpdateByNumber(lastUpdateBlockNum).Return(
	// 			&core.StateUpdate{StateDiff: &updateStateDiff}, nil,
	// 		)

	// 		blockID := blockIDL1Accepted(t)
	// 		result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &blockID, flags)
	// 		require.Nil(t, rpcErr)

	// 		storageResult, ok := result.(*rpc.StorageResult)
	// 		require.True(t, ok)
	// 		assert.Equal(t, expectedStorage, storageResult.Value)
	// 		assert.Equal(t, lastUpdateBlockNum, storageResult.LastUpdateBlock)
	// 	})

	// 	t.Run("state update error returns internal error", func(t *testing.T) {
	// 		blockNumber := uint64(3)

	// 		mockReader.EXPECT().StateAtBlockNumber(blockNumber).Return(mockState, nopCloser, nil)
	// 		mockState.EXPECT().ContractClassHash(&targetAddress).Return(felt.Felt{}, nil)
	// 		mockState.EXPECT().ContractStorage(&targetAddress, &targetSlot).Return(*expectedStorage, nil)
	// 		mockReader.EXPECT().StateUpdateByNumber(blockNumber).Return(
	// 			nil, errors.New("db error"),
	// 		)

	// 		blockID := blockIDNumber(t, blockNumber)
	// 		result, rpcErr := handler.StorageAt(&targetAddress, &targetSlot, &blockID, flags)
	// 		assert.Nil(t, result)
	// 		assert.Equal(t, rpccore.ErrInternal, rpcErr)
	// 	})
	// })
}
