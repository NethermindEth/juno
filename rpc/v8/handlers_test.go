package rpcv8_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/node"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func nopCloser() error { return nil }

func TestVersion(t *testing.T) {
	const version = "1.2.3-rc1"

	handler := rpcv8.New(nil, nil, nil, version, nil)
	ver, err := handler.Version()
	require.Nil(t, err)
	assert.Equal(t, version, ver)
}

func TestSpecVersion(t *testing.T) {
	handler := rpcv8.New(nil, nil, nil, "", nil)
	version, rpcErr := handler.SpecVersion()
	require.Nil(t, rpcErr)
	require.Equal(t, "0.8.0", version)
}

func TestThrottledVMError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&utils.Mainnet).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)

	throttledVM := node.NewThrottledVM(mockVM, 0, 0)
	handler := rpcv8.New(mockReader, mockSyncReader, throttledVM, "", nil)
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	throttledErr := "VM throughput limit reached"
	t.Run("call", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(new(felt.Felt), nil)
		mockState.EXPECT().Class(new(felt.Felt)).Return(&core.DeclaredClass{Class: &core.Cairo1Class{}}, nil)
		_, rpcErr := handler.Call(rpcv8.FunctionCall{}, rpcv8.BlockID{Latest: true})
		assert.Equal(t, throttledErr, rpcErr.Data)
	})

	t.Run("simulate", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil)
		_, httpHeader, rpcErr := handler.SimulateTransactions(rpcv8.BlockID{Latest: true}, []rpcv8.BroadcastedTransaction{}, []rpcv8.SimulationFlag{rpcv8.SkipFeeChargeFlag})
		assert.Equal(t, throttledErr, rpcErr.Data)
		assert.NotEmpty(t, httpHeader.Get(rpcv8.ExecutionStepsHeader))
	})

	t.Run("trace", func(t *testing.T) {
		blockHash := utils.HexToFelt(t, "0x0001")
		header := &core.Header{
			// hash is not set because it's pending block
			ParentHash:      utils.HexToFelt(t, "0x0C3"),
			Number:          0,
			L1GasPriceETH:   utils.HexToFelt(t, "0x777"),
			ProtocolVersion: "99.12.3",
		}
		l1Tx := &core.L1HandlerTransaction{
			TransactionHash: utils.HexToFelt(t, "0x000000C"),
		}
		declaredClass := &core.DeclaredClass{
			At:    3002,
			Class: &core.Cairo1Class{},
		}
		declareTx := &core.DeclareTransaction{
			TransactionHash: utils.HexToFelt(t, "0x000000001"),
			ClassHash:       utils.HexToFelt(t, "0x00000BC00"),
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{l1Tx, declareTx},
		}

		mockReader.EXPECT().BlockByHash(blockHash).Return(block, nil)
		state := mocks.NewMockStateHistoryReader(mockCtrl)
		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(state, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(declareTx.ClassHash).Return(declaredClass, nil)
		mockSyncReader.EXPECT().PendingState().Return(headState, nopCloser, nil)
		_, httpHeader, rpcErr := handler.TraceBlockTransactions(context.Background(), rpcv8.BlockID{Hash: blockHash})
		assert.Equal(t, throttledErr, rpcErr.Data)
		assert.NotEmpty(t, httpHeader.Get(rpcv8.ExecutionStepsHeader))
	})
}
