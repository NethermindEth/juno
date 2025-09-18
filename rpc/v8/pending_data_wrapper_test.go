package rpcv8_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/types/felt"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPendingDataWrapper_PendingData(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	n := utils.HeapPtr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, log)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)
	require.NoError(t, err)

	t.Run("Returns pending block data when starknet version < 0.14.0", func(t *testing.T) {
		expectedPending := core.NewPending(latestBlock, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&expectedPending,
			nil,
		)
		pending, err := handler.PendingData()
		require.NoError(t, err)
		require.Equal(t, expectedPending, core.Pending{
			Block:       pending.GetBlock(),
			StateUpdate: pending.GetStateUpdate(),
			NewClasses:  pending.GetNewClasses(),
		})
	})
	t.Run("Returns empty pending block when starknet version >= 0.14.0", func(t *testing.T) {
		preConfirmed := core.NewPreConfirmed(nil, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		)
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)

		receipts := make([]*core.TransactionReceipt, 0)
		expectedPendingB := &core.Block{
			Header: &core.Header{
				ParentHash:       latestBlock.Hash,
				Number:           latestBlockNumber + 1,
				SequencerAddress: latestBlock.SequencerAddress,
				Timestamp:        uint64(time.Now().Unix()),
				ProtocolVersion:  latestBlock.ProtocolVersion,
				EventsBloom:      core.EventsBloom(receipts),
				L1GasPriceETH:    latestBlock.L1GasPriceETH,
				L1GasPriceSTRK:   latestBlock.L1GasPriceSTRK,
				L2GasPrice:       latestBlock.L2GasPrice,
				L1DataGasPrice:   latestBlock.L1DataGasPrice,
				L1DAMode:         latestBlock.L1DAMode,
			},
			Transactions: make([]core.Transaction, 0),
			Receipts:     receipts,
		}

		emptyStateDiff := &core.StateDiff{
			StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt),
			Nonces:            make(map[felt.Felt]*felt.Felt),
			DeployedContracts: make(map[felt.Felt]*felt.Felt),
			DeclaredV0Classes: make([]*felt.Felt, 0),
			DeclaredV1Classes: make(map[felt.Felt]*felt.Felt),
			ReplacedClasses:   make(map[felt.Felt]*felt.Felt),
		}

		expectedPending := &core.Pending{
			Block: expectedPendingB,
			StateUpdate: &core.StateUpdate{
				OldRoot:   latestBlock.GlobalStateRoot,
				StateDiff: emptyStateDiff,
			},
			NewClasses: make(map[felt.Felt]core.Class, 0),
		}

		pending, err := handler.PendingData()
		require.NoError(t, err)
		require.Equal(t, *expectedPending, core.NewPending(
			pending.GetBlock(),
			pending.GetStateUpdate(),
			pending.GetNewClasses(),
		))
	})
}

func TestPendingDataWrapper_PendingState(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, mockSyncReader, nil, nil)

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	t.Run("Returns pending state when starknet version < 0.14.0", func(t *testing.T) {
		mockSyncReader.EXPECT().PendingData().Return(
			&core.Pending{},
			nil,
		)
		mockSyncReader.EXPECT().PendingState().Return(mockState, nopCloser, nil)
		pendingState, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pendingState)
		require.NotNil(t, closer)
	})

	t.Run("Returns latest state starknet version >= 0.14.0", func(t *testing.T) {
		mockSyncReader.EXPECT().PendingData().Return(
			&core.PreConfirmed{},
			nil,
		)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		pending, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pending)
		require.NotNil(t, closer)
	})

	t.Run("Returns latest state when pending data is nil", func(t *testing.T) {
		mockSyncReader.EXPECT().PendingData().Return(
			nil,
			sync.ErrPendingBlockNotFound,
		)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		pending, closer, err := handler.PendingState()

		require.NoError(t, err)
		require.NotNil(t, pending)
		require.NotNil(t, closer)
	})
}
