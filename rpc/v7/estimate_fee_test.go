package rpcv7_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEstimateFee(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	n := &utils.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpcv7.New(mockReader, nil, mockVM, n, log)

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil).AnyTimes()

	blockInfo := vm.BlockInfo{Header: &core.Header{}}
	t.Run("ok with zero values", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &blockInfo, mockState, true, false, true, false, true, true).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         123,
			}, nil)

		_, httpHeader, err := handler.EstimateFee(
			rpcv7.BroadcastedTransactionInputs{},
			[]rpcv6.SimulationFlag{},
			rpcv7.BlockID{Latest: true},
		)
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "123")
	})

	t.Run("ok with zero values, skip validate", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &blockInfo, mockState, true, true, true, false, true, true).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         123,
			}, nil)

		_, httpHeader, err := handler.EstimateFee(
			rpcv7.BroadcastedTransactionInputs{},
			[]rpcv6.SimulationFlag{rpcv6.SkipValidateFlag},
			rpcv7.BlockID{Latest: true},
		)
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "123")
	})

	t.Run("transaction execution error", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &blockInfo, mockState, true, true, true, false, true, true).
			Return(vm.ExecutionResults{}, vm.TransactionExecutionError{
				Index: 44,
				Cause: json.RawMessage("oops"),
			})

		_, httpHeader, err := handler.EstimateFee(
			rpcv7.BroadcastedTransactionInputs{},
			[]rpcv6.SimulationFlag{rpcv6.SkipValidateFlag},
			rpcv7.BlockID{Latest: true},
		)
		require.Equal(t, rpccore.ErrTransactionExecutionError.CloneWithData(rpcv7.TransactionExecutionErrorData{
			TransactionIndex: 44,
			ExecutionError:   json.RawMessage("oops"),
		}), err)
		require.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "0")
	})

	t.Run("transaction with invalid contract class", func(t *testing.T) {
		toFelt := func(hex string) *felt.Felt {
			return felt.NewUnsafeFromString[felt.Felt](hex)
		}
		invalidTx := rpcv7.BroadcastedTransaction{
			Transaction: rpcv7.Transaction{
				Type:          rpcv7.TxnDeclare,
				Version:       toFelt("0x1"),
				Nonce:         toFelt("0x0"),
				MaxFee:        toFelt("0x1"),
				SenderAddress: toFelt("0x2"),
				Signature: &[]*felt.Felt{
					toFelt("0x123"),
				},
			},
			ContractClass: json.RawMessage(`{}`),
		}
		_, _, err := handler.EstimateFee(
			rpcv7.BroadcastedTransactionInputs{Data: []rpcv7.BroadcastedTransaction{invalidTx}},
			[]rpcv6.SimulationFlag{},
			rpcv7.BlockID{Latest: true},
		)
		expectedErr := &jsonrpc.Error{
			Code:    jsonrpc.InvalidParams,
			Message: "Invalid Params",
			Data:    "invalid program",
		}
		require.Equal(t, expectedErr, err)
	})
}
