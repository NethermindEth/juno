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
	t.Skip()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	network := utils.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&network).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpc.New(mockReader, nil, mockVM, "", utils.NewNopZapLogger())

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	headsHeader := &core.Header{
		SequencerAddress: network.BlockHashMetaInfo.FallBackSequencerAddress,
	}
	mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil).AnyTimes()

	t.Run("ok with zero values, skip fee", func(t *testing.T) {
		mockVM.EXPECT().Execute(nil, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, &network, true, false, false, false).
			Return([]*felt.Felt{}, []vm.TransactionTrace{}, nil)

		_, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipFeeChargeFlag})
		require.Nil(t, err)
	})

	t.Run("ok with zero values, skip validate", func(t *testing.T) {
		mockVM.EXPECT().Execute(nil, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, &network, false, false, false, false).
			Return([]*felt.Felt{}, []vm.TransactionTrace{}, nil)

		_, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag})
		require.Nil(t, err)
	})

	t.Run("transaction execution error", func(t *testing.T) {
		mockVM.EXPECT().Execute(nil, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, &network, false, false, false, false).
			Return(nil, nil, vm.TransactionExecutionError{
				Index: 44,
				Cause: errors.New("oops"),
			})

		_, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag})
		require.Equal(t, rpc.ErrTransactionExecutionError.CloneWithData(rpc.TransactionExecutionErrorData{
			TransactionIndex: 44,
			ExecutionError:   "oops",
		}), err)

		mockVM.EXPECT().Execute(nil, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, &network, false, true, true, false).
			Return(nil, nil, vm.TransactionExecutionError{
				Index: 44,
				Cause: errors.New("oops"),
			})

		_, err = handler.SimulateTransactionsV0_6(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag})
		require.Equal(t, rpc.ErrContractError.CloneWithData(rpc.ContractErrorData{
			RevertError: "oops",
		}), err)
	})
}
