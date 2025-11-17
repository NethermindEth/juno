package rpcv7_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/node"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func nopCloser() error { return nil }

func TestSpecVersion(t *testing.T) {
	handler := rpcv7.New(nil, nil, nil, &utils.Mainnet, nil)
	version, rpcErr := handler.SpecVersion()
	require.Nil(t, rpcErr)
	require.Equal(t, "0.7.1", version)
}

func TestThrottledVMError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&utils.Mainnet).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)

	throttledVM := node.NewThrottledVM(mockVM, 0, 0)
	handler := rpcv7.New(mockReader, mockSyncReader, throttledVM, &utils.Mainnet, nil)
	mockState := mocks.NewMockStateReader(mockCtrl)

	throttledErr := "VM throughput limit reached"
	t.Run("call", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(felt.Zero, nil)
		_, rpcErr := handler.Call(rpcv7.FunctionCall{}, rpcv7.BlockID{Latest: true})
		assert.Equal(t, throttledErr, rpcErr.Data)
	})

	t.Run("simulate", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil)
		_, httpHeader, rpcErr := handler.SimulateTransactions(
			rpcv7.BlockID{Latest: true},
			rpcv7.BroadcastedTransactionInputs{},
			[]rpcv6.SimulationFlag{rpcv6.SkipFeeChargeFlag},
		)
		assert.Equal(t, throttledErr, rpcErr.Data)
		assert.NotEmpty(t, httpHeader.Get(rpcv7.ExecutionStepsHeader))
	})

	t.Run("trace", func(t *testing.T) {
		blockHash := felt.NewUnsafeFromString[felt.Felt]("0x0001")
		header := &core.Header{
			Hash:            blockHash,
			ParentHash:      felt.NewUnsafeFromString[felt.Felt]("0x0C3"),
			Number:          0,
			L1GasPriceETH:   felt.NewUnsafeFromString[felt.Felt]("0x777"),
			ProtocolVersion: "99.12.3",
		}
		l1Tx := &core.L1HandlerTransaction{
			TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x000000C"),
		}
		declaredClass := &core.DeclaredClassDefinition{
			At:    3002,
			Class: &core.SierraClass{},
		}
		declareTx := &core.DeclareTransaction{
			TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x000000001"),
			ClassHash:       felt.NewUnsafeFromString[felt.Felt]("0x00000BC00"),
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{l1Tx, declareTx},
		}

		mockReader.EXPECT().BlockByHash(blockHash).Return(block, nil)
		state := mocks.NewMockStateReader(mockCtrl)
		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(state, nopCloser, nil)
		headState := mocks.NewMockStateReader(mockCtrl)
		headState.EXPECT().Class(declareTx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().HeadState().Return(headState, nopCloser, nil)
		_, httpHeader, rpcErr := handler.TraceBlockTransactions(
			t.Context(),
			rpcv7.BlockID{Hash: blockHash},
		)
		assert.Equal(t, throttledErr, rpcErr.Data)
		assert.NotEmpty(t, httpHeader.Get(rpcv7.ExecutionStepsHeader))
	})
}
