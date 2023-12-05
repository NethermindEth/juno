package builder_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

var (
	contractAddr      = new(felt.Felt).SetUint64(1)
	classHash         = new(felt.Felt).SetUint64(2)
	compiledClassHash = new(felt.Felt).SetUint64(3)
	testState         = &core.StateDiff{
		StorageDiffs: map[felt.Felt][]core.StorageDiff{
			*contractAddr: {{Key: new(felt.Felt), Value: new(felt.Felt).SetUint64(1)}},
		},
		Nonces: map[felt.Felt]*felt.Felt{
			*contractAddr: new(felt.Felt).SetUint64(2),
		},
		DeployedContracts: []core.AddressClassHashPair{{Address: contractAddr, ClassHash: classHash}},
		DeclaredV0Classes: []*felt.Felt{},
		DeclaredV1Classes: []core.DeclaredV1Class{{ClassHash: classHash, CompiledClassHash: compiledClassHash}},
		ReplacedClasses:   []core.AddressClassHashPair{},
	}
	testClass = &core.Cairo1Class{
		AbiHash: new(felt.Felt),
		EntryPoints: struct {
			Constructor []core.SierraEntryPoint
			External    []core.SierraEntryPoint
			L1Handler   []core.SierraEntryPoint
		}{
			Constructor: []core.SierraEntryPoint{},
			External:    []core.SierraEntryPoint{},
			L1Handler:   []core.SierraEntryPoint{},
		},
		Program:     []*felt.Felt{},
		ProgramHash: &felt.Felt{},
		Compiled:    json.RawMessage{},
	}
)

func makeState(t *testing.T) *builder.State {
	testDB := pebble.NewMemTest(t)
	pebbleTxn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pebbleTxn.Discard())
	})
	state := core.NewState(pebbleTxn)
	blockNumber := uint64(0)
	require.NoError(t, state.Update(blockNumber, &core.StateUpdate{
		BlockHash: new(felt.Felt),
		NewRoot:   utils.HexToFelt(t, "0x4b2d2338a1f3bd507a2263358fe84eef616528bfff633827ba5b7579b464c49"),
		OldRoot:   new(felt.Felt),
		StateDiff: testState,
	}, map[felt.Felt]core.Class{
		*classHash: testClass,
	}))
	return builder.NewState(state, blockNumber+1)
}

func TestStateNoUpdates(t *testing.T) {
	state := makeState(t)

	classHash, err := state.ContractClassHash(contractAddr)
	require.NoError(t, err)
	require.Equal(t, testState.DeployedContracts[0].ClassHash, classHash)

	nonce, err := state.ContractNonce(contractAddr)
	require.NoError(t, err)
	require.Equal(t, testState.Nonces[*contractAddr], nonce)

	kv := testState.StorageDiffs[*contractAddr][0]
	value, err = state.ContractStorage(contractAddr, kv.Key)
	require.NoError(t, err)
	require.Equal(t, kv.Value, value)

	class, err := state.Class(classHash)
	require.NoError(t, err)
	require.Equal(t, &core.DeclaredClass{
		Class: testClass,
		At:    0,
	}, class)
}

func TestIncrementNonce(t *testing.T) {
	state := makeState(t)
	require.NoError(t, state.IncrementNonce(contractAddr))

	got, err := state.ContractNonce(contractAddr)
	require.NoError(t, err)
	want := new(felt.Felt).Add(testState.Nonces[*contractAddr], new(felt.Felt).SetUint64(1))
	require.Equal(t, want, got)
}

func TestUpdateExistingStorageValue(t *testing.T) {
	state := makeState(t)
	kv := testState.StorageDiffs[*contractAddr][0]
	newValue := new(felt.Felt).Add(kv.Value, new(felt.Felt).SetUint64(1))
	state.SetStorage(contractAddr, kv.Key, newValue)

	got, err := state.ContractStorage(contractAddr, kv.Key)
	require.NoError(t, err)
	require.Equal(t, newValue, got)
}

func TestSetNewStorageValue(t *testing.T) {
	state := makeState(t)
	storageKey := new(felt.Felt).SetUint64(42)
	storageValue := new(felt.Felt).SetUint64(43)
	state.SetStorage(contractAddr, storageKey, storageValue)

	got, err := state.ContractStorage(contractAddr, storageKey)
	require.NoError(t, err)
	require.Equal(t, storageValue, got)
}

func TestDeployAndReplaceNewContract(t *testing.T) {
	state := makeState(t)
	addr := new(felt.Felt).SetUint64(42)
	classHash := new(felt.Felt).SetUint64(43)
	state.DeployContract(addr, classHash)

	gotNonce, err := state.ContractNonce(addr)
	require.NoError(t, err)
	require.Equal(t, new(felt.Felt), gotNonce)

	gotClassHash, err := state.ContractClassHash(addr)
	require.NoError(t, err)
	require.Equal(t, classHash, gotClassHash)

	otherClassHash := new(felt.Felt).SetUint64(44)
	state.ReplaceContractClass(addr, otherClassHash)
	gotNewClassHash, err := state.ContractClassHash(addr)
	require.NoError(t, err)
	require.Equal(t, otherClassHash, gotNewClassHash)
}

func TestReplaceExistingContractClass(t *testing.T) {
	state := makeState(t)
	otherClassHash := new(felt.Felt).SetUint64(44)
	state.ReplaceContractClass(contractAddr, otherClassHash)
	gotNewClassHash, err := state.ContractClassHash(contractAddr)
	require.NoError(t, err)
	require.Equal(t, otherClassHash, gotNewClassHash)
}

func TestDeclareClass(t *testing.T) {
	state := makeState(t)
	classHash := new(felt.Felt).SetUint64(42)
	compiledClassHash := new(felt.Felt).SetUint64(43)
	state.DeclareClass(classHash, compiledClassHash, testClass)

	class, err := state.Class(classHash)
	require.NoError(t, err)
	require.Equal(t, &core.DeclaredClass{
		Class: testClass,
		At:    1,
	}, class)
}

func TestDiff(t *testing.T) {
	state := makeState(t)

	state.DeclareClass(newClassHash, newCompiledClassHash, testClass)
	state.DeployContract(newContractAddr, newClassHash)
	state.ReplaceContractClass(contractAddr, newClassHash)
	require.NoError(t, state.IncrementNonce(contractAddr))
	state.SetStorage(newContractAddr, key, value)

	gotDiff := state.Diff()
	require.Equal(t, &builderDiff, gotDiff)
}

func TestUpdate(t *testing.T) {
	state := makeState(t)
	newContractAddr := new(felt.Felt).SetUint64(100)
	newClassHash := new(felt.Felt).SetUint64(101)
	newCompiledClassHash := new(felt.Felt).SetUint64(102)
	key := new(felt.Felt)
	value := new(felt.Felt).SetUint64(1)

	state.DeclareClass(newClassHash, newCompiledClassHash, testClass)
	state.DeployContract(newContractAddr, newClassHash)
	state.ReplaceContractClass(contractAddr, newClassHash)
	require.NoError(t, state.IncrementNonce(contractAddr))
	state.SetStorage(newContractAddr, key, value)

	other := makeState(t)
	other.Update(&builderDiff)

	require.Equal(t, other.Diff(), state.Diff())
}
