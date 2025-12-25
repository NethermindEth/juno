package rpcv6_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v6"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync/pendingdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStateUpdate(t *testing.T) {
	errTests := map[string]rpc.BlockID{
		"latest":  {Latest: true},
		"pending": {Pending: true},
		"hash":    {Hash: new(felt.Felt).SetUint64(1)},
		"number":  {Number: 1},
	}
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	var mockSyncReader *mocks.MockSyncReader

	n := &utils.Mainnet
	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			chain := blockchain.New(memory.New(), n)

			if description == "pending" /*nolint:goconst*/ {
				mockSyncReader = mocks.NewMockSyncReader(mockCtrl)
				mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
			}
			handler := rpc.New(chain, mockSyncReader, nil, n, nil)

			update, rpcErr := handler.StateUpdate(id)
			assert.Nil(t, update)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, mockSyncReader, nil, n, nil)
	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	update21656, err := mainnetGw.StateUpdate(t.Context(), 21656)
	require.NoError(t, err)

	checkUpdate := func(t *testing.T, coreUpdate *core.StateUpdate, rpcUpdate *rpc.StateUpdate) {
		t.Helper()
		require.Equal(t, coreUpdate.BlockHash, rpcUpdate.BlockHash)
		require.Equal(t, coreUpdate.NewRoot, rpcUpdate.NewRoot)
		require.Equal(t, coreUpdate.OldRoot, rpcUpdate.OldRoot)

		require.Equal(t, len(coreUpdate.StateDiff.StorageDiffs), len(rpcUpdate.StateDiff.StorageDiffs))
		for _, diff := range rpcUpdate.StateDiff.StorageDiffs {
			coreDiffs := coreUpdate.StateDiff.StorageDiffs[diff.Address]
			require.Equal(t, len(coreDiffs), len(diff.StorageEntries))
			for _, entry := range diff.StorageEntries {
				require.Equal(t, *coreDiffs[entry.Key], entry.Value)
			}
		}

		require.Equal(t, len(coreUpdate.StateDiff.Nonces), len(rpcUpdate.StateDiff.Nonces))
		for _, nonce := range rpcUpdate.StateDiff.Nonces {
			require.Equal(t, *coreUpdate.StateDiff.Nonces[nonce.ContractAddress], nonce.Nonce)
		}

		require.Equal(t, len(coreUpdate.StateDiff.DeployedContracts), len(rpcUpdate.StateDiff.DeployedContracts))
		for _, deployedContract := range rpcUpdate.StateDiff.DeployedContracts {
			require.Equal(t, *coreUpdate.StateDiff.DeployedContracts[deployedContract.Address], deployedContract.ClassHash)
		}

		require.Equal(t, coreUpdate.StateDiff.DeclaredV0Classes, rpcUpdate.StateDiff.DeprecatedDeclaredClasses)

		require.Equal(t, len(coreUpdate.StateDiff.ReplacedClasses), len(rpcUpdate.StateDiff.ReplacedClasses))
		for index := range rpcUpdate.StateDiff.ReplacedClasses {
			require.Equal(t, *coreUpdate.StateDiff.ReplacedClasses[rpcUpdate.StateDiff.ReplacedClasses[index].ContractAddress],
				rpcUpdate.StateDiff.ReplacedClasses[index].ClassHash)
		}

		require.Equal(t, len(coreUpdate.StateDiff.DeclaredV1Classes), len(rpcUpdate.StateDiff.DeclaredClasses))
		for index := range rpcUpdate.StateDiff.DeclaredClasses {
			require.Equal(t, *coreUpdate.StateDiff.DeclaredV1Classes[rpcUpdate.StateDiff.DeclaredClasses[index].ClassHash],
				rpcUpdate.StateDiff.DeclaredClasses[index].CompiledClassHash)
		}
	}

	t.Run("latest", func(t *testing.T) {
		mockReader.EXPECT().Height().Return(uint64(21656), nil)
		mockReader.EXPECT().StateUpdateByNumber(uint64(21656)).Return(update21656, nil)

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})

	t.Run("by height", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByNumber(uint64(21656)).Return(update21656, nil)

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Number: uint64(21656)})
		require.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})

	t.Run("by hash", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByHash(update21656.BlockHash).Return(update21656, nil)

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Hash: update21656.BlockHash})
		require.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})

	t.Run("post v0.11.0", func(t *testing.T) {
		integrationClient := feeder.NewTestClient(t, &utils.Integration)
		integGw := adaptfeeder.New(integrationClient)

		for name, height := range map[string]uint64{
			"declared Cairo0 classes": 283746,
			"declared Cairo1 classes": 283364,
			"replaced classes":        283428,
		} {
			t.Run(name, func(t *testing.T) {
				gwUpdate, err := integGw.StateUpdate(t.Context(), height)
				require.NoError(t, err)

				mockReader.EXPECT().StateUpdateByNumber(height).Return(gwUpdate, nil)

				update, rpcErr := handler.StateUpdate(rpc.BlockID{Number: height})
				require.Nil(t, rpcErr)

				checkUpdate(t, gwUpdate, update)
			})
		}
	})

	t.Run("pending starknet version < 0.14.0", func(t *testing.T) {
		update21656.BlockHash = nil
		update21656.NewRoot = nil
		pending := core.NewPending(nil, update21656, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&pending,
			nil,
		)

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Pending: true})
		require.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})

	t.Run("pending starknet version >= 0.14.0", func(t *testing.T) {
		update21656, err := mainnetGw.StateUpdate(t.Context(), 21656)
		require.NoError(t, err)

		preConfirmed := core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: 21657,
				},
			},
		}
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		)

		blockToRegisterHash := core.Header{
			Hash:   felt.NewUnsafeFromString[felt.Felt]("0xFFFF"),
			Number: preConfirmed.Block.Number - 10,
		}
		// to update block hash registry
		mockReader.EXPECT().BlockHeaderByNumber(blockToRegisterHash.Number).
			Return(&blockToRegisterHash, nil)
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{
			GlobalStateRoot: update21656.NewRoot,
			Number:          21656,
		}, nil)

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Pending: true})
		require.Nil(t, rpcErr)
		expectedStateDiff := core.EmptyStateDiff()
		expectedStateDiff.StorageDiffs[*pendingdata.BlockHashStorageContract] = map[felt.Felt]*felt.Felt{
			felt.FromUint64[felt.Felt](blockToRegisterHash.Number): blockToRegisterHash.Hash,
		}
		checkUpdate(t, &core.StateUpdate{
			OldRoot:   update21656.NewRoot,
			StateDiff: &expectedStateDiff,
		}, update)
	})
}
