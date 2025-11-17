package core_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStateSnapshot(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	deployedHeight := uint64(3)
	changeHeight := uint64(10)
	snapshotBeforeDeployment := core.NewStateSnapshot(mockState, deployedHeight-1)
	snapshotBeforeChange := core.NewStateSnapshot(mockState, deployedHeight)
	snapshotAfterChange := core.NewStateSnapshot(mockState, changeHeight+1)

	historyValue := new(felt.Felt).SetUint64(1)
	doAtReq := func(addr *felt.Felt, at uint64) (felt.Felt, error) {
		if addr.IsZero() {
			return felt.Zero, errors.New("some error")
		}

		if at > changeHeight {
			return felt.Zero, core.ErrCheckHeadState
		}
		return *historyValue, nil
	}

	mockState.EXPECT().ContractDeployedAt(
		gomock.Any(),
		gomock.Any(),
	).DoAndReturn(
		func(addr *felt.Felt, height uint64) (bool, error) {
			return deployedHeight <= height, nil
		}).AnyTimes()
	mockState.EXPECT().ContractClassHashAt(gomock.Any(), gomock.Any()).DoAndReturn(doAtReq).AnyTimes()
	mockState.EXPECT().ContractNonceAt(gomock.Any(), gomock.Any()).DoAndReturn(doAtReq).AnyTimes()
	mockState.EXPECT().ContractStorageAt(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(addr, loc *felt.Felt, at uint64) (felt.Felt, error) {
			return doAtReq(loc, at)
		},
	).AnyTimes()

	headValue := new(felt.Felt).SetUint64(2)
	var err error
	doHeadReq := func(_ *felt.Felt) (felt.Felt, error) {
		return *headValue, err
	}

	mockState.EXPECT().ContractClassHash(gomock.Any()).DoAndReturn(doHeadReq).AnyTimes()
	mockState.EXPECT().ContractNonce(gomock.Any()).DoAndReturn(doHeadReq).AnyTimes()
	mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addr, loc *felt.Felt) (felt.Felt, error) {
			return doHeadReq(loc)
		},
	).AnyTimes()

	addr := felt.NewRandom[felt.Felt]()
	require.NoError(t, err)

	for desc, test := range map[string]struct {
		snapshot core.CommonStateReader
		checker  func(*testing.T, felt.Felt, error)
	}{
		"contract is not deployed": {
			snapshot: snapshotBeforeDeployment,
			checker: func(t *testing.T, _ felt.Felt, err error) {
				require.ErrorIs(t, err, db.ErrKeyNotFound)
			},
		},
		"correct value is in history": {
			snapshot: snapshotBeforeChange,
			checker: func(t *testing.T, got felt.Felt, err error) {
				require.NoError(t, err)
				require.Equal(t, *historyValue, got)
			},
		},
		"correct value is in HEAD": {
			snapshot: snapshotAfterChange,
			checker: func(t *testing.T, got felt.Felt, err error) {
				require.NoError(t, err)
				require.Equal(t, *headValue, got)
			},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			t.Run("class hash", func(t *testing.T) {
				got, err := test.snapshot.ContractClassHash(addr)
				test.checker(t, got, err)
			})
			t.Run("nonce", func(t *testing.T) {
				got, err := test.snapshot.ContractNonce(addr)
				test.checker(t, got, err)
			})
			t.Run("storage value", func(t *testing.T) {
				got, err := test.snapshot.ContractStorage(addr, addr)
				test.checker(t, got, err)
			})
		})
	}

	t.Run("history returns some error", func(t *testing.T) {
		t.Run("class hash", func(t *testing.T) {
			_, err := snapshotAfterChange.ContractClassHash(&felt.Zero)
			require.EqualError(t, err, "some error")
		})
		t.Run("nonce", func(t *testing.T) {
			_, err := snapshotAfterChange.ContractNonce(&felt.Zero)
			require.EqualError(t, err, "some error")
		})
		t.Run("storage value", func(t *testing.T) {
			_, err := snapshotAfterChange.ContractStorage(&felt.Zero, &felt.Zero)
			require.EqualError(t, err, "some error")
		})
	})

	declareHeight := deployedHeight
	mockState.EXPECT().Class(gomock.Any()).Return(
		&core.DeclaredClassDefinition{At: declareHeight},
		nil,
	).AnyTimes()

	t.Run("before class is declared", func(t *testing.T) {
		_, err := snapshotBeforeDeployment.Class(addr)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("on height that class is declared", func(t *testing.T) {
		declared, err := snapshotBeforeChange.Class(addr)
		require.NoError(t, err)
		require.Equal(t, declareHeight, declared.At)
	})

	t.Run("after class is declared", func(t *testing.T) {
		declared, err := snapshotAfterChange.Class(addr)
		require.NoError(t, err)
		require.Equal(t, declareHeight, declared.At)
	})
}
