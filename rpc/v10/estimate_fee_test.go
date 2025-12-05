package rpcv10_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
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
	handler := rpcv10.New(mockReader, nil, mockVM, log)

	mockState := mocks.NewMockStateReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil).AnyTimes()

	blockID := rpcv9.BlockIDLatest()

	blockInfo := vm.BlockInfo{Header: &core.Header{}}
	t.Run("ok with zero values", func(t *testing.T) {
		mockVM.EXPECT().Execute(
			[]core.Transaction{},
			nil,
			[]*felt.Felt{},
			&blockInfo,
			mockState,
			true,
			false,
			true, true, true, true, false).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				GasConsumed:      []core.GasConsumed{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         uint64(123),
			}, nil)

		_, httpHeader, err := handler.EstimateFee(
			t.Context(),
			rpcv10.BroadcastedTransactionInputs{},
			[]rpcv10.EstimateFlag{},
			&blockID,
		)
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv10.ExecutionStepsHeader), "123")
	})

	t.Run("ok with zero values, skip validate", func(t *testing.T) {
		mockVM.EXPECT().Execute(
			[]core.Transaction{},
			nil,
			[]*felt.Felt{},
			&blockInfo,
			mockState,
			true,
			true,
			true, true, true, true, false).
			Return(
				vm.ExecutionResults{
					OverallFees:      []*felt.Felt{},
					DataAvailability: []core.DataAvailability{},
					GasConsumed:      []core.GasConsumed{},
					Traces:           []vm.TransactionTrace{},
					NumSteps:         uint64(123),
				},
				nil,
			)

		_, httpHeader, err := handler.EstimateFee(
			t.Context(),
			rpcv10.BroadcastedTransactionInputs{},
			[]rpcv10.EstimateFlag{
				rpcv10.EstimateSkipValidateFlag,
			},
			&blockID,
		)
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv10.ExecutionStepsHeader), "123")
	})

	t.Run("transaction execution error", func(t *testing.T) {
		mockVM.EXPECT().Execute(
			[]core.Transaction{},
			nil,
			[]*felt.Felt{},
			&blockInfo,
			mockState,
			true,
			true,
			true, true, true, true, false).
			Return(vm.ExecutionResults{}, vm.TransactionExecutionError{
				Index: 44,
				Cause: json.RawMessage("oops"),
			})

		_, httpHeader, err := handler.EstimateFee(
			t.Context(),
			rpcv10.BroadcastedTransactionInputs{},
			[]rpcv10.EstimateFlag{rpcv10.EstimateSkipValidateFlag},
			&blockID,
		)
		require.Equal(t, rpccore.ErrTransactionExecutionError.CloneWithData(
			rpcv9.TransactionExecutionErrorData{
				TransactionIndex: 44,
				ExecutionError:   json.RawMessage("oops"),
			}), err)
		require.Equal(t, httpHeader.Get(rpcv10.ExecutionStepsHeader), "0")
	})

	t.Run("transaction with invalid contract class", func(t *testing.T) {
		toFelt := func(hex string) *felt.Felt {
			return felt.NewUnsafeFromString[felt.Felt](hex)
		}
		invalidTx := rpcv9.BroadcastedTransaction{
			Transaction: rpcv9.Transaction{
				Type:          rpcv9.TxnDeclare,
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
			t.Context(),
			rpcv10.BroadcastedTransactionInputs{
				Data: []rpcv10.BroadcastedTransaction{
					{BroadcastedTransaction: invalidTx},
				},
			},
			[]rpcv10.EstimateFlag{},
			&blockID,
		)
		expectedErr := &jsonrpc.Error{
			Code:    jsonrpc.InvalidParams,
			Message: "Invalid Params",
			Data:    "invalid program",
		}
		require.Equal(t, expectedErr, err)
	})
}
