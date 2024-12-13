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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//nolint:dupl
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
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, n, true, false, false).
			Return([]*felt.Felt{}, []core.GasConsumed{}, []vm.TransactionTrace{}, []vm.TransactionReceipt{}, stepsUsed, nil)

		_, httpHeader, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipFeeChargeFlag})
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "123")
	})

	t.Run("ok with zero values, skip validate", func(t *testing.T) {
		stepsUsed := uint64(123)
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, n, false, true, false).
			Return([]*felt.Felt{}, []core.GasConsumed{}, []vm.TransactionTrace{}, []vm.TransactionReceipt{}, stepsUsed, nil)

		_, httpHeader, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag})
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "123")
	})

	t.Run("transaction execution error", func(t *testing.T) {
		t.Run("v0_7, v0_8", func(t *testing.T) { //nolint:dupl
			mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
				Header: headsHeader,
			}, mockState, n, false, true, false).
				Return(nil, nil, []vm.TransactionTrace{}, []vm.TransactionReceipt{}, uint64(0), vm.TransactionExecutionError{
					Index: 44,
					Cause: errors.New("oops"),
				})

			_, httpHeader, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag})
			require.Equal(t, rpc.ErrTransactionExecutionError.CloneWithData(rpc.TransactionExecutionErrorData{
				TransactionIndex: 44,
				ExecutionError:   "oops",
			}), err)
			require.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "0")
		})
	})
}
