package rpc_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSimulateTransactions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	n := utils.Ptr(utils.Mainnet)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpc.New(mockReader, nil, mockVM, "", utils.NewNopZapLogger())

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	headsHeader := &core.Header{
		SequencerAddress: n.BlockHashMetaInfo.FallBackSequencerAddress,
	}
	mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil).AnyTimes()

	t.Run("ok with zero values, skip fee", func(t *testing.T) {
		stepsUsed := uint64(123)
		tnx := make([]core.Transaction, 0)
		mockVM.EXPECT().Execute(tnx, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, n, true, false, false, true).
			Return([]*felt.Felt{}, []*felt.Felt{}, []vm.TransactionTrace{}, stepsUsed, nil)

		_, httpHeader, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipFeeChargeFlag})
		require.Nil(t, err)
		require.NotNil(t, httpHeader)
		require.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "123")
	})

	t.Run("ok with zero values, skip validate", func(t *testing.T) {
		stepsUsed := uint64(123)
		tnx := make([]core.Transaction, 0)
		mockVM.EXPECT().Execute(tnx, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, n, false, true, false, true).
			Return([]*felt.Felt{}, []*felt.Felt{}, []vm.TransactionTrace{}, stepsUsed, nil)

		_, httpHeader, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag})
		require.Nil(t, err)
		require.NotNil(t, httpHeader)
		require.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "123")
	})

	t.Run("transaction execution error", func(t *testing.T) {
		stepsUsed := uint64(0)
		tnx := make([]core.Transaction, 0)
		mockVM.EXPECT().Execute(tnx, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, n, false, true, false, true).
			Return(nil, nil, nil, stepsUsed, vm.TransactionExecutionError{
				Index: 44,
				Cause: errors.New("oops"),
			})

		_, httpHeader, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag})
		require.Equal(t, rpc.ErrTransactionExecutionError.CloneWithData(rpc.TransactionExecutionErrorData{
			TransactionIndex: 44,
			ExecutionError:   "oops",
		}), err)
		require.NotNil(t, httpHeader)
		require.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "0")

		mockVM.EXPECT().Execute(tnx, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, n, false, true, true, false).
			Return(nil, nil, nil, stepsUsed, vm.TransactionExecutionError{
				Index: 44,
				Cause: errors.New("oops"),
			})

		_, httpHeader, err = handler.SimulateTransactionsV0_6(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag})
		require.Equal(t, rpc.ErrTransactionExecutionError.CloneWithData(rpc.TransactionExecutionErrorData{
			ExecutionError:   "oops",
			TransactionIndex: 0x2c,
		}), err) // todo: re-check later!
		require.NotNil(t, httpHeader)
		require.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "0")
	})
}
