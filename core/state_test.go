package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

// Address of first deployed contract in mainnet block 1's state update.
var (
	_su1FirstDeployedAddress, _ = new(felt.Felt).SetString("0x6538fdd3aa353af8a87f5fe77d1f533ea82815076e30a86d65b72d3eb4f0b80")
	su1FirstDeployedAddress     = *_su1FirstDeployedAddress
)

func TestMain(m *testing.M) {
	txTypes := []core.Transaction{
		&core.DeclareTransaction{},
		&core.DeployTransaction{},
		&core.InvokeTransaction{},
		&core.L1HandlerTransaction{},
		&core.DeployAccountTransaction{},
	}

	for _, tx := range txTypes {
		_ = encoder.RegisterType(reflect.TypeOf(tx))
	}

	_ = encoder.RegisterType(reflect.TypeOf(core.Cairo0Class{}))
	_ = encoder.RegisterType(reflect.TypeOf(core.Cairo1Class{}))
	_ = encoder.RegisterType(reflect.TypeOf(core.StateContract{}))

	code := m.Run()

	os.Exit(code)
}

func TestUpdate(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
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
		gotNewRoot, rerr := state.Root()
		require.NoError(t, rerr)
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
		gotNewRoot, rerr := state.Root()
		require.NoError(t, rerr)
		assert.Equal(t, su1.NewRoot, gotNewRoot)

		require.NoError(t, state.Update(2, su2, nil))
		gotNewRoot, err = state.Root()
		require.NoError(t, err)
		assert.Equal(t, su2.NewRoot, gotNewRoot)
	})

	su3 := &core.StateUpdate{
		OldRoot: su2.NewRoot,
		NewRoot: utils.HexToFelt(t, "0x46f1033cfb8e0b2e16e1ad6f95c41fd3a123f168fe72665452b6cddbc1d8e7a"),
		StateDiff: &core.StateDiff{
			DeclaredV1Classes: map[felt.Felt]*felt.Felt{
				*utils.HexToFelt(t, "0xDEADBEEF"): utils.HexToFelt(t, "0xBEEFDEAD"),
			},
		},
	}

	t.Run("post v0.11.0 declared classes affect root", func(t *testing.T) {
		t.Run("without class definition", func(t *testing.T) {
			require.Error(t, state.Update(3, su3, nil))
		})
		require.NoError(t, state.Update(3, su3, map[felt.Felt]core.Class{
			*utils.HexToFelt(t, "0xDEADBEEF"): &core.Cairo1Class{},
		}))
		assert.NotEqual(t, su3.NewRoot, su3.OldRoot)
	})

	// These value were taken from part of integration state update number 299762
	// https://external.integration.starknet.io/feeder_gateway/get_state_update?blockNumber=299762
	scKey := utils.HexToFelt(t, "0x492e8")
	scValue := utils.HexToFelt(t, "0x10979c6b0b36b03be36739a21cc43a51076545ce6d3397f1b45c7e286474ad5")
	scAddr := new(felt.Felt).SetUint64(1)

	su4 := &core.StateUpdate{
		OldRoot: su3.NewRoot,
		NewRoot: utils.HexToFelt(t, "0x68ac0196d9b6276b8d86f9e92bca0ed9f854d06ded5b7f0b8bc0eeaa4377d9e"),
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{*scAddr: {*scKey: scValue}},
		},
	}

	t.Run("update noClassContracts storage", func(t *testing.T) {
		require.NoError(t, state.Update(4, su4, nil))

		gotValue, err := state.ContractStorage(scAddr, scKey)
		require.NoError(t, err)

		assert.Equal(t, scValue, gotValue)

		gotNonce, err := state.ContractNonce(scAddr)
		require.NoError(t, err)

		assert.Equal(t, &felt.Zero, gotNonce)

		gotClassHash, err := state.ContractClassHash(scAddr)
		require.NoError(t, err)

		assert.Equal(t, &felt.Zero, gotClassHash)
	})

	t.Run("cannot update unknown noClassContract", func(t *testing.T) {
		scAddr2 := utils.HexToFelt(t, "0x10")
		su5 := &core.StateUpdate{
			OldRoot: su4.NewRoot,
			NewRoot: utils.HexToFelt(t, "0x68ac0196d9b6276b8d86f9e92bca0ed9f854d06ded5b7f0b8bc0eeaa4377d9e"),
			StateDiff: &core.StateDiff{
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{*scAddr2: {*scKey: scValue}},
			},
		}
		assert.ErrorIs(t, state.Update(5, su5, nil), core.ErrContractNotDeployed)
	})
}

func TestContractClassHash(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
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

	for addr, classHash := range su0.StateDiff.DeployedContracts {
		allDeployedContracts[addr] = classHash
	}

	for addr, classHash := range su1.StateDiff.DeployedContracts {
		allDeployedContracts[addr] = classHash
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
				ReplacedClasses: map[felt.Felt]*felt.Felt{
					su1FirstDeployedAddress: utils.HexToFelt(t, "0x1337"),
				},
			},
		}

		require.NoError(t, state.Update(2, replaceUpdate, nil))

		gotClassHash, err := state.ContractClassHash(new(felt.Felt).Set(&su1FirstDeployedAddress))
		require.NoError(t, err)

		assert.Equal(t, utils.HexToFelt(t, "0x1337"), gotClassHash)
	})
}

func TestNonce(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
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
			DeployedContracts: map[felt.Felt]*felt.Felt{
				*addr: utils.HexToFelt(t, "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"),
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
	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	client := feeder.NewTestClient(t, &utils.Mainnet)
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
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*contractAddr: {
					*changedLoc: utils.HexToFelt(t, "0x44"),
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
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
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
	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	client := feeder.NewTestClient(t, &utils.Integration)
	gw := adaptfeeder.New(client)

	cairo0Hash := utils.HexToFelt(t, "0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04")
	cairo0Class, err := gw.Class(context.Background(), cairo0Hash)
	require.NoError(t, err)
	cairo1Hash := utils.HexToFelt(t, "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5")
	cairo1Class, err := gw.Class(context.Background(), cairo0Hash)
	require.NoError(t, err)

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
	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	state := core.NewState(txn)
	su0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)
	require.NoError(t, state.Update(0, su0, nil))
	su1, err := gw.StateUpdate(context.Background(), 1)
	require.NoError(t, err)
	require.NoError(t, state.Update(1, su1, nil))

	t.Run("revert a replaced class", func(t *testing.T) {
		replacedVal := utils.HexToFelt(t, "0xDEADBEEF")
		replaceStateUpdate := &core.StateUpdate{
			NewRoot: utils.HexToFelt(t, "0x30b1741b28893b892ac30350e6372eac3a6f32edee12f9cdca7fbe7540a5ee"),
			OldRoot: su1.NewRoot,
			StateDiff: &core.StateDiff{
				ReplacedClasses: map[felt.Felt]*felt.Felt{
					su1FirstDeployedAddress: replacedVal,
				},
			},
		}

		require.NoError(t, state.Update(2, replaceStateUpdate, nil))
		classHash, err := state.ContractClassHash(new(felt.Felt).Set(&su1FirstDeployedAddress))
		require.NoError(t, err)
		assert.Equal(t, replacedVal, classHash)

		require.NoError(t, state.Revert(2, replaceStateUpdate))
		classHash, sErr := state.ContractClassHash(new(felt.Felt).Set(&su1FirstDeployedAddress))
		require.NoError(t, sErr)
		assert.Equal(t, su1.StateDiff.DeployedContracts[*new(felt.Felt).Set(&su1FirstDeployedAddress)], classHash)
	})

	t.Run("revert a nonce update", func(t *testing.T) {
		replacedVal := utils.HexToFelt(t, "0xDEADBEEF")
		nonceStateUpdate := &core.StateUpdate{
			NewRoot: utils.HexToFelt(t, "0x6683657d2b6797d95f318e7c6091dc2255de86b72023c15b620af12543eb62c"),
			OldRoot: su1.NewRoot,
			StateDiff: &core.StateDiff{
				Nonces: map[felt.Felt]*felt.Felt{
					su1FirstDeployedAddress: replacedVal,
				},
			},
		}

		require.NoError(t, state.Update(2, nonceStateUpdate, nil))
		nonce, sErr := state.ContractNonce(new(felt.Felt).Set(&su1FirstDeployedAddress))
		require.NoError(t, sErr)
		assert.Equal(t, replacedVal, nonce)

		require.NoError(t, state.Revert(2, nonceStateUpdate))
		nonce, sErr = state.ContractNonce(new(felt.Felt).Set(&su1FirstDeployedAddress))
		require.NoError(t, sErr)
		assert.Equal(t, &felt.Zero, nonce)
	})

	t.Run("revert declared classes", func(t *testing.T) {
		classesM := make(map[felt.Felt]core.Class)
		cairo0 := &core.Cairo0Class{
			Abi:          json.RawMessage("some cairo 0 class abi"),
			Externals:    []core.EntryPoint{{new(felt.Felt).SetBytes([]byte("e1")), new(felt.Felt).SetBytes([]byte("e2"))}},
			L1Handlers:   []core.EntryPoint{{new(felt.Felt).SetBytes([]byte("l1")), new(felt.Felt).SetBytes([]byte("l2"))}},
			Constructors: []core.EntryPoint{{new(felt.Felt).SetBytes([]byte("c1")), new(felt.Felt).SetBytes([]byte("c2"))}},
			Program:      "some cairo 0 program",
		}

		cairo0Addr := utils.HexToFelt(t, "0xab1234")
		classesM[*cairo0Addr] = cairo0

		cairo1 := &core.Cairo1Class{
			Abi:     "some cairo 1 class abi",
			AbiHash: utils.HexToFelt(t, "0xcd98"),
			EntryPoints: struct {
				Constructor []core.SierraEntryPoint
				External    []core.SierraEntryPoint
				L1Handler   []core.SierraEntryPoint
			}{
				Constructor: []core.SierraEntryPoint{{1, new(felt.Felt).SetBytes([]byte("c1"))}},
				External:    []core.SierraEntryPoint{{0, new(felt.Felt).SetBytes([]byte("e1"))}},
				L1Handler:   []core.SierraEntryPoint{{2, new(felt.Felt).SetBytes([]byte("l1"))}},
			},
			Program:         []*felt.Felt{new(felt.Felt).SetBytes([]byte("random program"))},
			ProgramHash:     new(felt.Felt).SetBytes([]byte("random program hash")),
			SemanticVersion: "version 1",
			Compiled:        &core.CompiledClass{},
		}

		cairo1Addr := utils.HexToFelt(t, "0xcd5678")
		classesM[*cairo1Addr] = cairo1

		declaredClassesStateUpdate := &core.StateUpdate{
			NewRoot: utils.HexToFelt(t, "0x40427f2f4b5e1d15792e656b4d0c1d1dcf66ece1d8d60276d543aafedcc79d9"),
			OldRoot: su1.NewRoot,
			StateDiff: &core.StateDiff{
				DeclaredV0Classes: []*felt.Felt{cairo0Addr},
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{
					*cairo1Addr: utils.HexToFelt(t, "0xef9123"),
				},
			},
		}

		require.NoError(t, state.Update(2, declaredClassesStateUpdate, classesM))
		require.NoError(t, state.Revert(2, declaredClassesStateUpdate))

		var decClass *core.DeclaredClass
		decClass, err = state.Class(cairo0Addr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
		assert.Nil(t, decClass)

		decClass, err = state.Class(cairo1Addr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
		assert.Nil(t, decClass)
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
			return it.Close()
		}))
	})
}

// TestRevertGenesisStateDiff ensures the reverse diff for the genesis block sets all storage values to zero.
func TestRevertGenesisStateDiff(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})
	state := core.NewState(txn)

	addr := new(felt.Felt).SetUint64(1)
	key := new(felt.Felt).SetUint64(2)
	value := new(felt.Felt).SetUint64(3)
	su := &core.StateUpdate{
		BlockHash: new(felt.Felt),
		NewRoot:   utils.HexToFelt(t, "0xa89ee2d272016fd3708435efda2ce766692231f8c162e27065ce1607d5a9e8"),
		OldRoot:   new(felt.Felt),
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*addr: {
					*key: value,
				},
			},
		},
	}
	require.NoError(t, state.Update(0, su, nil))
	require.NoError(t, state.Revert(0, su))
}

func TestRevertNoClassContracts(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	state := core.NewState(txn)

	su0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	require.NoError(t, state.Update(0, su0, nil))

	su1, err := gw.StateUpdate(context.Background(), 1)
	require.NoError(t, err)

	// These value were taken from part of integration state update number 299762
	// https://external.integration.starknet.io/feeder_gateway/get_state_update?blockNumber=299762
	scKey := utils.HexToFelt(t, "0x492e8")
	scValue := utils.HexToFelt(t, "0x10979c6b0b36b03be36739a21cc43a51076545ce6d3397f1b45c7e286474ad5")
	scAddr := new(felt.Felt).SetUint64(1)

	// update state root
	su1.NewRoot = utils.HexToFelt(t, "0x2829ac1aea81c890339e14422fe757d6831744031479cf33a9260d14282c341")

	su1.StateDiff.StorageDiffs[*scAddr] = map[felt.Felt]*felt.Felt{*scKey: scValue}

	require.NoError(t, state.Update(1, su1, nil))

	require.NoError(t, state.Revert(1, su1))

	gotRoot, err := state.Root()
	require.NoError(t, err)

	assert.Equal(t, su0.NewRoot, gotRoot)
}

func TestRevertDeclaredClasses(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})
	state := core.NewState(txn)

	classHash := utils.HexToFelt(t, "0xDEADBEEF")
	sierraHash := utils.HexToFelt(t, "0xDEADBEEF2")
	declareDiff := &core.StateUpdate{
		OldRoot:   &felt.Zero,
		NewRoot:   utils.HexToFelt(t, "0x166a006ccf102903347ebe7b82ca0abc8c2fb82f0394d7797e5a8416afd4f8a"),
		BlockHash: &felt.Zero,
		StateDiff: &core.StateDiff{
			DeclaredV0Classes: []*felt.Felt{classHash},
			DeclaredV1Classes: map[felt.Felt]*felt.Felt{
				*sierraHash: sierraHash,
			},
		},
	}
	newClasses := map[felt.Felt]core.Class{
		*classHash:  &core.Cairo0Class{},
		*sierraHash: &core.Cairo1Class{},
	}

	require.NoError(t, state.Update(0, declareDiff, newClasses))
	declaredClass, err := state.Class(classHash)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), declaredClass.At)
	sierraClass, sErr := state.Class(sierraHash)
	require.NoError(t, sErr)
	assert.Equal(t, uint64(0), sierraClass.At)

	declareDiff.OldRoot = declareDiff.NewRoot
	require.NoError(t, state.Update(1, declareDiff, newClasses))

	t.Run("re-declaring a class shouldnt change it's DeclaredAt attribute", func(t *testing.T) {
		declaredClass, err = state.Class(classHash)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), declaredClass.At)
		sierraClass, sErr = state.Class(sierraHash)
		require.NoError(t, sErr)
		assert.Equal(t, uint64(0), sierraClass.At)
	})

	require.NoError(t, state.Revert(1, declareDiff))

	t.Run("reverting a re-declaration shouldnt change state commitment or remove class definitions", func(t *testing.T) {
		declaredClass, err = state.Class(classHash)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), declaredClass.At)
		sierraClass, sErr = state.Class(sierraHash)
		require.NoError(t, sErr)
		assert.Equal(t, uint64(0), sierraClass.At)
	})

	declareDiff.OldRoot = &felt.Zero
	require.NoError(t, state.Revert(0, declareDiff))
	_, err = state.Class(classHash)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
	_, err = state.Class(sierraHash)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}
