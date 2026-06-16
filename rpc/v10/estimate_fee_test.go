package rpcv10_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEstimateFee(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	n := &networks.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	logger := log.NewNopZapLogger()
	handler := rpcv10.New(mockReader, nil, mockVM, logger)

	mockState := mocks.NewMockStateReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil).AnyTimes()

	blockID := rpcv10.BlockIDLatest()

	blockInfo := vm.BlockInfo{Header: &core.Header{}}
	t.Run("ok with zero values", func(t *testing.T) {
		mockVM.EXPECT().EstimateFee(
			[]core.Transaction{},
			nil,
			&blockInfo,
			mockState,
			vm.EstimateFeeOptions{}).
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
		mockVM.EXPECT().EstimateFee(
			[]core.Transaction{},
			nil,
			&blockInfo,
			mockState,
			vm.EstimateFeeOptions{
				SkipValidate: true,
			}).
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
		mockVM.EXPECT().EstimateFee(
			[]core.Transaction{},
			nil,
			&blockInfo,
			mockState,
			vm.EstimateFeeOptions{
				SkipValidate: true,
			}).
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
			rpcv10.TransactionExecutionErrorData{
				TransactionIndex: 44,
				ExecutionError:   json.RawMessage("oops"),
			}), err)
		require.Equal(t, httpHeader.Get(rpcv10.ExecutionStepsHeader), "0")
	})

	t.Run("transaction with invalid contract class", func(t *testing.T) {
		invalidTx := rpcv10.BroadcastedTransaction{
			Transaction: rpcv10.Transaction{
				Type:          rpcv10.TxnDeclare,
				Version:       felt.NewUnsafeFromString[felt.Felt]("0x3"),
				Nonce:         felt.NewUnsafeFromString[felt.Felt]("0x0"),
				MaxFee:        felt.NewUnsafeFromString[felt.Felt]("0x1"),
				SenderAddress: felt.NewUnsafeFromString[felt.Felt]("0x2"),
				Signature: &[]*felt.Felt{
					felt.NewUnsafeFromString[felt.Felt]("0x123"),
				},
			},
			ContractClass: &rpcv10.ContractClass{},
		}
		_, _, err := handler.EstimateFee(
			t.Context(),
			rpcv10.BroadcastedTransactionInputs{
				Data: []rpcv10.BroadcastedTransaction{
					invalidTx,
				},
			},
			[]rpcv10.EstimateFlag{},
			&blockID,
		)
		expectedErr := &jsonrpc.Error{
			Code:    jsonrpc.InvalidParams,
			Message: "Invalid Params",
			Data:    "resource_bounds is required for this transaction type",
		}
		require.Equal(t, expectedErr, err)
	})
}

func TestEstimateMessageFee(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	n := &networks.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	logger := log.NewNopZapLogger()
	handler := rpcv10.New(mockReader, nil, mockVM, logger)

	blockID := rpcv10.BlockIDLatest()
	to := felt.FromUint64[felt.Felt](0xABCD)
	msg := &rpcv10.MsgFromL1{
		To:       to,
		Selector: felt.FromUint64[felt.Felt](0x1),
		Payload:  []felt.Felt{felt.FromUint64[felt.Felt](0xCAFE)},
	}

	t.Run("contract not found returns ErrContractNotFound", func(t *testing.T) {
		mockState := mocks.NewMockStateReader(mockCtrl)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&msg.To).Return(felt.Felt{}, errors.New("missing"))

		_, _, err := handler.EstimateMessageFee(t.Context(), msg, &blockID)
		require.Equal(t, rpccore.ErrContractNotFound, err)
	})

	t.Run("block not found returns ErrBlockNotFound", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(123)).Return(nil, nil, db.ErrKeyNotFound)
		blockID := rpcv10.BlockIDFromNumber(123)
		_, _, err := handler.EstimateMessageFee(t.Context(), msg, &blockID)
		require.Equal(t, rpccore.ErrBlockNotFound, err)
	})

	t.Run("happy path returns FeeEstimate", func(t *testing.T) {
		mockState := mocks.NewMockStateReader(mockCtrl)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).Times(2)
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil)
		mockState.EXPECT().ContractClassHash(&msg.To).Return(felt.Felt{}, nil)

		mockVM.EXPECT().EstimateFee(
			gomock.Any(),
			nil,
			gomock.Any(),
			mockState,
			gomock.Any(),
		).Return(vm.ExecutionResults{
			OverallFees:      []*felt.Felt{felt.NewFromUint64[felt.Felt](42)},
			DataAvailability: []core.DataAvailability{{}},
			GasConsumed:      []core.GasConsumed{{L1Gas: 1, L2Gas: 2, L1DataGas: 3}},
			Traces:           []vm.TransactionTrace{{Type: vm.TxnL1Handler}},
			NumSteps:         10,
		}, nil)

		feeEstimate, _, err := handler.EstimateMessageFee(t.Context(), msg, &blockID)
		require.Nil(t, err)
		assert.NotEmpty(t, feeEstimate)
	})
}
