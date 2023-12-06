package blockchain_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPendingState(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	deployedAddr, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	deployedAddr2, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	deployedClassHash, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	replacedAddr, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	replacedClassHash, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	declaredV1ClassHash, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	declaredV1CompiledClassHash, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	pending := blockchain.Pending{
		Block: nil,
		StateUpdate: &core.StateUpdate{
			BlockHash: nil,
			NewRoot:   nil,
			OldRoot:   nil,
			StateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Felt]*felt.Felt{
					*deployedAddr:  deployedClassHash,
					*deployedAddr2: deployedClassHash,
				},
				ReplacedClasses: map[felt.Felt]*felt.Felt{
					*replacedAddr: replacedClassHash,
				},
				Nonces: map[felt.Felt]*felt.Felt{
					*deployedAddr: new(felt.Felt).SetUint64(44),
				},
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
					*deployedAddr: {
						*new(felt.Felt).SetUint64(44): new(felt.Felt).SetUint64(37),
					},
				},
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{
					*declaredV1ClassHash: declaredV1CompiledClassHash,
				},
			},
		},
		NewClasses: map[felt.Felt]core.Class{
			*deployedClassHash: &core.Cairo0Class{},
		},
	}
	state := blockchain.NewPendingState(pending.StateUpdate.StateDiff, pending.NewClasses, mockState)

	t.Run("ContractClassHash", func(t *testing.T) {
		t.Run("from pending", func(t *testing.T) {
			t.Run("deployed", func(t *testing.T) {
				cH, cErr := state.ContractClassHash(deployedAddr)
				require.NoError(t, cErr)
				assert.Equal(t, deployedClassHash, cH)

				cH, cErr = state.ContractClassHash(deployedAddr2)
				require.NoError(t, cErr)
				assert.Equal(t, deployedClassHash, cH)
			})
			t.Run("replaced", func(t *testing.T) {
				cH, cErr := state.ContractClassHash(replacedAddr)
				require.NoError(t, cErr)
				assert.Equal(t, replacedClassHash, cH)
			})
		})
		t.Run("from head", func(t *testing.T) {
			expectedClassHash := new(felt.Felt).SetUint64(37)
			mockState.EXPECT().ContractClassHash(gomock.Any()).Return(expectedClassHash, nil)

			cH, cErr := state.ContractClassHash(&felt.Zero)
			require.NoError(t, cErr)
			assert.Equal(t, expectedClassHash, cH)
		})
	})
	t.Run("ContractNonce", func(t *testing.T) {
		t.Run("from pending", func(t *testing.T) {
			cN, cErr := state.ContractNonce(deployedAddr)
			require.NoError(t, cErr)
			assert.Equal(t, new(felt.Felt).SetUint64(44), cN)

			cN, cErr = state.ContractNonce(deployedAddr2)
			require.NoError(t, cErr)
			assert.Equal(t, &felt.Zero, cN)
		})
		t.Run("from head", func(t *testing.T) {
			expectedNonce := new(felt.Felt).SetUint64(1337)
			mockState.EXPECT().ContractNonce(gomock.Any()).Return(expectedNonce, nil)

			cN, cErr := state.ContractNonce(&felt.Zero)
			require.NoError(t, cErr)
			assert.Equal(t, expectedNonce, cN)
		})
	})
	t.Run("ContractStorage", func(t *testing.T) {
		t.Run("from pending", func(t *testing.T) {
			expectedValue := new(felt.Felt).SetUint64(37)
			cV, cErr := state.ContractStorage(deployedAddr, new(felt.Felt).SetUint64(44))
			require.NoError(t, cErr)
			assert.Equal(t, expectedValue, cV)

			cV, cErr = state.ContractStorage(deployedAddr, new(felt.Felt).SetUint64(0xDEADBEEF))
			require.NoError(t, cErr)
			assert.Equal(t, &felt.Zero, cV)

			cV, cErr = state.ContractStorage(deployedAddr2, new(felt.Felt).SetUint64(0xDEADBEEF))
			require.NoError(t, cErr)
			assert.Equal(t, &felt.Zero, cV)
		})
		t.Run("from head", func(t *testing.T) {
			expectedValue := new(felt.Felt).SetUint64(0xDEADBEEF)
			mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedValue, nil)

			cV, cErr := state.ContractStorage(&felt.Zero, &felt.Zero)
			require.NoError(t, cErr)
			assert.Equal(t, expectedValue, cV)
		})
	})
	t.Run("Class", func(t *testing.T) {
		t.Run("from pending", func(t *testing.T) {
			pC, pErr := state.Class(deployedClassHash)
			require.NoError(t, pErr)
			_, ok := pC.Class.(*core.Cairo0Class)
			assert.True(t, ok)
		})
		t.Run("from head", func(t *testing.T) {
			mockState.EXPECT().Class(gomock.Any()).Return(&core.DeclaredClass{
				Class: &core.Cairo1Class{},
			}, nil)
			pC, pErr := state.Class(&felt.Zero)
			require.NoError(t, pErr)
			_, ok := pC.Class.(*core.Cairo1Class)
			assert.True(t, ok)
		})
	})
}

// "Write" functions are tested below.
// The variables below are at package scope for convenience.
// All tests are careful not to modify them.

var (
	contractAddr      = new(felt.Felt).SetUint64(1)
	classHash         = new(felt.Felt).SetUint64(2)
	compiledClassHash = new(felt.Felt).SetUint64(3)
	storageKey        = felt.Felt{}
	testState         = &core.StateDiff{
		StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
			*contractAddr: {storageKey: new(felt.Felt).SetUint64(1)},
		},
		Nonces: map[felt.Felt]*felt.Felt{
			*contractAddr: new(felt.Felt).SetUint64(2),
		},
		DeployedContracts: map[felt.Felt]*felt.Felt{*contractAddr: classHash},
		DeclaredV0Classes: []*felt.Felt{},
		DeclaredV1Classes: map[felt.Felt]*felt.Felt{*classHash: compiledClassHash},
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt),
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

func makeState(t *testing.T) *blockchain.PendingState {
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
	return blockchain.NewPendingState(core.EmptyStateDiff(), map[felt.Felt]core.Class{*contractAddr: testClass}, state)
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
	kv := testState.StorageDiffs[*contractAddr]
	newValue := new(felt.Felt).Add(kv[storageKey], new(felt.Felt).SetUint64(1))
	require.NoError(t, state.SetStorage(contractAddr, &storageKey, newValue))

	got, err := state.ContractStorage(contractAddr, &storageKey)
	require.NoError(t, err)
	require.Equal(t, newValue, got)
}

func TestSetNewStorageValue(t *testing.T) {
	state := makeState(t)
	storageKey := new(felt.Felt).SetUint64(42)
	storageValue := new(felt.Felt).SetUint64(43)
	require.NoError(t, state.SetStorage(contractAddr, storageKey, storageValue))

	got, err := state.ContractStorage(contractAddr, storageKey)
	require.NoError(t, err)
	require.Equal(t, storageValue, got)
}

func TestSetClassHashTwiceForNewContract(t *testing.T) {
	state := makeState(t)
	addr := new(felt.Felt).SetUint64(42)
	classHash := new(felt.Felt).SetUint64(43)
	require.NoError(t, state.SetClassHash(addr, classHash))

	gotClassHash, err := state.ContractClassHash(addr)
	require.NoError(t, err)
	require.Equal(t, classHash, gotClassHash)

	otherClassHash := new(felt.Felt).SetUint64(44)
	require.NoError(t, state.SetClassHash(addr, otherClassHash))
	gotNewClassHash, err := state.ContractClassHash(addr)
	require.NoError(t, err)
	require.Equal(t, otherClassHash, gotNewClassHash)
}

func TestSetClassHashForExistingContract(t *testing.T) {
	state := makeState(t)
	otherClassHash := new(felt.Felt).SetUint64(44)
	require.NoError(t, state.SetClassHash(contractAddr, otherClassHash))
	gotNewClassHash, err := state.ContractClassHash(contractAddr)
	require.NoError(t, err)
	require.Equal(t, otherClassHash, gotNewClassHash)
}

func TestSetCompiledClassHash(t *testing.T) {
	state := makeState(t)
	classHash := new(felt.Felt).SetUint64(42)
	compiledClassHash := new(felt.Felt).SetUint64(43)
	require.NoError(t, state.SetCompiledClassHash(classHash, compiledClassHash))
}
