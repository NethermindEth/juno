package state

import (
	"context"
	"maps"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db/memory"
	_ "github.com/NethermindEth/juno/encoder/registry"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Address of first deployed contract in mainnet block 1's state update.
var (
	_su1FirstDeployedAddress, _ = new(felt.Felt).SetString("0x6538fdd3aa353af8a87f5fe77d1f533ea82815076e30a86d65b72d3eb4f0b80")
	su1FirstDeployedAddress     = *_su1FirstDeployedAddress
	su3DeclaredClasses          = func() map[felt.Felt]core.Class {
		classHash, _ := new(felt.Felt).SetString("0xDEADBEEF")
		return map[felt.Felt]core.Class{
			*classHash: &core.Cairo1Class{},
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

func TestUpdate(t *testing.T) {
	runWithDBTypes(t, func(t *testing.T, dbType triedb.Scheme) {
		// These value were taken from part of integration state update number 299762
		scKey := utils.HexToFelt(t, "0x492e8")
		scValue := utils.HexToFelt(t, "0x10979c6b0b36b03be36739a21cc43a51076545ce6d3397f1b45c7e286474ad5")
		scAddr := new(felt.Felt).SetUint64(1)

		stateUpdates := getStateUpdates(t)

		su3 := &core.StateUpdate{
			OldRoot: stateUpdates[2].NewRoot,
			NewRoot: utils.HexToFelt(t, "0x46f1033cfb8e0b2e16e1ad6f95c41fd3a123f168fe72665452b6cddbc1d8e7a"),
			StateDiff: &core.StateDiff{
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{
					*utils.HexToFelt(t, "0xDEADBEEF"): utils.HexToFelt(t, "0xBEEFDEAD"),
				},
			},
		}

		su4 := &core.StateUpdate{
			OldRoot: su3.NewRoot,
			NewRoot: utils.HexToFelt(t, "0x68ac0196d9b6276b8d86f9e92bca0ed9f854d06ded5b7f0b8bc0eeaa4377d9e"),
			StateDiff: &core.StateDiff{
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{*scAddr: {*scKey: scValue}},
			},
		}

		stateUpdates = append(stateUpdates, su3, su4)

		t.Run("block 0 to block 3 state updates", func(t *testing.T) {
			setupState(t, stateUpdates, 3, dbType)
		})

		t.Run("error when current state comm doesn't match the one in state diff", func(t *testing.T) {
			oldRoot := new(felt.Felt).SetBytes([]byte("some old root"))
			su := &core.StateUpdate{
				OldRoot: oldRoot,
			}
			stateDB := setupState(t, stateUpdates, 0, dbType)
			state, err := New(felt.Zero, stateDB)
			require.NoError(t, err)
			err = state.Update(block0, su, nil)
			require.Error(t, err)
		})

		t.Run("error when state's new root doesn't match state update's new root", func(t *testing.T) {
			newRoot := new(felt.Felt).SetBytes([]byte("some new root"))
			su := &core.StateUpdate{
				NewRoot:   newRoot,
				OldRoot:   stateUpdates[0].NewRoot,
				StateDiff: new(core.StateDiff),
			}
			stateDB := setupState(t, stateUpdates, 0, dbType)
			state, err := New(felt.Zero, stateDB)
			require.NoError(t, err)
			err = state.Update(block0, su, nil)
			require.Error(t, err)
		})

		t.Run("post v0.11.0 declared classes affect root", func(t *testing.T) {
			t.Run("without class definition", func(t *testing.T) {
				stateDB := setupState(t, stateUpdates, 3, dbType)
				state, err := New(*stateUpdates[3].OldRoot, stateDB)
				require.NoError(t, err)
				require.Error(t, state.Update(block3, stateUpdates[3], nil))
			})
			t.Run("with class definition", func(t *testing.T) {
				stateDB := setupState(t, stateUpdates, 3, dbType)
				state, err := New(*stateUpdates[3].OldRoot, stateDB)
				require.NoError(t, err)
				require.NoError(t, state.Update(block3, su3, su3DeclaredClasses()))
			})
		})

		t.Run("update noClassContracts storage", func(t *testing.T) {
			stateDB := setupState(t, stateUpdates, 5, dbType)
			state, err := New(*stateUpdates[4].NewRoot, stateDB)
			require.NoError(t, err)

			gotValue, err := state.ContractStorage(*scAddr, *scKey)
			require.NoError(t, err)
			assert.Equal(t, *scValue, gotValue)

			gotNonce, err := state.ContractNonce(*scAddr)
			require.NoError(t, err)
			assert.Equal(t, felt.Zero, gotNonce)

			gotClassHash, err := state.ContractClassHash(*scAddr)
			require.NoError(t, err)
			assert.Equal(t, felt.Zero, gotClassHash)
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
			stateDB := setupState(t, stateUpdates, 5, dbType)
			state, err := New(*stateUpdates[4].NewRoot, stateDB)
			require.NoError(t, err)
			require.ErrorIs(t, state.Update(block5, su5, nil), ErrContractNotDeployed)
		})
	})
}

func TestContractClassHash(t *testing.T) {
	runWithDBTypes(t, func(t *testing.T, dbType triedb.Scheme) {
		stateUpdates := getStateUpdates(t)
		stateUpdates = stateUpdates[:2]

		su0 := stateUpdates[0]
		su1 := stateUpdates[1]

		stateDB := setupState(t, stateUpdates, 2, dbType)
		state, err := New(*su1.NewRoot, stateDB)
		require.NoError(t, err)

		allDeployedContracts := make(map[felt.Felt]*felt.Felt)
		maps.Copy(allDeployedContracts, su0.StateDiff.DeployedContracts)
		maps.Copy(allDeployedContracts, su1.StateDiff.DeployedContracts)

		for addr, expected := range allDeployedContracts {
			gotClassHash, err := state.ContractClassHash(addr)
			require.NoError(t, err)
			assert.Equal(t, *expected, gotClassHash)
		}
	})
}

func TestNonce(t *testing.T) {
	runWithDBTypes(t, func(t *testing.T, dbType triedb.Scheme) {
		addr := utils.HexToFelt(t, "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6")
		root := utils.HexToFelt(t, "0x4bdef7bf8b81a868aeab4b48ef952415fe105ab479e2f7bc671c92173542368")

		su0 := &core.StateUpdate{
			OldRoot: &felt.Zero,
			NewRoot: root,
			StateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Felt]*felt.Felt{
					*addr: utils.HexToFelt(t, "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"),
				},
			},
		}

		t.Run("newly deployed contract has zero nonce", func(t *testing.T) {
			stateDB := setupState(t, nil, 0, dbType)
			state, err := New(felt.Zero, stateDB)
			require.NoError(t, err)
			require.NoError(t, state.Update(block0, su0, nil))

			gotNonce, err := state.ContractNonce(*addr)
			require.NoError(t, err)
			assert.Equal(t, felt.Zero, gotNonce)
		})

		t.Run("update contract nonce", func(t *testing.T) {
			stateDB := setupState(t, nil, 0, dbType)
			state, err := New(felt.Zero, stateDB)
			require.NoError(t, err)
			require.NoError(t, state.Update(block0, su0, nil))

			expectedNonce := new(felt.Felt).SetUint64(1)
			su1 := &core.StateUpdate{
				NewRoot: utils.HexToFelt(t, "0x6210642ffd49f64617fc9e5c0bbe53a6a92769e2996eb312a42d2bdb7f2afc1"),
				OldRoot: root,
				StateDiff: &core.StateDiff{
					Nonces: map[felt.Felt]*felt.Felt{*addr: expectedNonce},
				},
			}

			state1, err := New(*su1.OldRoot, stateDB)
			require.NoError(t, err)
			require.NoError(t, state1.Update(block1, su1, nil))

			gotNonce, err := state1.ContractNonce(*addr)
			require.NoError(t, err)
			assert.Equal(t, *expectedNonce, gotNonce)
		})
	})
}

func TestClass(t *testing.T) {
	runWithDBTypes(t, func(t *testing.T, dbType triedb.Scheme) {
		stateDB := setupState(t, nil, 0, dbType)

		client := feeder.NewTestClient(t, &utils.Integration)
		gw := adaptfeeder.New(client)

		cairo0Hash := utils.HexToFelt(t, "0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04")
		cairo0Class, err := gw.Class(t.Context(), cairo0Hash)
		require.NoError(t, err)
		cairo1Hash := utils.HexToFelt(t, "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5")
		cairo1Class, err := gw.Class(t.Context(), cairo1Hash)
		require.NoError(t, err)

		state, err := New(felt.Zero, stateDB)
		require.NoError(t, err)

		su0, err := gw.StateUpdate(t.Context(), 0)
		require.NoError(t, err)
		require.NoError(t, state.Update(0, su0, map[felt.Felt]core.Class{
			*cairo0Hash: cairo0Class,
			*cairo1Hash: cairo1Class,
		}))

		gotCairo1Class, err := state.Class(*cairo1Hash)
		require.NoError(t, err)
		assert.Zero(t, gotCairo1Class.At)
		assert.Equal(t, cairo1Class, gotCairo1Class.Class)

		gotCairo0Class, err := state.Class(*cairo0Hash)
		require.NoError(t, err)
		assert.Zero(t, gotCairo0Class.At)
		assert.Equal(t, cairo0Class, gotCairo0Class.Class)
	})
}

func TestContractDeployedAt(t *testing.T) {
	runWithDBTypes(t, func(t *testing.T, dbType triedb.Scheme) {
		stateUpdates := getStateUpdates(t)
		stateDB := setupState(t, stateUpdates, 2, dbType)
		root := *stateUpdates[1].NewRoot

		t.Run("deployed on genesis", func(t *testing.T) {
			state, err := New(root, stateDB)
			require.NoError(t, err)

			d0 := utils.HexToFelt(t, "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6")
			deployed, err := state.ContractDeployedAt(*d0, block0)
			require.NoError(t, err)
			assert.True(t, deployed)

			deployed, err = state.ContractDeployedAt(*d0, block1)
			require.NoError(t, err)
			assert.True(t, deployed)
		})

		t.Run("deployed after genesis", func(t *testing.T) {
			state, err := New(root, stateDB)
			require.NoError(t, err)

			d1 := utils.HexToFelt(t, "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6")
			deployed, err := state.ContractDeployedAt(*d1, block0)
			require.NoError(t, err)
			assert.True(t, deployed)

			deployed, err = state.ContractDeployedAt(*d1, block1)
			require.NoError(t, err)
			assert.True(t, deployed)
		})

		t.Run("not deployed", func(t *testing.T) {
			state, err := New(root, stateDB)
			require.NoError(t, err)

			notDeployed := utils.HexToFelt(t, "0xDEADBEEF")
			deployed, err := state.ContractDeployedAt(*notDeployed, block0)
			require.NoError(t, err)
			assert.False(t, deployed)
		})
	})
}

func TestRevert(t *testing.T) {
	runWithDBTypes(t, func(t *testing.T, dbType triedb.Scheme) {
		stateUpdates := getStateUpdates(t)

		su1 := stateUpdates[1]

		t.Run("revert a replace class", func(t *testing.T) {
			stateDB := setupState(t, stateUpdates, 2, dbType)

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

			state, err := New(*su1.NewRoot, stateDB)
			require.NoError(t, err)
			require.NoError(t, state.Update(block2, replaceStateUpdate, nil))

			gotClassHash, err := state.ContractClassHash(su1FirstDeployedAddress)
			require.NoError(t, err)
			assert.Equal(t, *replacedVal, gotClassHash)

			state, err = New(*replaceStateUpdate.NewRoot, stateDB)
			require.NoError(t, err)
			require.NoError(t, state.Revert(block2, replaceStateUpdate))

			gotClassHash, err = state.ContractClassHash(su1FirstDeployedAddress)
			require.NoError(t, err)
			assert.Equal(t, *su1.StateDiff.DeployedContracts[*new(felt.Felt).Set(&su1FirstDeployedAddress)], gotClassHash)
		})

		t.Run("revert a replace class with a new class", func(t *testing.T) {
			stateDB := setupState(t, stateUpdates, 2, dbType)

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

			state, err := New(*su1.NewRoot, stateDB)
			require.NoError(t, err)
			require.NoError(t, state.Update(block2, replaceStateUpdate, nil))

			gotClassHash, err := state.ContractClassHash(su1FirstDeployedAddress)
			require.NoError(t, err)
			assert.Equal(t, *replacedVal, gotClassHash)

			state, err = New(*replaceStateUpdate.NewRoot, stateDB)
			require.NoError(t, err)
			require.NoError(t, state.Revert(block2, replaceStateUpdate))

			gotClassHash, err = state.ContractClassHash(su1FirstDeployedAddress)
			require.NoError(t, err)
			assert.Equal(t, *su1.StateDiff.DeployedContracts[*new(felt.Felt).Set(&su1FirstDeployedAddress)], gotClassHash)
		})

		t.Run("revert a replace class with a new class and storage", func(t *testing.T) {
			stateDB := setupState(t, stateUpdates, 2, dbType)

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

			state, err := New(*su1.NewRoot, stateDB)
			require.NoError(t, err)
			require.NoError(t, state.Update(block2, replaceStateUpdate, nil))

			gotClassHash, err := state.ContractClassHash(su1FirstDeployedAddress)
			require.NoError(t, err)
			assert.Equal(t, *replacedVal, gotClassHash)

			state, err = New(*replaceStateUpdate.NewRoot, stateDB)
			require.NoError(t, err)
			require.NoError(t, state.Revert(block2, replaceStateUpdate))

			gotClassHash, err = state.ContractClassHash(su1FirstDeployedAddress)
			require.NoError(t, err)
			assert.Equal(t, *su1.StateDiff.DeployedContracts[*new(felt.Felt).Set(&su1FirstDeployedAddress)], gotClassHash)
		})
	})
}

func TestContractHistory(t *testing.T) {
	runWithDBTypes(t, func(t *testing.T, dbType triedb.Scheme) {
		addr := utils.HexToFelt(t, "0x1234567890abcdef")
		classHash := new(felt.Felt).SetBytes([]byte("class_hash"))
		nonce := new(felt.Felt).SetBytes([]byte("nonce"))
		storageKey := new(felt.Felt).SetBytes([]byte("storage_key"))
		storageValue := new(felt.Felt).SetBytes([]byte("storage_value"))

		su := &core.StateUpdate{
			OldRoot: &felt.Zero,
			NewRoot: utils.HexToFelt(t, "0x55075b726402e12fa85ad5a063774764b5f3f119053d4d72e1ef3986063bee6"),
			StateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Felt]*felt.Felt{*addr: classHash},
				Nonces:            map[felt.Felt]*felt.Felt{*addr: nonce},
				StorageDiffs:      map[felt.Felt]map[felt.Felt]*felt.Felt{*addr: {*storageKey: storageValue}},
			},
		}

		t.Run("empty", func(t *testing.T) {
			stateDB := newTestStateDB(dbType)
			state, err := New(felt.Zero, stateDB)
			require.NoError(t, err)

			gotNonce, err := state.ContractNonceAt(*addr, block0)
			require.NoError(t, err)
			assert.Equal(t, felt.Zero, gotNonce)

			gotClassHash, err := state.ContractClassHashAt(*addr, block0)
			require.NoError(t, err)
			assert.Equal(t, felt.Zero, gotClassHash)

			gotStorage, err := state.ContractStorageAt(*addr, *storageKey, block0)
			require.NoError(t, err)
			assert.Equal(t, felt.Zero, gotStorage)
		})

		t.Run("deployed", func(t *testing.T) {
			stateDB := newTestStateDB(dbType)
			state, err := New(felt.Zero, stateDB)
			require.NoError(t, err)
			require.NoError(t, state.Update(block0, su, nil))

			gotNonce, err := state.ContractNonceAt(*addr, block0)
			require.NoError(t, err)
			assert.Equal(t, *nonce, gotNonce)

			gotClassHash, err := state.ContractClassHashAt(*addr, block0)
			require.NoError(t, err)
			assert.Equal(t, *classHash, gotClassHash)

			gotStorage, err := state.ContractStorageAt(*addr, *storageKey, block0)
			require.NoError(t, err)
			assert.Equal(t, *storageValue, gotStorage)
		})

		t.Run("deployed and updated", func(t *testing.T) {
			stateDB := newTestStateDB(dbType)
			state, err := New(felt.Zero, stateDB)
			require.NoError(t, err)
			require.NoError(t, state.Update(block0, su, nil))

			newNonce := new(felt.Felt).SetBytes([]byte("new_nonce"))
			newStorageValue := new(felt.Felt).SetBytes([]byte("new_storage_value"))

			su2 := &core.StateUpdate{
				OldRoot: su.NewRoot,
				NewRoot: utils.HexToFelt(t, "0x55075b726402e12fa85ad5a063774764b5f3f119053d4d72e1ef3986063bee6"),
				StateDiff: &core.StateDiff{
					Nonces:       map[felt.Felt]*felt.Felt{*addr: newNonce},
					StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{*addr: {*storageKey: newStorageValue}},
				},
			}

			require.NoError(t, state.Update(block1, su2, nil))

			gotNonce, err := state.ContractNonceAt(*addr, block1)
			require.NoError(t, err)
			assert.Equal(t, *newNonce, gotNonce)

			gotClassHash, err := state.ContractClassHashAt(*addr, block1)
			require.NoError(t, err)
			assert.Equal(t, *classHash, gotClassHash)

			gotStorage, err := state.ContractStorageAt(*addr, *storageKey, block1)
			require.NoError(t, err)
			assert.Equal(t, *newStorageValue, gotStorage)
		})
	})
}

func BenchmarkStateUpdate(b *testing.B) {
	runWithDBTypesBench(b, func(b *testing.B, dbType triedb.Scheme) {
		client := feeder.NewTestClient(b, &utils.Mainnet)
		gw := adaptfeeder.New(client)

		su0, err := gw.StateUpdate(b.Context(), block0)
		require.NoError(b, err)

		su1, err := gw.StateUpdate(b.Context(), block1)
		require.NoError(b, err)

		su2, err := gw.StateUpdate(b.Context(), block2)
		require.NoError(b, err)

		stateUpdates := []*core.StateUpdate{su0, su1, su2}

		for b.Loop() {
			stateDB := newTestStateDB(dbType)
			for i, su := range stateUpdates {
				state, err := New(*su.OldRoot, stateDB)
				require.NoError(b, err)
				err = state.Update(uint64(i), su, nil)
				if err != nil {
					b.Fatalf("Error updating state: %v", err)
				}
			}
		}
	})
}

// Get the first 3 state updates from the mainnet.
func getStateUpdates(t *testing.T) []*core.StateUpdate {
	client := feeder.NewTestClient(t, &utils.Mainnet)
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
func setupState(t *testing.T, stateUpdates []*core.StateUpdate, blocks uint64, dbType triedb.Scheme) *StateDB {
	stateDB := newTestStateDB(dbType)
	for i, su := range stateUpdates[:blocks] {
		state, err := New(*su.OldRoot, stateDB)
		require.NoError(t, err)
		var declaredClasses map[felt.Felt]core.Class
		if i == 3 {
			declaredClasses = su3DeclaredClasses()
		}
		require.NoError(t, state.Update(uint64(i), su, declaredClasses), "failed to update state for block %d", i)
		newComm, err := state.Commitment()
		require.NoError(t, err)
		assert.Equal(t, *su.NewRoot, newComm)
	}

	return stateDB
}

func runWithDBTypes(t *testing.T, testFn func(t *testing.T, dbType triedb.Scheme)) {
	t.Helper()

	for _, dbType := range []triedb.Scheme{triedb.PathDB, triedb.HashDB} {
		t.Run(string(dbType), func(t *testing.T) {
			testFn(t, dbType)
		})
	}
}

func runWithDBTypesBench(b *testing.B, benchFn func(b *testing.B, dbType triedb.Scheme)) {
	b.Helper()

	for _, dbType := range []triedb.Scheme{triedb.PathDB, triedb.HashDB} {
		b.Run(string(dbType), func(b *testing.B) {
			benchFn(b, dbType)
		})
	}
}

func newTestStateDB(dbType triedb.Scheme) *StateDB {
	memDB := memory.New()
	db, err := triedb.New(memDB, nil)
	if err != nil {
		panic(err)
	}
	return NewStateDB(memDB, db)
}
