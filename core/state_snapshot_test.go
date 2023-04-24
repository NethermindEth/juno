package core_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestStateSnapshot(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	changeHeight := uint64(10)
	snapshotBefore := core.NewStateSnapshot(mockState, changeHeight-1)
	snapshotAfter := core.NewStateSnapshot(mockState, changeHeight+1)

	historyValue := new(felt.Felt).SetUint64(1)
	doAtReq := func(addr *felt.Felt, at uint64) (*felt.Felt, error) {
		if addr.IsZero() {
			return nil, errors.New("some error")
		}

		if at > changeHeight {
			return nil, core.ErrCheckHeadState
		}
		return historyValue, nil
	}

	mockState.EXPECT().ContractClassHashAt(gomock.Any(), gomock.Any()).DoAndReturn(doAtReq).AnyTimes()
	mockState.EXPECT().ContractNonceAt(gomock.Any(), gomock.Any()).DoAndReturn(doAtReq).AnyTimes()
	mockState.EXPECT().ContractStorageAt(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(addr, loc *felt.Felt, at uint64) (*felt.Felt, error) {
			return doAtReq(loc, at)
		},
	).AnyTimes()

	headValue := new(felt.Felt).SetUint64(2)
	var err error
	doHeadReq := func(_ *felt.Felt) (*felt.Felt, error) {
		return headValue, err
	}

	mockState.EXPECT().ContractClassHash(gomock.Any()).DoAndReturn(doHeadReq).AnyTimes()
	mockState.EXPECT().ContractNonce(gomock.Any()).DoAndReturn(doHeadReq).AnyTimes()
	mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addr, loc *felt.Felt) (*felt.Felt, error) {
			return doHeadReq(loc)
		},
	).AnyTimes()

	addr, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)
	t.Run("class hash", func(t *testing.T) {
		t.Run("correct value is in history", func(t *testing.T) {
			got, err := snapshotBefore.ContractClassHash(addr)
			require.NoError(t, err)
			require.Equal(t, historyValue, got)
		})

		t.Run("correct value is in HEAD", func(t *testing.T) {
			got, err := snapshotAfter.ContractClassHash(addr)
			require.NoError(t, err)
			require.Equal(t, headValue, got)
		})
	})
	t.Run("nonce", func(t *testing.T) {
		t.Run("correct value is in history", func(t *testing.T) {
			got, err := snapshotBefore.ContractNonce(addr)
			require.NoError(t, err)
			require.Equal(t, historyValue, got)
		})

		t.Run("correct value is in HEAD", func(t *testing.T) {
			got, err := snapshotAfter.ContractNonce(addr)
			require.NoError(t, err)
			require.Equal(t, headValue, got)
		})
	})
	t.Run("storage value", func(t *testing.T) {
		t.Run("correct value is in history", func(t *testing.T) {
			got, err := snapshotBefore.ContractStorage(addr, addr)
			require.NoError(t, err)
			require.Equal(t, historyValue, got)
		})

		t.Run("correct value is in HEAD", func(t *testing.T) {
			got, err := snapshotAfter.ContractStorage(addr, addr)
			require.NoError(t, err)
			require.Equal(t, headValue, got)
		})
	})

	t.Run("history returns some error", func(t *testing.T) {
		t.Run("class hash", func(t *testing.T) {
			_, err := snapshotAfter.ContractClassHash(&felt.Zero)
			require.EqualError(t, err, "some error")
		})
		t.Run("nonce", func(t *testing.T) {
			_, err := snapshotAfter.ContractNonce(&felt.Zero)
			require.EqualError(t, err, "some error")
		})
		t.Run("storage value", func(t *testing.T) {
			_, err := snapshotAfter.ContractStorage(&felt.Zero, &felt.Zero)
			require.EqualError(t, err, "some error")
		})
	})
}
