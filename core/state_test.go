package core_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/encoder"
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
		expectedErr := fmt.Sprintf("state's current root: %s does not match the expected root: %s", su0.NewRoot, oldRoot)
		require.EqualError(t, state.Update(1, su, nil), expectedErr)
	})

	t.Run("error when state new root doesn't match state update's new root", func(t *testing.T) {
		newRoot := new(felt.Felt).SetBytes([]byte("some new root"))
		su := &core.StateUpdate{
			NewRoot:   newRoot,
			OldRoot:   su0.NewRoot,
			StateDiff: new(core.StateDiff),
		}
		expectedErr := fmt.Sprintf("state's current root: %s does not match the expected root: %s", su0.NewRoot, newRoot)
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

	t.Run("post v0.11.0 declared classes affect root", func(t *testing.T) {
		su := &core.StateUpdate{
			OldRoot: su2.NewRoot,
			NewRoot: utils.HexToFelt(t, "0x46f1033cfb8e0b2e16e1ad6f95c41fd3a123f168fe72665452b6cddbc1d8e7a"),
			StateDiff: &core.StateDiff{
				DeclaredV1Classes: []core.DeclaredV1Class{
					{
						ClassHash:         utils.HexToFelt(t, "0xDEADBEEF"),
						CompiledClassHash: utils.HexToFelt(t, "0xBEEFDEAD"),
					},
				},
			},
		}

		require.NoError(t, state.Update(3, su, nil))
		assert.NotEqual(t, su.NewRoot, su.OldRoot)
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

	t.Run("replace class hash", func(t *testing.T) {
		replaceUpdate := &core.StateUpdate{
			OldRoot:   su1.NewRoot,
			BlockHash: utils.HexToFelt(t, "0xDEADBEEF"),
			NewRoot:   utils.HexToFelt(t, "0x484ff378143158f9af55a1210b380853ae155dfdd8cd4c228f9ece918bb982b"),
			StateDiff: &core.StateDiff{
				ReplacedClasses: []core.ReplacedClass{
					{
						Address:   su1.StateDiff.DeployedContracts[0].Address,
						ClassHash: utils.HexToFelt(t, "0x1337"),
					},
				},
			},
		}

		require.NoError(t, state.Update(2, replaceUpdate, nil))

		gotClassHash, err := state.ContractClassHash(su1.StateDiff.DeployedContracts[0].Address)
		require.NoError(t, err)

		assert.Equal(t, utils.HexToFelt(t, "0x1337"), gotClassHash)
	})
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

func TestStateHistory(t *testing.T) {
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
		assert.ErrorIs(t, err, core.ErrCheckHeadState)
	})

	t.Run("should return an error for not changed location", func(t *testing.T) {
		_, err := state.ContractStorageAt(contractAddr, utils.HexToFelt(t, "0xDEADBEEF"), 0)
		assert.ErrorIs(t, err, core.ErrCheckHeadState)
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

func TestContractIsDeployedAt(t *testing.T) {
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

	t.Run("deployed on genesis", func(t *testing.T) {
		deployedOn0 := utils.HexToFelt(t, "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6")
		deployed, err := state.ContractIsAlreadyDeployedAt(deployedOn0, 0)
		require.NoError(t, err)
		assert.True(t, deployed)

		deployed, err = state.ContractIsAlreadyDeployedAt(deployedOn0, 1)
		require.NoError(t, err)
		assert.True(t, deployed)
	})

	t.Run("deployed after genesis", func(t *testing.T) {
		deployedOn1 := utils.HexToFelt(t, "0x6538fdd3aa353af8a87f5fe77d1f533ea82815076e30a86d65b72d3eb4f0b80")
		deployed, err := state.ContractIsAlreadyDeployedAt(deployedOn1, 0)
		require.NoError(t, err)
		assert.False(t, deployed)

		deployed, err = state.ContractIsAlreadyDeployedAt(deployedOn1, 1)
		require.NoError(t, err)
		assert.True(t, deployed)
	})

	t.Run("not deployed", func(t *testing.T) {
		notDeployed := utils.HexToFelt(t, "0xDEADBEEF")
		deployed, err := state.ContractIsAlreadyDeployedAt(notDeployed, 1)
		require.NoError(t, err)
		assert.False(t, deployed)
	})
}

func TestClass(t *testing.T) {
	testDB := pebble.NewMemTest()
	txn := testDB.NewTransaction(true)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	client, closeFn := feeder.NewTestClient(utils.INTEGRATION)
	t.Cleanup(closeFn)
	gw := adaptfeeder.New(client)

	cairo0Hash := utils.HexToFelt(t, "0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04")
	cairo0Class, err := gw.Class(context.Background(), cairo0Hash)
	require.NoError(t, err)
	cairo1Hash := utils.HexToFelt(t, "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5")
	cairo1Class, err := gw.Class(context.Background(), cairo0Hash)
	require.NoError(t, err)

	err = encoder.RegisterType(reflect.TypeOf(cairo0Class))
	if err != nil {
		require.Contains(t, err.Error(), "already exists in TagSet")
	}
	err = encoder.RegisterType(reflect.TypeOf(cairo1Hash))
	if err != nil {
		require.Contains(t, err.Error(), "already exists in TagSet")
	}

	state := core.NewState(txn)
	su0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)
	require.NoError(t, state.Update(0, su0, map[felt.Felt]core.Class{
		*cairo0Hash: cairo0Class,
		*cairo1Hash: cairo1Class,
	}))

	gotCairo1Class, err := state.Class(cairo1Hash)
	require.NoError(t, err)
	assert.Zero(t, gotCairo1Class.At)
	assert.Equal(t, cairo1Class, gotCairo1Class.Class)
	gotCairo0Class, err := state.Class(cairo0Hash)
	require.NoError(t, err)
	assert.Zero(t, gotCairo0Class.At)
	assert.Equal(t, cairo0Class, gotCairo0Class.Class)
}

func TestRevert(t *testing.T) {
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
	su1, err := gw.StateUpdate(context.Background(), 1)
	require.NoError(t, err)
	require.NoError(t, state.Update(1, su1, nil))

	t.Run("revert a replaced class", func(t *testing.T) {
		addr := su1.StateDiff.DeployedContracts[0].Address
		replaceStateUpdate := &core.StateUpdate{
			NewRoot: utils.HexToFelt(t, "0x30b1741b28893b892ac30350e6372eac3a6f32edee12f9cdca7fbe7540a5ee"),
			OldRoot: su1.NewRoot,
			StateDiff: &core.StateDiff{
				ReplacedClasses: []core.ReplacedClass{
					{
						Address:   addr,
						ClassHash: utils.HexToFelt(t, "0xDEADBEEF"),
					},
				},
			},
		}

		require.NoError(t, state.Update(2, replaceStateUpdate, nil))
		require.NoError(t, state.Revert(2, replaceStateUpdate))
		classHash, sErr := state.ContractClassHash(addr)
		require.NoError(t, sErr)
		assert.Equal(t, su1.StateDiff.DeployedContracts[0].ClassHash, classHash)
	})

	t.Run("revert a nonce update", func(t *testing.T) {
		addr := su1.StateDiff.DeployedContracts[0].Address
		nonceStateUpdate := &core.StateUpdate{
			NewRoot: utils.HexToFelt(t, "0x6683657d2b6797d95f318e7c6091dc2255de86b72023c15b620af12543eb62c"),
			OldRoot: su1.NewRoot,
			StateDiff: &core.StateDiff{
				Nonces: map[felt.Felt]*felt.Felt{
					*addr: utils.HexToFelt(t, "0xDEADBEEF"),
				},
			},
		}

		require.NoError(t, state.Update(2, nonceStateUpdate, nil))
		require.NoError(t, state.Revert(2, nonceStateUpdate))
		nonce, sErr := state.ContractNonce(addr)
		require.NoError(t, sErr)
		assert.Equal(t, &felt.Zero, nonce)
	})

	su2, err := gw.StateUpdate(context.Background(), 2)
	require.NoError(t, err)
	t.Run("should be able to apply new update after a Revert", func(t *testing.T) {
		require.NoError(t, state.Update(2, su2, nil))
	})

	t.Run("should be able to revert all the state", func(t *testing.T) {
		require.NoError(t, state.Revert(2, su2))
		root, err := state.Root()
		require.NoError(t, err)
		require.Equal(t, su2.OldRoot, root)
		require.NoError(t, state.Revert(1, su1))
		root, err = state.Root()
		require.NoError(t, err)
		require.Equal(t, su1.OldRoot, root)
		require.NoError(t, state.Revert(0, su0))
		root, err = state.Root()
		require.NoError(t, err)
		require.Equal(t, su0.OldRoot, root)
	})

	t.Run("empty state should mean empty db", func(t *testing.T) {
		require.NoError(t, testDB.View(func(txn db.Transaction) error {
			it, err := txn.NewIterator()
			if err != nil {
				return err
			}
			assert.False(t, it.Next())
			return nil
		}))
	})
}
