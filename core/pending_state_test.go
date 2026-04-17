package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPendingState(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockState := mocks.NewMockStateReader(mockCtrl)

	deployedAddr := felt.NewRandom[felt.Felt]()
	deployedAddr2 := felt.NewRandom[felt.Felt]()
	deployedClassHash := felt.NewRandom[felt.Felt]()
	replacedAddr := felt.NewRandom[felt.Felt]()
	replacedClassHash := felt.NewRandom[felt.Felt]()
	existingContractAddr := felt.NewRandom[felt.Address]()

	key := felt.NewFromUint64[felt.Felt](44)
	value := felt.NewFromUint64[felt.Felt](37)

	stateDiff := &core.StateDiff{
		DeployedContracts: map[felt.Felt]*felt.Felt{
			*deployedAddr:  deployedClassHash,
			*deployedAddr2: deployedClassHash,
		},
		ReplacedClasses: map[felt.Felt]*felt.Felt{
			*replacedAddr: replacedClassHash,
		},
		Nonces: map[felt.Felt]*felt.Felt{
			*deployedAddr: felt.NewFromUint64[felt.Felt](44),
		},
		StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
			*deployedAddr: {
				*key: value,
			},
			felt.Felt(*existingContractAddr): {
				*key: value,
			},
		},
	}

	newClasses := map[felt.Felt]core.ClassDefinition{
		*deployedClassHash: &core.DeprecatedCairoClass{},
	}

	const pendingBlockNumber = uint64(5)

	state := core.NewPendingState(
		stateDiff, newClasses, mockState, pendingBlockNumber,
	)

	t.Run("ContractClassHash", func(t *testing.T) {
		t.Run("from pending", func(t *testing.T) {
			t.Run("deployed", func(t *testing.T) {
				cH, cErr := state.ContractClassHash(deployedAddr)
				require.NoError(t, cErr)
				assert.Equal(t, deployedClassHash, &cH)

				cH, cErr = state.ContractClassHash(deployedAddr2)
				require.NoError(t, cErr)
				assert.Equal(t, deployedClassHash, &cH)
			})
			t.Run("replaced", func(t *testing.T) {
				cH, cErr := state.ContractClassHash(replacedAddr)
				require.NoError(t, cErr)
				assert.Equal(t, replacedClassHash, &cH)
			})
		})
		t.Run("from head", func(t *testing.T) {
			expectedClassHash := new(felt.Felt).SetUint64(37)
			mockState.EXPECT().ContractClassHash(gomock.Any()).Return(*expectedClassHash, nil)

			cH, cErr := state.ContractClassHash(&felt.Zero)
			require.NoError(t, cErr)
			assert.Equal(t, expectedClassHash, &cH)
		})
	})
	t.Run("ContractNonce", func(t *testing.T) {
		t.Run("from pending", func(t *testing.T) {
			cN, cErr := state.ContractNonce(deployedAddr)
			require.NoError(t, cErr)
			assert.Equal(t, new(felt.Felt).SetUint64(44), &cN)

			cN, cErr = state.ContractNonce(deployedAddr2)
			require.NoError(t, cErr)
			assert.Equal(t, felt.Zero, cN)
		})
		t.Run("from head", func(t *testing.T) {
			expectedNonce := new(felt.Felt).SetUint64(1337)
			mockState.EXPECT().ContractNonce(gomock.Any()).Return(*expectedNonce, nil)

			cN, cErr := state.ContractNonce(&felt.Zero)
			require.NoError(t, cErr)
			assert.Equal(t, expectedNonce, &cN)
		})
	})
	t.Run("ContractStorage", func(t *testing.T) {
		t.Run("from pending", func(t *testing.T) {
			cV, cErr := state.ContractStorage(deployedAddr, key)
			require.NoError(t, cErr)
			assert.Equal(t, value, &cV)

			cV, cErr = state.ContractStorage(deployedAddr, new(felt.Felt).SetUint64(0xDEADBEEF))
			require.NoError(t, cErr)
			assert.Equal(t, felt.Zero, cV)

			cV, cErr = state.ContractStorage(deployedAddr2, new(felt.Felt).SetUint64(0xDEADBEEF))
			require.NoError(t, cErr)
			assert.Equal(t, felt.Zero, cV)
		})
		t.Run("from head", func(t *testing.T) {
			expectedValue := new(felt.Felt).SetUint64(0xDEADBEEF)
			mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(*expectedValue, nil)

			cV, cErr := state.ContractStorage(&felt.Zero, &felt.Zero)
			require.NoError(t, cErr)
			assert.Equal(t, expectedValue, &cV)
		})
	})
	t.Run("ContractStorageLastUpdatedBlock", func(t *testing.T) {
		deployedAddr := (*felt.Address)(deployedAddr)
		deployedAddr2 := (*felt.Address)(deployedAddr2)
		t.Run("from pending", func(t *testing.T) {
			blockNum, err := state.ContractStorageLastUpdatedBlock(
				deployedAddr, key,
			)
			require.NoError(t, err)
			assert.Equal(t, pendingBlockNumber, blockNum)
		})
		t.Run("deployed contract with unchanged storage key", func(t *testing.T) {
			unchangedKey := felt.NewRandom[felt.Felt]()
			blockNum, err := state.ContractStorageLastUpdatedBlock(
				deployedAddr, unchangedKey,
			)
			require.NoError(t, err)
			assert.Equal(t, uint64(0), blockNum)
		})
		t.Run("deployed contract with no storage diffs", func(t *testing.T) {
			blockNum, err := state.ContractStorageLastUpdatedBlock(
				deployedAddr2, new(felt.Felt).SetUint64(0xDEADBEEF),
			)
			require.NoError(t, err)
			assert.Equal(t, uint64(0), blockNum)
		})
		t.Run("existing contract with unrelated storage diffs (falls to head)", func(t *testing.T) {
			expectedBlock := uint64(3)
			unrelatedKey := felt.NewRandom[felt.Felt]()

			mockState.EXPECT().ContractStorageLastUpdatedBlock(
				existingContractAddr, unrelatedKey,
			).Return(expectedBlock, nil)

			blockNum, err := state.ContractStorageLastUpdatedBlock(
				existingContractAddr, unrelatedKey,
			)
			require.NoError(t, err)
			assert.Equal(t, expectedBlock, blockNum)
		})
		t.Run("contract not mentioned in pending state (falls to head)", func(t *testing.T) {
			expectedBlock := uint64(3)
			randomAddr := felt.NewRandom[felt.Address]()
			randomKey := felt.NewRandom[felt.Felt]()

			mockState.EXPECT().ContractStorageLastUpdatedBlock(
				randomAddr, randomKey,
			).Return(expectedBlock, nil)

			blockNum, err := state.ContractStorageLastUpdatedBlock(randomAddr, randomKey)
			require.NoError(t, err)
			assert.Equal(t, expectedBlock, blockNum)
		})
	})
	t.Run("Class", func(t *testing.T) {
		t.Run("from pending", func(t *testing.T) {
			pC, pErr := state.Class(deployedClassHash)
			require.NoError(t, pErr)
			_, ok := pC.Class.(*core.DeprecatedCairoClass)
			assert.True(t, ok)
		})
		t.Run("from head", func(t *testing.T) {
			mockState.EXPECT().Class(gomock.Any()).Return(&core.DeclaredClassDefinition{
				Class: &core.SierraClass{},
			}, nil)
			pC, pErr := state.Class(&felt.Zero)
			require.NoError(t, pErr)
			_, ok := pC.Class.(*core.SierraClass)
			assert.True(t, ok)
		})
	})
}
