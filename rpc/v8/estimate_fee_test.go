package rpcv8_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v8"
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
	handler := rpc.New(mockReader, nil, mockVM, logger)

	mockState := mocks.NewMockStateReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil).AnyTimes()

	blockID := blockIDLatest(t)

	blockInfo := vm.BlockInfo{Header: &core.Header{}}
	t.Run("ok with zero values", func(t *testing.T) {
		mockVM.EXPECT().EstimateFee(
			[]core.Transaction{},
			nil,
			&blockInfo,
			mockState,
			vm.EstimateFeeOptions{}).
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
			rpc.BroadcastedTransactionInputs{},
			[]rpc.SimulationFlag{},
			&blockID,
		)
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "123")
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
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				GasConsumed:      []core.GasConsumed{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         uint64(123),
			}, nil)

		_, httpHeader, err := handler.EstimateFee(
			t.Context(),
			rpc.BroadcastedTransactionInputs{},
			[]rpc.SimulationFlag{rpc.SkipValidateFlag},
			&blockID,
		)
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "123")
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
			Return(
				vm.ExecutionResults{},
				vm.TransactionExecutionError{
					Index: 44,
					Cause: json.RawMessage("oops"),
				},
			)

		_, httpHeader, err := handler.EstimateFee(
			t.Context(),
			rpc.BroadcastedTransactionInputs{},
			[]rpc.SimulationFlag{rpc.SkipValidateFlag},
			&blockID,
		)
		require.Equal(t, rpccore.ErrTransactionExecutionError.CloneWithData(rpc.TransactionExecutionErrorData{
			TransactionIndex: 44,
			ExecutionError:   json.RawMessage("oops"),
		}), err)
		require.Equal(t, httpHeader.Get(rpc.ExecutionStepsHeader), "0")
	})

	t.Run("transaction with invalid contract class", func(t *testing.T) {
		invalidTx := rpc.BroadcastedTransaction{
			Transaction: rpc.Transaction{
				Type:          rpc.TxnDeclare,
				Version:       felt.NewUnsafeFromString[felt.Felt]("0x3"),
				Nonce:         felt.NewUnsafeFromString[felt.Felt]("0x0"),
				MaxFee:        felt.NewUnsafeFromString[felt.Felt]("0x1"),
				SenderAddress: felt.NewUnsafeFromString[felt.Felt]("0x2"),
				Signature: &felt.Slice[felt.Felt]{
					felt.UnsafeFromString[felt.Felt]("0x123"),
				},
			},
			ContractClass: json.RawMessage(`{}`),
		}
		_, _, err := handler.EstimateFee(
			t.Context(),
			rpc.BroadcastedTransactionInputs{Data: []rpc.BroadcastedTransaction{invalidTx}},
			[]rpc.SimulationFlag{},
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

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpc.New(mockReader, nil, mockVM, log.NewNopZapLogger())

	mockState := mocks.NewMockStateReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil).AnyTimes()

	blockID := blockIDLatest(t)
	msg := &rpc.MsgFromL1{
		To:       felt.FromUint64[felt.Felt](0xABCD),
		Selector: felt.FromUint64[felt.Felt](0x1),
		Payload:  []felt.Felt{felt.FromUint64[felt.Felt](0xCAFE)},
	}

	t.Run("overall fee below the minimum is raised to the minimum", func(t *testing.T) {
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
		}, nil)

		got, _, err := handler.EstimateMessageFee(t.Context(), msg, &blockID)
		require.Nil(t, err)
		assert.Equal(t, rpccore.MinL1ToL2MessageFee, *got.OverallFee)
		assert.Equal(t, uint64(1), got.L1GasConsumed.Uint64())
	})

	t.Run("overall fee above the minimum is unchanged", func(t *testing.T) {
		want := felt.FromUint64[felt.Felt](60_000_000_000_000)
		mockVM.EXPECT().EstimateFee(
			gomock.Any(),
			nil,
			gomock.Any(),
			mockState,
			gomock.Any(),
		).Return(vm.ExecutionResults{
			OverallFees:      []*felt.Felt{&want},
			DataAvailability: []core.DataAvailability{{}},
			GasConsumed:      []core.GasConsumed{{}},
			Traces:           []vm.TransactionTrace{{Type: vm.TxnL1Handler}},
		}, nil)

		got, _, err := handler.EstimateMessageFee(t.Context(), msg, &blockID)
		require.Nil(t, err)
		assert.Equal(t, want, *got.OverallFee)
	})
}
