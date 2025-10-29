package rpcv10_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStateUpdate_ErrorCases(t *testing.T) {
	errTests := map[string]rpcv9.BlockID{
		"latest":        rpcv9.BlockIDLatest(),
		"pre_confirmed": rpcv9.BlockIDPreConfirmed(),
		"hash":          rpcv9.BlockIDFromHash(&felt.One),
		"number":        rpcv9.BlockIDFromNumber(1),
		"l1_accepted":   rpcv9.BlockIDL1Accepted(),
	}
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	var mockSyncReader *mocks.MockSyncReader

	n := &utils.Mainnet
	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			chain := blockchain.New(memory.New(), n)
			if description == "pre_confirmed" {
				mockSyncReader = mocks.NewMockSyncReader(mockCtrl)
				mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
			}
			log := utils.NewNopZapLogger()
			handler := rpcv10.New(chain, mockSyncReader, nil, log)

			update, rpcErr := handler.StateUpdate(&id)
			assert.Empty(t, update)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}
}

func TestStateUpdate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := &utils.Mainnet

	log := utils.NewNopZapLogger()
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	handler := rpcv10.New(mockReader, mockSyncReader, nil, log)
	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	// TODO(Ege): Update test data with 0.14.1 state update
	update21656, err := mainnetGw.StateUpdate(t.Context(), 21656)
	require.NoError(t, err)

	t.Run("latest", func(t *testing.T) {
		mockReader.EXPECT().Height().Return(uint64(21656), nil)
		mockReader.EXPECT().StateUpdateByNumber(uint64(21656)).Return(update21656, nil)
		latest := rpcv9.BlockIDLatest()
		update, rpcErr := handler.StateUpdate(&latest)
		require.Nil(t, rpcErr)
		assertStateUpdateEq(t, update21656, &update)
	})

	t.Run("by height", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByNumber(uint64(21656)).Return(update21656, nil)
		number := rpcv9.BlockIDFromNumber(21656)
		update, rpcErr := handler.StateUpdate(&number)
		require.Nil(t, rpcErr)
		assertStateUpdateEq(t, update21656, &update)
	})

	t.Run("by hash", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByHash(update21656.BlockHash).Return(update21656, nil)
		hash := rpcv9.BlockIDFromHash(update21656.BlockHash)
		update, rpcErr := handler.StateUpdate(&hash)
		require.Nil(t, rpcErr)
		assertStateUpdateEq(t, update21656, &update)
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
				number := rpcv9.BlockIDFromNumber(height)
				mockReader.EXPECT().StateUpdateByNumber(height).Return(gwUpdate, nil)
				update, rpcErr := handler.StateUpdate(&number)
				require.Nil(t, rpcErr)

				assertStateUpdateEq(t, gwUpdate, &update)
			})
		}
	})

	t.Run("l1_accepted", func(t *testing.T) {
		mockReader.EXPECT().L1Head().Return(
			core.L1Head{
				BlockNumber: uint64(21656),
				BlockHash:   update21656.BlockHash,
				StateRoot:   update21656.NewRoot,
			},
			nil,
		)
		mockReader.EXPECT().StateUpdateByNumber(uint64(21656)).Return(update21656, nil)
		l1AcceptedID := rpcv9.BlockIDL1Accepted()
		update, rpcErr := handler.StateUpdate(&l1AcceptedID)
		require.Nil(t, rpcErr)
		assertStateUpdateEq(t, update21656, &update)
	})

	t.Run("pre_confirmed", func(t *testing.T) {
		update21656.BlockHash = nil
		update21656.NewRoot = nil
		preConfirmed := core.NewPreConfirmed(nil, update21656, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		)
		preConfirmedID := rpcv9.BlockIDPreConfirmed()
		update, rpcErr := handler.StateUpdate(&preConfirmedID)
		require.Nil(t, rpcErr)
		assertStateUpdateEq(t, update21656, &update)
	})
}

func assertStateUpdateEq(
	t *testing.T,
	coreUpdate *core.StateUpdate,
	rpcUpdate *rpcv10.StateUpdate,
) {
	t.Helper()
	require.Equal(t, coreUpdate.BlockHash, rpcUpdate.BlockHash)
	require.Equal(t, coreUpdate.NewRoot, rpcUpdate.NewRoot)
	require.Equal(t, coreUpdate.OldRoot, rpcUpdate.OldRoot)
	assertStateDiffEq(t, coreUpdate, rpcUpdate)
}

func assertStateDiffEq(
	t *testing.T,
	coreUpdate *core.StateUpdate,
	rpcUpdate *rpcv10.StateUpdate,
) {
	require.Equal(
		t,
		len(coreUpdate.StateDiff.StorageDiffs),
		len(rpcUpdate.StateDiff.StorageDiffs),
	)

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

	require.Equal(
		t,
		len(coreUpdate.StateDiff.DeployedContracts),
		len(rpcUpdate.StateDiff.DeployedContracts),
	)
	for _, deployedContract := range rpcUpdate.StateDiff.DeployedContracts {
		require.Equal(
			t,
			*coreUpdate.StateDiff.DeployedContracts[deployedContract.Address],
			deployedContract.ClassHash,
		)
	}

	require.Equal(
		t,
		coreUpdate.StateDiff.DeclaredV0Classes,
		rpcUpdate.StateDiff.DeprecatedDeclaredClasses,
	)

	require.Equal(
		t,
		len(coreUpdate.StateDiff.ReplacedClasses),
		len(rpcUpdate.StateDiff.ReplacedClasses),
	)
	for index := range rpcUpdate.StateDiff.ReplacedClasses {
		replacedClass := &rpcUpdate.StateDiff.ReplacedClasses[index]
		require.Equal(
			t,
			*coreUpdate.StateDiff.ReplacedClasses[replacedClass.ContractAddress],
			replacedClass.ClassHash,
		)
	}

	require.Equal(
		t,
		len(coreUpdate.StateDiff.DeclaredV1Classes),
		len(rpcUpdate.StateDiff.DeclaredClasses),
	)
	for index := range rpcUpdate.StateDiff.DeclaredClasses {
		declaredClass := &rpcUpdate.StateDiff.DeclaredClasses[index]
		require.Equal(
			t,
			*coreUpdate.StateDiff.DeclaredV1Classes[declaredClass.ClassHash],
			declaredClass.CompiledClassHash,
		)
	}

	require.Equal(
		t,
		len(coreUpdate.StateDiff.MigratedClasses),
		len(rpcUpdate.StateDiff.MigratedCompiledClasses),
	)
	for index := range rpcUpdate.StateDiff.MigratedCompiledClasses {
		migratedCompiledClass := &rpcUpdate.StateDiff.MigratedCompiledClasses[index]
		require.Equal(
			t,
			coreUpdate.StateDiff.MigratedClasses[migratedCompiledClass.ClassHash],
			migratedCompiledClass.CompiledClassHash,
		)
	}
}
