package rpcv7_test

import (
	"encoding/json"
	"errors"
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

	n := utils.Ptr(utils.Mainnet)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpcv7.New(mockReader, nil, mockVM, "", n, log)

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil).AnyTimes()

	blockInfo := vm.BlockInfo{Header: &core.Header{}}
	t.Run("ok with zero values", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &blockInfo, mockState, n, true, false, true).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         123,
			}, nil)

		_, httpHeader, err := handler.EstimateFee([]rpcv7.BroadcastedTransaction{}, []rpcv6.SimulationFlag{}, rpcv7.BlockID{Latest: true})
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "123")
	})

	t.Run("ok with zero values, skip validate", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &blockInfo, mockState, n, true, true, true).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         123,
			}, nil)

		_, httpHeader, err := handler.EstimateFee([]rpcv7.BroadcastedTransaction{}, []rpcv6.SimulationFlag{rpcv6.SkipValidateFlag}, rpcv7.BlockID{Latest: true})
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "123")
	})

	t.Run("transaction execution error", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &blockInfo, mockState, n, true, true, true).
			Return(vm.ExecutionResults{}, vm.TransactionExecutionError{
				Index: 44,
				Cause: errors.New("oops"),
			})

		_, httpHeader, err := handler.EstimateFee([]rpcv7.BroadcastedTransaction{}, []rpcv6.SimulationFlag{rpcv6.SkipValidateFlag}, rpcv7.BlockID{Latest: true})
		require.Equal(t, rpccore.ErrTransactionExecutionError.CloneWithData(rpcv7.TransactionExecutionErrorData{
			TransactionIndex: 44,
			ExecutionError:   "oops",
		}), err)
		require.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "0")
	})

	t.Run("transaction with invalid contract class", func(t *testing.T) {
		toFelt := func(hex string) *felt.Felt {
			return utils.HexToFelt(t, hex)
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
		_, _, err := handler.EstimateFee([]rpcv7.BroadcastedTransaction{invalidTx}, []rpcv6.SimulationFlag{}, rpcv7.BlockID{Latest: true})
		expectedErr := &jsonrpc.Error{
			Code:    jsonrpc.InvalidParams,
			Message: "Invalid Params",
			Data:    "invalid program",
		}
		require.Equal(t, expectedErr, err)
	})
}

func assertEqualCairo0Class(t *testing.T, cairo0Class *core.Cairo0Class, class *rpcv7.Class) {
	assert.Equal(t, cairo0Class.Program, class.Program)
	assert.Equal(t, cairo0Class.Abi, class.Abi.(json.RawMessage))

	require.Equal(t, len(cairo0Class.L1Handlers), len(class.EntryPoints.L1Handler))
	for idx := range cairo0Class.L1Handlers {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Index)
		assert.Equal(t, cairo0Class.L1Handlers[idx].Offset, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(t, cairo0Class.L1Handlers[idx].Selector, class.EntryPoints.L1Handler[idx].Selector)
	}

	require.Equal(t, len(cairo0Class.Constructors), len(class.EntryPoints.Constructor))
	for idx := range cairo0Class.Constructors {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Index)
		assert.Equal(t, cairo0Class.Constructors[idx].Offset, class.EntryPoints.Constructor[idx].Offset)
		assert.Equal(t, cairo0Class.Constructors[idx].Selector, class.EntryPoints.Constructor[idx].Selector)
	}

	require.Equal(t, len(cairo0Class.Externals), len(class.EntryPoints.External))
	for idx := range cairo0Class.Externals {
		assert.Nil(t, class.EntryPoints.External[idx].Index)
		assert.Equal(t, cairo0Class.Externals[idx].Offset, class.EntryPoints.External[idx].Offset)
		assert.Equal(t, cairo0Class.Externals[idx].Selector, class.EntryPoints.External[idx].Selector)
	}
}

func assertEqualCairo1Class(t *testing.T, cairo1Class *core.Cairo1Class, class *rpcv7.Class) {
	assert.Equal(t, cairo1Class.Program, class.SierraProgram)
	assert.Equal(t, cairo1Class.Abi, class.Abi.(string))
	assert.Equal(t, cairo1Class.SemanticVersion, class.ContractClassVersion)

	require.Equal(t, len(cairo1Class.EntryPoints.L1Handler), len(class.EntryPoints.L1Handler))
	for idx := range cairo1Class.EntryPoints.L1Handler {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.L1Handler[idx].Index, *class.EntryPoints.L1Handler[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.L1Handler[idx].Selector, class.EntryPoints.L1Handler[idx].Selector)
	}

	require.Equal(t, len(cairo1Class.EntryPoints.Constructor), len(class.EntryPoints.Constructor))
	for idx := range cairo1Class.EntryPoints.Constructor {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.Constructor[idx].Index, *class.EntryPoints.Constructor[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.Constructor[idx].Selector, class.EntryPoints.Constructor[idx].Selector)
	}

	require.Equal(t, len(cairo1Class.EntryPoints.External), len(class.EntryPoints.External))
	for idx := range cairo1Class.EntryPoints.External {
		assert.Nil(t, class.EntryPoints.External[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.External[idx].Index, *class.EntryPoints.External[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.External[idx].Selector, class.EntryPoints.External[idx].Selector)
	}
}
