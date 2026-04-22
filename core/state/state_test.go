package state

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	_ "github.com/NethermindEth/juno/encoder/registry"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Address of first deployed contract in mainnet block 1's state update.
var (
	_su1FirstDeployedAddress = felt.NewUnsafeFromString[felt.Felt](
		"0x6538fdd3aa353af8a87f5fe77d1f533ea82815076e30a86d65b72d3eb4f0b80",
	)
	su1FirstDeployedAddress = *_su1FirstDeployedAddress
	su3DeclaredClasses      = func() map[felt.Felt]core.ClassDefinition {
		classHash := felt.UnsafeFromString[felt.Felt]("0xDEADBEEF")
		return map[felt.Felt]core.ClassDefinition{
			classHash: &core.SierraClass{},
		}
	}
)

const (
	block0 = 0
	block1 = 1
	block2 = 2
	block3 = 3
	block5 = 5
)

func TestNew(t *testing.T) {
	t.Run("nil batch returns error", func(t *testing.T) {
		stateDB := newTestStateDB()
		_, err := New(&felt.Zero, stateDB, nil)
		require.Error(t, err)
	})

	t.Run("valid batch returns state", func(t *testing.T) {
		stateDB := newTestStateDB()
		batch := stateDB.disk.NewBatch()
		st, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)
		assert.NotNil(t, st)
	})
}

func TestUpdate(t *testing.T) {
	// These value were taken from part of integration state update number 299762
	// https://external.integration.starknet.io/feeder_gateway/get_state_update?blockNumber=299762
	scKey := felt.NewUnsafeFromString[felt.Felt]("0x492e8")
	scValue := felt.NewUnsafeFromString[felt.Felt](
		"0x10979c6b0b36b03be36739a21cc43a51076545ce6d3397f1b45c7e286474ad5",
	)
	scAddr := felt.NewFromUint64[felt.Felt](1)

	stateUpdates := getStateUpdates(t)

	su3 := &core.StateUpdate{
		OldRoot: stateUpdates[2].NewRoot,
		NewRoot: felt.NewUnsafeFromString[felt.Felt](
			"0x46f1033cfb8e0b2e16e1ad6f95c41fd3a123f168fe72665452b6cddbc1d8e7a",
		),
		StateDiff: &core.StateDiff{
			DeclaredV1Classes: map[felt.Felt]*felt.Felt{
				*felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF"): felt.NewUnsafeFromString[felt.Felt]("0xBEEFDEAD"),
			},
		},
	}

	su4 := &core.StateUpdate{
		OldRoot: su3.NewRoot,
		NewRoot: felt.NewUnsafeFromString[felt.Felt](
			"0x68ac0196d9b6276b8d86f9e92bca0ed9f854d06ded5b7f0b8bc0eeaa4377d9e",
		),
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{*scAddr: {*scKey: scValue}},
		},
	}

	stateUpdates = append(stateUpdates, su3, su4)

	t.Run("block 0 to block 3 state updates", func(t *testing.T) {
		setupState(t, stateUpdates, 3)
	})

	t.Run("error when current state comm doesn't match the one in state diff", func(t *testing.T) {
		oldRoot := felt.NewUnsafeFromString[felt.Felt]("0xdeadbeef")
		su := &core.StateUpdate{
			OldRoot: oldRoot,
		}
		stateDB := setupState(t, stateUpdates, 0)
		batch := stateDB.disk.NewBatch()
		state, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)
		err = state.Update(&core.Header{Number: block0}, su, nil, false)
		require.Error(t, err)
	})

	t.Run("error when state's new root doesn't match state update's new root", func(t *testing.T) {
		newRoot := felt.NewUnsafeFromString[felt.Felt]("0xcafebabe")
		su := &core.StateUpdate{
			NewRoot:   newRoot,
			OldRoot:   stateUpdates[0].NewRoot,
			StateDiff: new(core.StateDiff),
		}
		stateDB := setupState(t, stateUpdates, 0)
		batch := stateDB.disk.NewBatch()
		state, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)
		err = state.Update(&core.Header{Number: block0}, su, nil, false)
		require.Error(t, err)
	})

	t.Run("post v0.11.0 declared classes affect root", func(t *testing.T) {
		t.Run("without class definition", func(t *testing.T) {
			stateDB := setupState(t, stateUpdates, 3)
			batch := stateDB.disk.NewBatch()
			state, err := New(stateUpdates[3].OldRoot, stateDB, batch)
			require.NoError(t, err)
			require.Error(t, state.Update(&core.Header{Number: block3}, stateUpdates[3], nil, false))
		})
		t.Run("with class definition", func(t *testing.T) {
			stateDB := setupState(t, stateUpdates, 3)
			batch := stateDB.disk.NewBatch()
			state, err := New(stateUpdates[3].OldRoot, stateDB, batch)
			require.NoError(t, err)
			require.NoError(t, state.Update(&core.Header{Number: block3}, su3, su3DeclaredClasses(), false))
		})
	})

	t.Run("update noClassContracts storage", func(t *testing.T) {
		stateDB := setupState(t, stateUpdates, 5)
		batch := stateDB.disk.NewBatch()
		state, err := New(stateUpdates[4].NewRoot, stateDB, batch)
		require.NoError(t, err)

		gotValue, err := state.ContractStorage(scAddr, scKey)
		require.NoError(t, err)
		assert.Equal(t, *scValue, gotValue)

		gotNonce, err := state.ContractNonce(scAddr)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, gotNonce)

		gotClassHash, err := state.ContractClassHash(scAddr)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, gotClassHash)
	})

	t.Run("cannot update unknown noClassContract", func(t *testing.T) {
		scAddr2 := felt.NewUnsafeFromString[felt.Felt]("0x10")
		su5 := &core.StateUpdate{
			OldRoot: su4.NewRoot,
			NewRoot: felt.NewUnsafeFromString[felt.Felt](
				"0x68ac0196d9b6276b8d86f9e92bca0ed9f854d06ded5b7f0b8bc0eeaa4377d9e",
			),
			StateDiff: &core.StateDiff{
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{*scAddr2: {*scKey: scValue}},
			},
		}
		stateDB := setupState(t, stateUpdates, 5)
		batch := stateDB.disk.NewBatch()
		state, err := New(stateUpdates[4].NewRoot, stateDB, batch)
		require.NoError(t, err)
		err = state.Update(&core.Header{Number: block5}, su5, nil, false)
		require.ErrorIs(t, err, ErrContractNotDeployed)
	})
}

func TestRevert(t *testing.T) {
	stateUpdates := getStateUpdates(t)

	su0 := stateUpdates[0]
	su1 := stateUpdates[1]
	su2 := stateUpdates[2]

	t.Run("revert a replace class", func(t *testing.T) {
		stateDB := setupState(t, stateUpdates, 2)

		replacedVal := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")
		replaceStateUpdate := &core.StateUpdate{
			NewRoot: felt.NewUnsafeFromString[felt.Felt](
				"0x30b1741b28893b892ac30350e6372eac3a6f32edee12f9cdca7fbe7540a5ee",
			),
			OldRoot: su1.NewRoot,
			StateDiff: &core.StateDiff{
				ReplacedClasses: map[felt.Felt]*felt.Felt{
					su1FirstDeployedAddress: replacedVal,
				},
			},
		}

		batch := stateDB.disk.NewBatch()
		state, err := New(su1.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block2}, replaceStateUpdate, nil, false))
		require.NoError(t, batch.Write())

		reader, err := NewStateReader(replaceStateUpdate.NewRoot, stateDB)
		require.NoError(t, err)
		gotClassHash, err := reader.ContractClassHash(&su1FirstDeployedAddress)
		require.NoError(t, err)
		assert.Equal(t, *replacedVal, gotClassHash)

		batch1 := stateDB.disk.NewBatch()
		state, err = New(replaceStateUpdate.NewRoot, stateDB, batch1)
		require.NoError(t, err)
		require.NoError(t, state.Revert(&core.Header{Number: block2}, replaceStateUpdate))
		require.NoError(t, batch1.Write())

		reader, err = NewStateReader(replaceStateUpdate.OldRoot, stateDB)
		require.NoError(t, err)
		gotClassHash, err = reader.ContractClassHash(&su1FirstDeployedAddress)
		require.NoError(t, err)
		assert.Equal(t, *su1.StateDiff.DeployedContracts[su1FirstDeployedAddress], gotClassHash)
	})

	t.Run("revert a nonce update", func(t *testing.T) {
		stateDB := setupState(t, stateUpdates, 2)

		replacedVal := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")
		nonceStateUpdate := &core.StateUpdate{
			NewRoot: felt.NewUnsafeFromString[felt.Felt](
				"0x6683657d2b6797d95f318e7c6091dc2255de86b72023c15b620af12543eb62c",
			),
			OldRoot: su1.NewRoot,
			StateDiff: &core.StateDiff{
				Nonces: map[felt.Felt]*felt.Felt{
					su1FirstDeployedAddress: replacedVal,
				},
			},
		}

		batch := stateDB.disk.NewBatch()
		state, err := New(su1.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block2}, nonceStateUpdate, nil, false))
		require.NoError(t, batch.Write())

		reader, err := NewStateReader(nonceStateUpdate.NewRoot, stateDB)
		require.NoError(t, err)
		gotNonce, err := reader.ContractNonce(&su1FirstDeployedAddress)
		require.NoError(t, err)
		assert.Equal(t, *replacedVal, gotNonce)

		batch = stateDB.disk.NewBatch()
		state, err = New(nonceStateUpdate.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Revert(&core.Header{Number: block2}, nonceStateUpdate))
		require.NoError(t, batch.Write())

		reader, err = NewStateReader(nonceStateUpdate.OldRoot, stateDB)
		require.NoError(t, err)
		gotNonce, err = reader.ContractNonce(&su1FirstDeployedAddress)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, gotNonce)
	})

	t.Run("revert a storage update", func(t *testing.T) {
		stateDB := setupState(t, stateUpdates, 2)

		replacedVal := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")
		storageStateUpdate := &core.StateUpdate{
			NewRoot: felt.NewUnsafeFromString[felt.Felt](
				"0x7bc3bf782373601d53e0ac26357e6df4a4e313af8e65414c92152810d8d0626",
			),
			OldRoot: su1.NewRoot,
			StateDiff: &core.StateDiff{
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
					su1FirstDeployedAddress: {
						*replacedVal: replacedVal,
					},
				},
			},
		}

		batch := stateDB.disk.NewBatch()
		state, err := New(su1.NewRoot, stateDB, batch)
		require.NoError(t, err)

		require.NoError(t, state.Update(&core.Header{Number: block2}, storageStateUpdate, nil, false))
		require.NoError(t, batch.Write())

		reader, err := NewStateReader(storageStateUpdate.NewRoot, stateDB)
		require.NoError(t, err)
		gotStorage, err := reader.ContractStorage(&su1FirstDeployedAddress, replacedVal)
		require.NoError(t, err)
		assert.Equal(t, *replacedVal, gotStorage)

		batch = stateDB.disk.NewBatch()
		state, err = New(storageStateUpdate.NewRoot, stateDB, batch)
		require.NoError(t, err)

		require.NoError(t, state.Revert(&core.Header{Number: block2}, storageStateUpdate))
		require.NoError(t, batch.Write())

		storage, sErr := state.ContractStorage(&su1FirstDeployedAddress, replacedVal)
		require.NoError(t, sErr)
		assert.Equal(t, felt.Zero, storage)
	})

	t.Run("revert a declare class", func(t *testing.T) {
		stateDB := setupState(t, stateUpdates, 2)

		classesM := make(map[felt.Felt]core.ClassDefinition)
		deprecatedCairo := &core.DeprecatedCairoClass{
			Abi: json.RawMessage("some cairo 0 class abi"),
			Externals: []core.DeprecatedEntryPoint{{
				Selector: felt.NewUnsafeFromString[felt.Felt]("0xe1"),
				Offset:   felt.NewUnsafeFromString[felt.Felt]("0xe2"),
			}},
			L1Handlers: []core.DeprecatedEntryPoint{{
				Selector: felt.NewUnsafeFromString[felt.Felt]("0xb1"),
				Offset:   felt.NewUnsafeFromString[felt.Felt]("0xb2"),
			}},
			Constructors: []core.DeprecatedEntryPoint{{
				Selector: felt.NewUnsafeFromString[felt.Felt]("0xc1"),
				Offset:   felt.NewUnsafeFromString[felt.Felt]("0xc2"),
			}},
			Program: "some cairo 0 program",
		}

		deprecatedCairoAddr := felt.NewUnsafeFromString[felt.Felt]("0xab1234")
		classesM[*deprecatedCairoAddr] = deprecatedCairo

		sierra := &core.SierraClass{
			Abi:     "some sierra class abi",
			AbiHash: felt.NewUnsafeFromString[felt.Felt]("0xcd98"),
			EntryPoints: struct {
				Constructor []core.SierraEntryPoint
				External    []core.SierraEntryPoint
				L1Handler   []core.SierraEntryPoint
			}{
				Constructor: []core.SierraEntryPoint{{
					Index:    1,
					Selector: felt.NewUnsafeFromString[felt.Felt]("0xc1"),
				}},
				External: []core.SierraEntryPoint{{
					Index:    0,
					Selector: felt.NewUnsafeFromString[felt.Felt]("0xe1"),
				}},
				L1Handler: []core.SierraEntryPoint{{
					Index:    2,
					Selector: felt.NewUnsafeFromString[felt.Felt]("0xb1"),
				}},
			},
			Program:         []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x9999")},
			ProgramHash:     felt.NewUnsafeFromString[felt.Felt]("0x8888"),
			SemanticVersion: "version 1",
			Compiled:        &core.CasmClass{},
		}

		cairo1Addr := felt.NewUnsafeFromString[felt.Felt]("0xcd5678")
		classesM[*cairo1Addr] = sierra

		declaredClassesStateUpdate := &core.StateUpdate{
			NewRoot: felt.NewUnsafeFromString[felt.Felt](
				"0x40427f2f4b5e1d15792e656b4d0c1d1dcf66ece1d8d60276d543aafedcc79d9",
			),
			OldRoot: su1.NewRoot,
			StateDiff: &core.StateDiff{
				DeclaredV0Classes: []*felt.Felt{deprecatedCairoAddr},
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{
					*cairo1Addr: felt.NewUnsafeFromString[felt.Felt]("0xef9123"),
				},
			},
		}

		batch := stateDB.disk.NewBatch()
		state, err := New(su1.NewRoot, stateDB, batch)
		require.NoError(t, err)
		header := &core.Header{Number: block2}
		require.NoError(t, state.Update(header, declaredClassesStateUpdate, classesM, false))
		require.NoError(t, batch.Write())

		batch = stateDB.disk.NewBatch()
		state, err = New(declaredClassesStateUpdate.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Revert(&core.Header{Number: block2}, declaredClassesStateUpdate))
		require.NoError(t, batch.Write())

		var decClass *core.DeclaredClassDefinition
		decClass, err = state.Class(deprecatedCairoAddr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
		assert.Nil(t, decClass)

		decClass, err = state.Class(cairo1Addr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
		assert.Nil(t, decClass)
	})

	t.Run("should be able to update after a revert", func(t *testing.T) {
		stateDB := setupState(t, stateUpdates, 2)

		batch := stateDB.disk.NewBatch()
		state, err := New(su1.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block2}, su2, nil, false))
		require.NoError(t, batch.Write())

		batch = stateDB.disk.NewBatch()
		state, err = New(su2.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Revert(&core.Header{Number: block2}, su2))
		require.NoError(t, batch.Write())

		batch = stateDB.disk.NewBatch()
		state, err = New(su1.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block2}, su2, nil, false))
	})

	t.Run("should be able to revert all the updates", func(t *testing.T) {
		stateDB := setupState(t, stateUpdates, 3)

		batch := stateDB.disk.NewBatch()
		state, err := New(su2.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Revert(&core.Header{Number: block2}, su2))
		require.NoError(t, batch.Write())

		batch = stateDB.disk.NewBatch()
		state, err = New(su1.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Revert(&core.Header{Number: block1}, su1))
		require.NoError(t, batch.Write())

		batch = stateDB.disk.NewBatch()
		state, err = New(su0.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Revert(&core.Header{Number: block0}, su0))
		require.NoError(t, batch.Write())
	})

	t.Run("revert no class contracts", func(t *testing.T) {
		stateDB := setupState(t, stateUpdates, 1)

		su1 := *stateUpdates[1]

		// These value were taken from part of integration state update number 299762
		// https://external.integration.starknet.io/feeder_gateway/get_state_update?blockNumber=299762
		scKey := felt.NewUnsafeFromString[felt.Felt]("0x492e8")
		scValue := felt.NewUnsafeFromString[felt.Felt](
			"0x10979c6b0b36b03be36739a21cc43a51076545ce6d3397f1b45c7e286474ad5",
		)

		// update state root
		su1.NewRoot = felt.NewUnsafeFromString[felt.Felt](
			"0x2829ac1aea81c890339e14422fe757d6831744031479cf33a9260d14282c341",
		)
		su1.StateDiff.StorageDiffs[felt.One] = map[felt.Felt]*felt.Felt{*scKey: scValue}

		batch := stateDB.disk.NewBatch()
		state, err := New(su1.OldRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block1}, &su1, nil, false))
		require.NoError(t, batch.Write())

		batch = stateDB.disk.NewBatch()
		state, err = New(su1.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Revert(&core.Header{Number: block1}, &su1))
		require.NoError(t, batch.Write())
	})

	t.Run("revert declared classes", func(t *testing.T) {
		stateDB := newTestStateDB()

		classHash := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")
		sierraHash := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF2")
		declareDiff := &core.StateUpdate{
			OldRoot: &felt.Zero,
			NewRoot: felt.NewUnsafeFromString[felt.Felt](
				"0x166a006ccf102903347ebe7b82ca0abc8c2fb82f0394d7797e5a8416afd4f8a",
			),
			BlockHash: &felt.Zero,
			StateDiff: &core.StateDiff{
				DeclaredV0Classes: []*felt.Felt{classHash},
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{
					*sierraHash: sierraHash,
				},
			},
		}
		newClasses := map[felt.Felt]core.ClassDefinition{
			*classHash:  &core.DeprecatedCairoClass{},
			*sierraHash: &core.SierraClass{},
		}

		batch := stateDB.disk.NewBatch()
		state, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block0}, declareDiff, newClasses, false))
		require.NoError(t, batch.Write())

		declaredClass, err := state.Class(classHash)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), declaredClass.At)
		sierraClass, err := state.Class(sierraHash)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), sierraClass.At)

		batch = stateDB.disk.NewBatch()
		state, err = New(declareDiff.NewRoot, stateDB, batch)
		require.NoError(t, err)
		declareDiff.OldRoot = declareDiff.NewRoot
		require.NoError(t, state.Update(&core.Header{Number: block1}, declareDiff, newClasses, false))
		require.NoError(t, batch.Write())

		// Redeclaring should not change the declared at block number
		declaredClass, err = state.Class(classHash)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), declaredClass.At)
		sierraClass, err = state.Class(sierraHash)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), sierraClass.At)

		batch = stateDB.disk.NewBatch()
		state, err = New(declareDiff.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Revert(&core.Header{Number: block1}, declareDiff))
		require.NoError(t, batch.Write())

		// Reverting a re-declaration should not change state commitment or remove class definitions
		declaredClass, err = state.Class(classHash)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), declaredClass.At)
		sierraClass, err = state.Class(sierraHash)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), sierraClass.At)

		batch = stateDB.disk.NewBatch()
		state, err = New(declareDiff.NewRoot, stateDB, batch)
		require.NoError(t, err)
		declareDiff.OldRoot = &felt.Zero
		require.NoError(t, state.Revert(&core.Header{Number: block0}, declareDiff))
		require.NoError(t, batch.Write())

		declaredClass, err = state.Class(classHash)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
		assert.Nil(t, declaredClass)
		sierraClass, err = state.Class(sierraHash)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
		assert.Nil(t, sierraClass)
	})

	t.Run("revert genesis", func(t *testing.T) {
		stateDB := newTestStateDB()

		addr := felt.NewFromUint64[felt.Felt](1)
		key := felt.NewFromUint64[felt.Felt](2)
		value := felt.NewFromUint64[felt.Felt](3)
		su := &core.StateUpdate{
			BlockHash: &felt.Zero,
			NewRoot: felt.NewUnsafeFromString[felt.Felt](
				"0xa89ee2d272016fd3708435efda2ce766692231f8c162e27065ce1607d5a9e8",
			),
			OldRoot: &felt.Zero,
			StateDiff: &core.StateDiff{
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
					*addr: {
						*key: value,
					},
				},
			},
		}

		batch := stateDB.disk.NewBatch()
		state, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block0}, su, nil, false))
		require.NoError(t, batch.Write())

		batch = stateDB.disk.NewBatch()
		state, err = New(su.NewRoot, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Revert(&core.Header{Number: block0}, su))
		require.NoError(t, batch.Write())
	})

	t.Run("db should be empty after block0 revert", func(t *testing.T) {
		stateDB := setupState(t, stateUpdates, 1)

		batch := stateDB.disk.NewBatch()
		state, err := New(su0.NewRoot, stateDB, batch)
		require.NoError(t, err)

		require.NoError(t, state.Revert(&core.Header{Number: block0}, su0))
		require.NoError(t, batch.Write())

		it, err := stateDB.disk.NewIterator(nil, false)
		require.NoError(t, err)
		defer it.Close()

		for it.First(); it.Next(); it.Valid() {
			key := it.Key()
			val, err := it.Value()
			require.NoError(t, err)
			t.Errorf("key: %v, val: %v", key, val)
		}
	})

	// Revert with MigratedClasses: State only restores V1 in the class trie.
	// Writing/reverting CASM hash metadata is the caller's responsibility.
	t.Run("revert migrated casm classes with caller-managed metadata", func(t *testing.T) {
		stateDB := newTestStateDB()
		sierraHash := felt.FromUint64[felt.SierraClassHash](0x1234)
		v1CasmHash := felt.FromUint64[felt.CasmClassHash](0x1111)
		v2CasmHash := felt.FromUint64[felt.CasmClassHash](0x2222)
		sierraHashFelt := (*felt.Felt)(&sierraHash)

		classes := map[felt.Felt]core.ClassDefinition{
			*sierraHashFelt: &core.SierraClass{},
		}

		su0 := &core.StateUpdate{
			OldRoot: &felt.Zero,
			StateDiff: &core.StateDiff{
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{
					*sierraHashFelt: (*felt.Felt)(&v1CasmHash),
				},
			},
		}

		batch := stateDB.disk.NewBatch()
		state, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block0}, su0, classes, true))
		require.NoError(t, batch.Write())
		root0, err := state.Commitment("")
		require.NoError(t, err)

		// Caller writes CASM hash metadata (State does not).
		metadata := core.NewCasmHashMetadataDeclaredV1(block0, &v1CasmHash, &v2CasmHash)
		require.NoError(t, core.WriteClassCasmHashMetadata(stateDB.disk, &sierraHash, &metadata))

		su1 := &core.StateUpdate{
			OldRoot: &root0,
			StateDiff: &core.StateDiff{
				MigratedClasses: map[felt.SierraClassHash]felt.CasmClassHash{
					sierraHash: v2CasmHash,
				},
			},
		}

		batch = stateDB.disk.NewBatch()
		state, err = New(&root0, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block1}, su1, nil, true))
		require.NoError(t, batch.Write())
		newRoot, err := state.Commitment("")
		require.NoError(t, err)
		su1.NewRoot = &newRoot

		// Caller migrates metadata (State does not).
		require.NoError(t, metadata.Migrate(block1))
		require.NoError(t, core.WriteClassCasmHashMetadata(stateDB.disk, &sierraHash, &metadata))

		batch = stateDB.disk.NewBatch()
		state, err = New(su1.NewRoot, stateDB, batch)
		require.NoError(t, err)
		stateRoot, err := state.Commitment("")
		require.NoError(t, err)
		assert.Equal(t, *su1.NewRoot, stateRoot, "state root after migration should match new root")
		gotCasmHash, err := state.CompiledClassHash(&sierraHash)
		require.NoError(t, err)
		require.Equal(t, v2CasmHash, gotCasmHash, "CompiledClassHash should return V2 before revert")
		// Revert: State restores V1 in the class trie only; it does not write metadata.
		require.NoError(t, state.Revert(&core.Header{Number: block1}, su1))
		require.NoError(t, batch.Write())

		metadataReverted, err := core.GetClassCasmHashMetadata(stateDB.disk, &sierraHash)
		require.NoError(t, err)
		require.NoError(t, metadataReverted.Unmigrate())
		require.NoError(t, core.WriteClassCasmHashMetadata(stateDB.disk, &sierraHash, &metadataReverted))

		batch = stateDB.disk.NewBatch()
		state, err = New(&root0, stateDB, batch)
		require.NoError(t, err)
		gotRoot, err := state.Commitment("")
		require.NoError(t, err)
		assert.Equal(t, root0, gotRoot, "commitment after revert should match block 0")
		gotCasmHash, err = state.CompiledClassHash(&sierraHash)
		require.NoError(t, err)
		assert.Equal(t, v1CasmHash, gotCasmHash, "should return V1 after caller persisted metadata")
	})
}

func BenchmarkStateUpdate(b *testing.B) {
	client := feeder.NewTestClient(b, &networks.Mainnet)
	gw := adaptfeeder.New(client)

	su0, err := gw.StateUpdate(b.Context(), block0)
	require.NoError(b, err)

	su1, err := gw.StateUpdate(b.Context(), block1)
	require.NoError(b, err)

	su2, err := gw.StateUpdate(b.Context(), block2)
	require.NoError(b, err)

	stateUpdates := []*core.StateUpdate{su0, su1, su2}

	for b.Loop() {
		stateDB := newTestStateDB()
		for i, su := range stateUpdates {
			batch := stateDB.disk.NewBatch()
			state, err := New(su.OldRoot, stateDB, batch)
			require.NoError(b, err)
			err = state.Update(&core.Header{Number: uint64(i)}, su, nil, false)
			if err != nil {
				b.Fatalf("Error updating state: %v", err)
			}
			require.NoError(b, batch.Write())
		}
	}
}

// Get the first 3 state updates from the mainnet.
func getStateUpdates(t *testing.T) []*core.StateUpdate {
	client := feeder.NewTestClient(t, &networks.Mainnet)
	gw := adaptfeeder.New(client)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	su0, err := gw.StateUpdate(ctx, 0)
	require.NoError(t, err)

	su1, err := gw.StateUpdate(ctx, 1)
	require.NoError(t, err)

	su2, err := gw.StateUpdate(ctx, 2)
	require.NoError(t, err)

	return []*core.StateUpdate{su0, su1, su2}
}

// Create a new state from a new database and update it with the given state updates.
func setupState(t *testing.T, stateUpdates []*core.StateUpdate, blocks uint64) *StateDB {
	stateDB := newTestStateDB()
	for i, su := range stateUpdates[:blocks] {
		batch := stateDB.disk.NewBatch()
		state, err := New(su.OldRoot, stateDB, batch)
		require.NoError(t, err)
		var declaredClasses map[felt.Felt]core.ClassDefinition
		if i == 3 {
			declaredClasses = su3DeclaredClasses()
		}
		require.NoError(
			t,
			state.Update(&core.Header{Number: uint64(i)}, su, declaredClasses, false),
			"failed to update state for block %d",
			i,
		)
		require.NoError(t, batch.Write())
		newComm, err := state.Commitment("")
		require.NoError(t, err)
		assert.Equal(t, *su.NewRoot, newComm)
	}

	return stateDB
}

func newTestStateDB() *StateDB {
	memDB := memory.New()
	trieDB := triedb.New(memDB, nil)
	return NewStateDB(memDB, trieDB)
}
