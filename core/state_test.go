package core_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdate(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)

	gw := adaptfeeder.New(client)

	testDB := pebble.NewMemTest()
	txn := testDB.NewTransaction(true)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	state := core.NewState(txn)

	su0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	su1, err := gw.StateUpdate(context.Background(), 1)
	require.NoError(t, err)

	su2, err := gw.StateUpdate(context.Background(), 2)
	require.NoError(t, err)

	t.Run("empty state updated with mainnet block 0 state update", func(t *testing.T) {
		require.NoError(t, state.Update(0, su0, nil))
		gotNewRoot, err := state.Root()
		require.NoError(t, err)
		assert.Equal(t, su0.NewRoot, gotNewRoot)
	})

	t.Run("error when state current root doesn't match state update's old root", func(t *testing.T) {
		oldRoot := new(felt.Felt).SetBytes([]byte("some old root"))
		su := &core.StateUpdate{
			OldRoot: oldRoot,
		}
		expectedErr := fmt.Sprintf("state's current root: %s does not match state update's old root: %s", su0.NewRoot, oldRoot)
		require.EqualError(t, state.Update(1, su, nil), expectedErr)
	})

	t.Run("error when state new root doesn't match state update's new root", func(t *testing.T) {
		newRoot := new(felt.Felt).SetBytes([]byte("some new root"))
		su := &core.StateUpdate{
			NewRoot:   newRoot,
			OldRoot:   su0.NewRoot,
			StateDiff: new(core.StateDiff),
		}
		expectedErr := fmt.Sprintf("state's new root: %s does not match state update's new root: %s", su0.NewRoot, newRoot)
		require.EqualError(t, state.Update(1, su, nil), expectedErr)
	})

	t.Run("non-empty state updated multiple times", func(t *testing.T) {
		require.NoError(t, state.Update(1, su1, nil))
		gotNewRoot, err := state.Root()
		require.NoError(t, err)
		assert.Equal(t, su1.NewRoot, gotNewRoot)

		require.NoError(t, state.Update(2, su2, nil))
		gotNewRoot, err = state.Root()
		require.NoError(t, err)
		assert.Equal(t, su2.NewRoot, gotNewRoot)
	})
}

func TestContractClassHash(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)

	gw := adaptfeeder.New(client)

	testDB := pebble.NewMemTest()
	txn := testDB.NewTransaction(true)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	state := core.NewState(txn)

	su0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	su1, err := gw.StateUpdate(context.Background(), 1)
	require.NoError(t, err)

	require.NoError(t, state.Update(0, su0, nil))
	require.NoError(t, state.Update(1, su1, nil))

	allDeployedContracts := make(map[felt.Felt]*felt.Felt)

	for _, dc := range su0.StateDiff.DeployedContracts {
		allDeployedContracts[*dc.Address] = dc.ClassHash
	}

	for _, dc := range su1.StateDiff.DeployedContracts {
		allDeployedContracts[*dc.Address] = dc.ClassHash
	}

	for addr, expectedClassHash := range allDeployedContracts {
		gotClassHash, err := state.ContractClassHash(&addr)
		require.NoError(t, err)

		assert.Equal(t, expectedClassHash, gotClassHash)
	}
}

func TestNonce(t *testing.T) {
	testDB := pebble.NewMemTest()
	txn := testDB.NewTransaction(true)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	state := core.NewState(txn)

	addr := utils.HexToFelt(t, "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6")
	root := utils.HexToFelt(t, "0x4bdef7bf8b81a868aeab4b48ef952415fe105ab479e2f7bc671c92173542368")

	su := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: root,
		StateDiff: &core.StateDiff{
			DeployedContracts: []core.DeployedContract{
				{
					Address:   addr,
					ClassHash: utils.HexToFelt(t, "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"),
				},
			},
		},
	}

	require.NoError(t, state.Update(0, su, nil))

	t.Run("newly deployed contract has zero nonce", func(t *testing.T) {
		nonce, err := state.ContractNonce(addr)
		require.NoError(t, err)
		assert.Equal(t, &felt.Zero, nonce)
	})

	t.Run("update contract nonce", func(t *testing.T) {
		expectedNonce := new(felt.Felt).SetUint64(1)
		su = &core.StateUpdate{
			NewRoot: utils.HexToFelt(t, "0x6210642ffd49f64617fc9e5c0bbe53a6a92769e2996eb312a42d2bdb7f2afc1"),
			OldRoot: root,
			StateDiff: &core.StateDiff{
				Nonces: map[felt.Felt]*felt.Felt{*addr: expectedNonce},
			},
		}

		require.NoError(t, state.Update(1, su, nil))

		gotNonce, err := state.ContractNonce(addr)
		require.NoError(t, err)
		assert.Equal(t, expectedNonce, gotNonce)
	})
}

func TestHistory(t *testing.T) {
	testDB := pebble.NewMemTest()
	txn := testDB.NewTransaction(true)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)

	gw := adaptfeeder.New(client)

	state := core.NewState(txn)
	su0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)
	require.NoError(t, state.Update(0, su0, nil))

	contractAddr := utils.HexToFelt(t, "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6")
	changedLoc := utils.HexToFelt(t, "0x5")
	t.Run("should return an error for a location that changed on the given height", func(t *testing.T) {
		_, err = state.ContractStorageAt(contractAddr, changedLoc, 0)
		require.EqualError(t, err, "check head state")
	})

	t.Run("should return an error for not changed location", func(t *testing.T) {
		_, err := state.ContractStorageAt(contractAddr, utils.HexToFelt(t, "0xDEADBEEF"), 0)
		require.EqualError(t, err, "check head state")
	})

	// update the same location again
	su := &core.StateUpdate{
		NewRoot: utils.HexToFelt(t, "0xac747e0ea7497dad7407ecf2baf24b1598b0b40943207fc9af8ded09a64f1c"),
		OldRoot: su0.NewRoot,
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt][]core.StorageDiff{
				*contractAddr: {
					{
						Key:   changedLoc,
						Value: utils.HexToFelt(t, "0x44"),
					},
				},
			},
		},
	}
	require.NoError(t, state.Update(1, su, nil))

	t.Run("should give old value for a location that changed after the given height", func(t *testing.T) {
		oldValue, err := state.ContractStorageAt(contractAddr, changedLoc, 0)
		require.NoError(t, err)
		require.Equal(t, oldValue, utils.HexToFelt(t, "0x22b"))
	})
}
