package blockchain_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPendingState(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	deployedAddr, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	deployedClassHash, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	replacedAddr, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	replacedClassHash, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	pending := blockchain.Pending{
		Block: nil,
		StateUpdate: &core.StateUpdate{
			BlockHash: nil,
			NewRoot:   nil,
			OldRoot:   nil,
			StateDiff: &core.StateDiff{
				DeployedContracts: []core.AddressClassHashPair{
					{
						Address:   deployedAddr,
						ClassHash: deployedClassHash,
					},
				},
				ReplacedClasses: []core.AddressClassHashPair{
					{
						Address:   replacedAddr,
						ClassHash: replacedClassHash,
					},
				},
				Nonces: map[felt.Felt]*felt.Felt{
					*deployedAddr: new(felt.Felt).SetUint64(44),
				},
				StorageDiffs: map[felt.Felt][]core.StorageDiff{
					*deployedAddr: {
						{
							Key:   new(felt.Felt).SetUint64(44),
							Value: new(felt.Felt).SetUint64(37),
						},
					},
				},
			},
		},
		NewClasses: map[felt.Felt]core.Class{
			*deployedClassHash: &core.Cairo0Class{},
		},
	}
	state := blockchain.NewPendingState(pending, mockState)

	t.Run("ContractClassHash", func(t *testing.T) {
		t.Run("from pending", func(t *testing.T) {
			t.Run("deployed", func(t *testing.T) {
				cH, cErr := state.ContractClassHash(deployedAddr)
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
