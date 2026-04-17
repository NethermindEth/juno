package state

import (
	"maps"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStateReader(t *testing.T) {
	t.Run("returns read-only state at root", func(t *testing.T) {
		stateDB := newTestStateDB()
		st, err := NewStateReader(&felt.Zero, stateDB)
		require.NoError(t, err)
		assert.NotNil(t, st)
	})

	t.Run("state commitment readable", func(t *testing.T) {
		stateDB := newTestStateDB()
		st, err := NewStateReader(&felt.Zero, stateDB)
		require.NoError(t, err)

		_, err = st.Commitment("")
		require.NoError(t, err)
	})
}

func TestContractClassHash(t *testing.T) {
	stateUpdates := getStateUpdates(t)
	stateUpdates = stateUpdates[:2]

	su0 := stateUpdates[0]
	su1 := stateUpdates[1]

	stateDB := setupState(t, stateUpdates, 2)
	state, err := NewStateReader(su1.NewRoot, stateDB)
	require.NoError(t, err)

	allDeployedContracts := make(map[felt.Felt]*felt.Felt)
	maps.Copy(allDeployedContracts, su0.StateDiff.DeployedContracts)
	maps.Copy(allDeployedContracts, su1.StateDiff.DeployedContracts)

	for addr, expected := range allDeployedContracts {
		gotClassHash, err := state.ContractClassHash(&addr)
		require.NoError(t, err)
		assert.Equal(t, *expected, gotClassHash)
	}
}

func TestNonce(t *testing.T) {
	addr := felt.UnsafeFromString[felt.Felt](
		"0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
	)
	root := felt.NewUnsafeFromString[felt.Felt](
		"0x4bdef7bf8b81a868aeab4b48ef952415fe105ab479e2f7bc671c92173542368",
	)

	su0 := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: root,
		StateDiff: &core.StateDiff{
			DeployedContracts: map[felt.Felt]*felt.Felt{
				addr: felt.NewUnsafeFromString[felt.Felt](
					"0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
				),
			},
		},
	}

	t.Run("newly deployed contract has zero nonce", func(t *testing.T) {
		stateDB := setupState(t, nil, 0)
		batch := stateDB.disk.NewBatch()
		state, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block0}, su0, nil, false))
		require.NoError(t, batch.Write())

		reader, err := NewStateReader(su0.NewRoot, stateDB)
		require.NoError(t, err)
		gotNonce, err := reader.ContractNonce(&addr)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, gotNonce)
	})

	t.Run("update contract nonce", func(t *testing.T) {
		stateDB := setupState(t, nil, 0)
		batch := stateDB.disk.NewBatch()
		state, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)
		require.NoError(t, state.Update(&core.Header{Number: block0}, su0, nil, false))
		require.NoError(t, batch.Write())

		su1 := &core.StateUpdate{
			NewRoot: felt.NewUnsafeFromString[felt.Felt](
				"0x6210642ffd49f64617fc9e5c0bbe53a6a92769e2996eb312a42d2bdb7f2afc1",
			),
			OldRoot: root,
			StateDiff: &core.StateDiff{
				Nonces: map[felt.Felt]*felt.Felt{addr: &felt.One},
			},
		}

		batch1 := stateDB.disk.NewBatch()
		state1, err := New(su1.OldRoot, stateDB, batch1)
		require.NoError(t, err)
		require.NoError(t, state1.Update(&core.Header{Number: block1}, su1, nil, false))
		require.NoError(t, batch1.Write())

		reader, err := NewStateReader(su1.NewRoot, stateDB)
		require.NoError(t, err)
		gotNonce, err := reader.ContractNonce(&addr)
		require.NoError(t, err)
		assert.Equal(t, felt.One, gotNonce)
	})
}

func TestClass(t *testing.T) {
	stateDB := setupState(t, nil, 0)

	client := feeder.NewTestClient(t, &networks.Integration)
	gw := adaptfeeder.New(client)

	deprecatedCairoHash := felt.NewUnsafeFromString[felt.Felt](
		"0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04",
	)
	deprecatedCairoClass, err := gw.Class(t.Context(), deprecatedCairoHash)
	require.NoError(t, err)
	sierraHash := felt.NewUnsafeFromString[felt.Felt](
		"0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5",
	)
	sierraClass, err := gw.Class(t.Context(), sierraHash)
	require.NoError(t, err)

	batch := stateDB.disk.NewBatch()
	state, err := New(&felt.Zero, stateDB, batch)
	require.NoError(t, err)

	su0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)
	require.NoError(t, state.Update(&core.Header{Number: 0}, su0, map[felt.Felt]core.ClassDefinition{
		*deprecatedCairoHash: deprecatedCairoClass,
		*sierraHash:          sierraClass,
	}, false))
	require.NoError(t, batch.Write())

	reader, err := NewStateReader(su0.NewRoot, stateDB)
	require.NoError(t, err)

	gotSierraClass, err := reader.Class(sierraHash)
	require.NoError(t, err)
	assert.Zero(t, gotSierraClass.At)
	assert.Equal(t, sierraClass, gotSierraClass.Class)

	gotDeprecatedCairoClass, err := reader.Class(deprecatedCairoHash)
	require.NoError(t, err)
	assert.Zero(t, gotDeprecatedCairoClass.At)
	assert.Equal(t, deprecatedCairoClass, gotDeprecatedCairoClass.Class)
}

func TestStateTries(t *testing.T) {
	stateUpdates := getStateUpdates(t)
	stateDB := setupState(t, stateUpdates, 1)
	root := *stateUpdates[0].NewRoot

	state, err := NewStateReader(&root, stateDB)
	require.NoError(t, err)

	classTrie, err := state.ClassTrie()
	require.NoError(t, err)
	require.NotNil(t, classTrie)

	contractTrie, err := state.ContractTrie()
	require.NoError(t, err)
	require.NotNil(t, contractTrie)

	// ContractStorageTrie for a deployed contract (first deployed in mainnet block 1)
	addr := &su1FirstDeployedAddress
	storageTrie, err := state.ContractStorageTrie(addr)
	require.NoError(t, err)
	require.NotNil(t, storageTrie)
}

func TestContractDeployedAt(t *testing.T) {
	stateUpdates := getStateUpdates(t)
	stateDB := setupState(t, stateUpdates, 2)
	root := *stateUpdates[1].NewRoot

	t.Run("deployed on genesis", func(t *testing.T) {
		state, err := NewStateReader(&root, stateDB)
		require.NoError(t, err)

		d0 := felt.NewUnsafeFromString[felt.Felt](
			"0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
		)
		deployed, err := state.ContractDeployedAt(d0, block0)
		require.NoError(t, err)
		assert.True(t, deployed)

		deployed, err = state.ContractDeployedAt(d0, block1)
		require.NoError(t, err)
		assert.True(t, deployed)
	})

	t.Run("deployed after genesis", func(t *testing.T) {
		state, err := NewStateReader(&root, stateDB)
		require.NoError(t, err)

		d1 := felt.NewUnsafeFromString[felt.Felt](
			"0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
		)
		deployed, err := state.ContractDeployedAt(d1, block0)
		require.NoError(t, err)
		assert.True(t, deployed)

		deployed, err = state.ContractDeployedAt(d1, block1)
		require.NoError(t, err)
		assert.True(t, deployed)
	})

	t.Run("not deployed", func(t *testing.T) {
		state, err := NewStateReader(&root, stateDB)
		require.NoError(t, err)

		notDeployed := felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")
		deployed, err := state.ContractDeployedAt(notDeployed, block0)
		require.NoError(t, err)
		assert.False(t, deployed)
	})
}

func TestContractHistory(t *testing.T) {
	addr := felt.UnsafeFromString[felt.Felt]("0x1234567890abcdef")
	classHash := felt.NewUnsafeFromString[felt.Felt]("0xc1a55")
	nonce := felt.NewUnsafeFromString[felt.Felt]("0xace")
	storageKey := felt.NewUnsafeFromString[felt.Felt]("0x5107e")
	storageValue := felt.NewUnsafeFromString[felt.Felt]("0x5a1")

	emptyStateUpdate := &core.StateUpdate{
		OldRoot:   &felt.Zero,
		NewRoot:   &felt.Zero,
		StateDiff: &core.StateDiff{},
	}

	su := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: felt.NewUnsafeFromString[felt.Felt](
			"0x164c1b480d2a20afe107d1cc193a78128452d5205d1721f9a7aee35babca845",
		),
		StateDiff: &core.StateDiff{
			DeployedContracts: map[felt.Felt]*felt.Felt{addr: classHash},
			Nonces:            map[felt.Felt]*felt.Felt{addr: nonce},
			StorageDiffs:      map[felt.Felt]map[felt.Felt]*felt.Felt{addr: {*storageKey: storageValue}},
		},
	}

	t.Run("empty", func(t *testing.T) {
		stateDB := newTestStateDB()
		state, err := NewStateReader(&felt.Zero, stateDB)
		require.NoError(t, err)

		nonce, err := state.ContractNonceAt(&addr, block0)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, nonce)

		classHash, err := state.ContractClassHashAt(&addr, block0)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, classHash)

		storage, err := state.ContractStorageAt(&addr, storageKey, block0)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, storage)
	})

	t.Run("retrieve block height is the same as update", func(t *testing.T) {
		stateDB := newTestStateDB()
		batch := stateDB.disk.NewBatch()
		state, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)

		su0 := &core.StateUpdate{
			OldRoot: &felt.Zero,
			NewRoot: felt.NewUnsafeFromString[felt.Felt](
				"0x164c1b480d2a20afe107d1cc193a78128452d5205d1721f9a7aee35babca845",
			),
			StateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Felt]*felt.Felt{addr: classHash},
				Nonces:            map[felt.Felt]*felt.Felt{addr: nonce},
				StorageDiffs:      map[felt.Felt]map[felt.Felt]*felt.Felt{addr: {*storageKey: storageValue}},
			},
		}

		require.NoError(t, state.Update(&core.Header{Number: block0}, su0, nil, false))
		require.NoError(t, batch.Write())

		reader, err := NewStateReader(su0.NewRoot, stateDB)
		require.NoError(t, err)

		gotNonce, err := reader.ContractNonceAt(&addr, block0)
		require.NoError(t, err)
		assert.Equal(t, *nonce, gotNonce)

		gotClassHash, err := reader.ContractClassHashAt(&addr, block0)
		require.NoError(t, err)
		assert.Equal(t, *classHash, gotClassHash)

		gotStorage, err := reader.ContractStorageAt(&addr, storageKey, block0)
		require.NoError(t, err)
		assert.Equal(t, *storageValue, gotStorage)
	})

	t.Run("retrieve block height before update", func(t *testing.T) {
		stateDB := newTestStateDB()
		batch := stateDB.disk.NewBatch()
		state0, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)
		su0 := emptyStateUpdate
		require.NoError(t, state0.Update(&core.Header{Number: block0}, su0, nil, false))
		require.NoError(t, batch.Write())

		batch = stateDB.disk.NewBatch()
		state1, err := New(su0.NewRoot, stateDB, batch)
		require.NoError(t, err)
		su1 := su
		require.NoError(t, state1.Update(&core.Header{Number: block1}, su1, nil, false))
		require.NoError(t, batch.Write())

		reader, err := NewStateReader(su1.NewRoot, stateDB)
		require.NoError(t, err)

		gotNonce, err := reader.ContractNonceAt(&addr, block0)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, gotNonce)

		gotClassHash, err := reader.ContractClassHashAt(&addr, block0)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, gotClassHash)

		gotStorage, err := reader.ContractStorageAt(&addr, storageKey, block0)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, gotStorage)
	})

	t.Run("retrieve block height in between updates", func(t *testing.T) {
		stateDB := newTestStateDB()
		batch := stateDB.disk.NewBatch()
		state0, err := New(&felt.Zero, stateDB, batch)
		require.NoError(t, err)
		su0 := su
		require.NoError(t, state0.Update(&core.Header{Number: block0}, su0, nil, false))
		require.NoError(t, batch.Write())

		batch = stateDB.disk.NewBatch()
		state1, err := New(su0.NewRoot, stateDB, batch)
		require.NoError(t, err)
		su1 := &core.StateUpdate{
			OldRoot:   su0.NewRoot,
			NewRoot:   su0.NewRoot,
			StateDiff: &core.StateDiff{},
		}
		require.NoError(t, state1.Update(&core.Header{Number: block1}, su1, nil, false))
		require.NoError(t, batch.Write())

		batch = stateDB.disk.NewBatch()
		state2, err := New(su1.NewRoot, stateDB, batch)
		require.NoError(t, err)
		su2 := &core.StateUpdate{
			OldRoot: su1.NewRoot,
			NewRoot: felt.NewUnsafeFromString[felt.Felt](
				"0x719e4c1206c30bb8e2924c821910582045c2acd0be342ad4ec9037d15f955be",
			),
			StateDiff: &core.StateDiff{
				Nonces: map[felt.Felt]*felt.Felt{addr: felt.NewUnsafeFromString[felt.Felt]("0xaced")},
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
					addr: {*storageKey: felt.NewUnsafeFromString[felt.Felt]("0x5a1e")},
				},
				DeployedContracts: map[felt.Felt]*felt.Felt{
					*classHash: felt.NewUnsafeFromString[felt.Felt]("0xc1a5e"),
				},
			},
		}
		require.NoError(t, state2.Update(&core.Header{Number: block2}, su2, nil, false))
		require.NoError(t, batch.Write())

		reader, err := NewStateReader(su2.NewRoot, stateDB)
		require.NoError(t, err)

		gotNonce, err := reader.ContractNonceAt(&addr, block1)
		require.NoError(t, err)
		assert.Equal(t, *nonce, gotNonce)

		gotClassHash, err := reader.ContractClassHashAt(&addr, block1)
		require.NoError(t, err)
		assert.Equal(t, *classHash, gotClassHash)

		gotStorage, err := reader.ContractStorageAt(&addr, storageKey, block1)
		require.NoError(t, err)
		assert.Equal(t, *storageValue, gotStorage)
	})
}

func TestContractStorageLastUpdatedBlock(t *testing.T) {
	addr := felt.FromUint64[felt.Address](1)
	addrFelt := felt.Felt(addr)
	key := felt.NewFromUint64[felt.Felt](10)
	value := felt.NewFromUint64[felt.Felt](99)

	stateDB := newTestStateDB()
	batch := stateDB.disk.NewBatch()
	state, err := New(&felt.Zero, stateDB, batch)
	require.NoError(t, err)

	t.Run("storage never updated returns not found", func(t *testing.T) {
		blockNum, err := state.ContractStorageLastUpdatedBlock(&addr, key)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), blockNum)
	})

	su := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				addrFelt: {*key: value},
			},
		},
	}
	require.NoError(t, state.Update(&core.Header{Number: block0}, su, nil, true))
	require.NoError(t, batch.Write())

	t.Run("storage updated at block 0 returns block 0", func(t *testing.T) {
		blockNum, err := state.ContractStorageLastUpdatedBlock(&addr, key)
		require.NoError(t, err)
		assert.Equal(t, uint64(block0), blockNum)
	})

	// update the key at block 1
	root0, err := state.Commitment("")
	require.NoError(t, err)
	batch = stateDB.disk.NewBatch()
	state, err = New(&root0, stateDB, batch)
	require.NoError(t, err)
	su1 := &core.StateUpdate{
		OldRoot: &root0,
		NewRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				addrFelt: {*key: value},
			},
		},
	}
	require.NoError(t, state.Update(&core.Header{Number: block1}, su1, nil, true))
	require.NoError(t, batch.Write())

	// two unrelated updates: block 2 and 3
	root1, err := state.Commitment("")
	require.NoError(t, err)
	batch = stateDB.disk.NewBatch()
	state, err = New(&root1, stateDB, batch)
	require.NoError(t, err)
	su2 := &core.StateUpdate{
		OldRoot: &root0,
		NewRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				addrFelt: {*felt.NewRandom[felt.Felt](): value}, // unrelated key
			},
		},
	}
	require.NoError(t, state.Update(&core.Header{Number: block2}, su2, nil, true))
	require.NoError(t, batch.Write())

	root2, err := state.Commitment("")
	require.NoError(t, err)
	batch = stateDB.disk.NewBatch()
	state, err = New(&root2, stateDB, batch)
	require.NoError(t, err)
	su3 := &core.StateUpdate{
		OldRoot: &root2,
		NewRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				addrFelt: {*felt.NewRandom[felt.Felt](): value}, // unrelated key
			},
		},
	}
	require.NoError(t, state.Update(&core.Header{Number: block3}, su3, nil, true))
	require.NoError(t, batch.Write())

	t.Run("returns latest updated block", func(t *testing.T) {
		blockNum, err := state.ContractStorageLastUpdatedBlock(&addr, key)
		require.NoError(t, err)
		assert.Equal(t, uint64(block1), blockNum)
	})

	t.Run("unrelated key returns not found", func(t *testing.T) {
		blockNum, err := state.ContractStorageLastUpdatedBlock(
			&addr, felt.NewFromUint64[felt.Felt](999),
		)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), blockNum)
	})
}

func TestStateCommitmentPre014ReturnsContractRoot(t *testing.T) {
	// Pre-0.14 with no declared classes (classRoot == 0): commitment must equal
	// the raw contractRoot, not a Poseidon hash.
	stateDB := newTestStateDB()
	batch := stateDB.disk.NewBatch()
	state, err := New(&felt.Zero, stateDB, batch)
	require.NoError(t, err)

	storageKey := felt.NewFromUint64[felt.Felt](0)
	storageVal := felt.NewFromUint64[felt.Felt](0x80)
	contractAddr := felt.NewFromUint64[felt.Felt](2)

	su := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: &felt.Zero, // skip root verification
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*contractAddr: {*storageKey: storageVal},
			},
		},
	}

	require.NoError(t, state.Update(&core.Header{Number: 0, ProtocolVersion: "0.13.0"}, su, nil, true))

	// Pre-0.14: classRoot is zero, so commitment == contractRoot (no Poseidon)
	pre014, err := state.Commitment("0.13.0")
	require.NoError(t, err)
	assert.False(t, pre014.IsZero())

	// v0.14+: same state but Poseidon is always applied
	v014, err := state.Commitment("0.14.2")
	require.NoError(t, err)

	assert.NotEqual(t,
		pre014,
		v014,
		"pre-0.14 commitment (raw contractRoot) must differ from v0.14 (Poseidon)",
	)
}
