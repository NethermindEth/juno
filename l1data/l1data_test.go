package l1data_test

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/l1data"
	"github.com/NethermindEth/juno/mocks"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func hexToBigInt(t *testing.T, hex string) *big.Int {
	formatted := hex
	if formatted[:2] == "0x" {
		formatted = formatted[2:]
	}
	got, ok := new(big.Int).SetString(formatted, 16)
	require.True(t, ok)
	return got
}

func blockRangeMatches(x ethereum.FilterQuery, from, to uint64) bool {
	return x.FromBlock.Uint64() == from && x.ToBlock.Uint64() == to
}

func TestStateUpdateLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	ethclient := mocks.NewMockEthClient(ctrl)
	ctx := context.Background()
	c, err := l1data.New(ethclient)
	require.NoError(t, err)

	returnedRawLog := types.Log{
		Data: common.Hex2Bytes("021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee60000000000000000000000000000000000000000000000000000000000000000"),
	}
	{
		i := 0
		ethclient.
			EXPECT().
			FilterLogs(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
				var logs []types.Log
				var returnedErr error
				var from, to uint64
				switch i {
				case 0: // Error
					from = 0
					to = 1000
					returnedErr = errors.New("test err")
				case 1: // Empty logs
					from = 0
					to = 500
				case 2: // Happy path
					from = 500
					to = 1500
					logs = []types.Log{returnedRawLog}
				}
				require.Equal(t, from, q.FromBlock.Uint64())
				require.Equal(t, to, q.ToBlock.Uint64())
				t.Log(i)
				i++
				return logs, returnedErr
			}).
			Times(3)
	}

	equalStateUpdateLogs := func(t *testing.T, want *l1data.LogStateUpdate, got *l1data.LogStateUpdate) {
		require.Equal(t, got.Raw, want.Raw)
		// We use EqualExportedValues because the `abs` unexported struct field in big.Int{} differs (`{}` vs `nil`).
		require.EqualExportedValues(t, *got.BlockNumber, *want.BlockNumber)
		require.Equal(t, got.GlobalRoot, want.GlobalRoot)
	}

	got, err := c.StateUpdateLogs(ctx, 0, 0)
	require.NoError(t, err)
	require.Len(t, got, 1)
	equalStateUpdateLogs(t, &l1data.LogStateUpdate{
		GlobalRoot:  hexToBigInt(t, "0x021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6"),
		BlockNumber: new(big.Int),
		Raw:         returnedRawLog,
	}, got[0])

	// Post-v0.12.3 LogStateUpdate
	stateRoot := "02a5b2a6238f8d99d731cf4b9ee3d06ace02b4a12b2aa2b1e7c614b744ccc94b"
	blockNumber := "00000000000000000000000000000000000000000000000000000000000545e7"
	blockHash := "03ff9b62614635c53f6cb27b0b5fb7ae596436babe4855be57e5df545a09f317"
	newReturnedRawLog := types.Log{
		Data: common.Hex2Bytes(stateRoot + blockNumber + blockHash),
	}
	ethclient.
		EXPECT().
		FilterLogs(gomock.AssignableToTypeOf(ctx), gomock.Cond(func(x any) bool {
			return blockRangeMatches(x.(ethereum.FilterQuery), 0, 1000)
		})).
		Return([]types.Log{newReturnedRawLog}, nil).
		Times(1)
	got, err = c.StateUpdateLogs(ctx, 0, 0)
	require.NoError(t, err)
	require.Len(t, got, 1)
	equalStateUpdateLogs(t, &l1data.LogStateUpdate{
		GlobalRoot:  hexToBigInt(t, stateRoot),
		BlockNumber: hexToBigInt(t, blockNumber),
		BlockHash:   hexToBigInt(t, blockHash),
		Raw:         newReturnedRawLog,
	}, got[0])
}

func TestStateTransitionFact(t *testing.T) {
	ctrl := gomock.NewController(t)
	ethclient := mocks.NewMockEthClient(ctrl)
	ctx := context.Background()
	c, err := l1data.New(ethclient)
	require.NoError(t, err)

	blockNumber := uint64(2)
	txIndex := uint(1)
	wantFact := hexToBigInt(t, "9866f8ddfe70bb512b2f2b28b49d4017c43f7ba775f1a20c61c13eea8cdac111")
	ethclient.
		EXPECT().
		FilterLogs(gomock.AssignableToTypeOf(ctx), gomock.Cond(func(x any) bool {
			return blockRangeMatches(x.(ethereum.FilterQuery), blockNumber, blockNumber)
		})).
		Return([]types.Log{{
			TxIndex: txIndex,
			Data:    wantFact.Bytes(),
		}}, nil).
		Times(1)

	fact, err := c.StateTransitionFact(ctx, txIndex, blockNumber)
	require.NoError(t, err)
	require.Equal(t, wantFact, fact)
}

func testBackwardsFetch(t *testing.T, ethclient *mocks.MockEthClient, log types.Log) { //nolint:gocritic
	i := 0
	ethclient.
		EXPECT().
		FilterLogs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
			var logs []types.Log
			var err error
			var from, to uint64
			switch i {
			case 0: // Error
				from = 1000
				to = 2000
				err = errors.New("test err")
			case 1: // Empty logs
				from = 1500
				to = 2000
			case 2: // Happy path
				from = 500
				to = 1500
				logs = []types.Log{log}
			}
			require.Equal(t, from, q.FromBlock.Uint64())
			require.Equal(t, to, q.ToBlock.Uint64())
			i++
			return logs, err
		}).
		Times(3)
}

func TestMemoryPagesHashesLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	ethclient := mocks.NewMockEthClient(ctrl)
	ctx := context.Background()
	c, err := l1data.New(ethclient)
	require.NoError(t, err)

	startBlockNumber := uint64(2000)
	programOutputFact := "cd02812618d4fb2f5d5ae4109c470c8f4ebecd693c7811883a765f8ea1baf1b7"
	abiData := "00000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000002"
	pageHash0 := "ca915ac21d0036b86c6e241b4221d5aeb30c2a140e2d6e2d5b465675bc2031ad"
	pageHash1 := "ff874398d8d0eb80d9ceaffdcfdd36b042f312cfb6d517e2f77a8cedc7ebe7cc"
	returnedRawLog := types.Log{
		Data: common.Hex2Bytes(programOutputFact + abiData + pageHash0 + pageHash1),
	}
	testBackwardsFetch(t, ethclient, returnedRawLog)
	log, err := c.MemoryPagesHashesLog(ctx, hexToBigInt(t, programOutputFact), startBlockNumber)
	require.NoError(t, err)
	require.Len(t, log.PagesHashes, 2)
	require.Equal(t, common.Hex2Bytes(pageHash0), log.PagesHashes[0][:])
	require.Equal(t, common.Hex2Bytes(pageHash1), log.PagesHashes[1][:])
	require.Equal(t, returnedRawLog, log.Raw)
}

func TestMemoryPageFactContinuousLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	ethclient := mocks.NewMockEthClient(ctrl)
	ctx := context.Background()
	c, err := l1data.New(ethclient)
	require.NoError(t, err)

	startBlockNumber := uint64(2000)
	factHash := "2d6ccfc085b95c5f1e551f064e26fc54a35fa73a9c50de119fd5d3d147307b67"
	memoryHash := "f37cdd51f9027fa9e1089ecdc004db100e54600a1ec0bdeb4db9afcf8ac3356b"
	prod := "02691e8591d3bc298591d13b90a1b5c00790c7f0353abaf6ba1bd882d00f499c"
	returnedRawLog := types.Log{
		Data: common.Hex2Bytes(factHash + memoryHash + prod),
	}
	testBackwardsFetch(t, ethclient, returnedRawLog)
	pageHashBytes := [32]byte(common.Hex2Bytes(memoryHash))
	logs, err := c.MemoryPageFactContinuousLogs(ctx, [][32]byte{pageHashBytes}, startBlockNumber)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	log := logs[0]
	factHashBytes := [32]byte(common.Hex2Bytes(factHash))
	require.Equal(t, factHashBytes, log.FactHash)
	require.Equal(t, hexToBigInt(t, memoryHash), log.MemoryHash)
	require.Equal(t, hexToBigInt(t, prod), log.Prod)
	require.Equal(t, returnedRawLog, log.Raw)
}

func TestEncodedStateDiff(t *testing.T) {
	ctrl := gomock.NewController(t)
	ethclient := mocks.NewMockEthClient(ctrl)
	ctx := context.Background()
	c, err := l1data.New(ethclient)
	require.NoError(t, err)

	txHashes := []common.Hash{common.HexToHash("0x0"), common.HexToHash("0x1")}
	methodID := "5578ceae"
	startAddr := "000000000000000000000000000000000000000000000000000000000041c1cb"
	offset := "00000000000000000000000000000000000000000000000000000000000000a0"
	numberOfValues := "0000000000000000000000000000000000000000000000000000000000000001"
	// Technically this is an invalid memory page but it doesn't matter since we aren't decoding it.
	values := "000000000000000000000000d837445c54824cb227897309010a89bafa1591b3"
	z := "05a61ae4ef396e0dafbb0a18d837445c54824cb227897309010a89bafa1591b3"
	alpha := "0255b8cf3ae0449b62d0e47bcbbea0b9143b62ac52569f1f1bedcfd9f0383b17"
	prime := "0800000000000011000000000000000000000000000000000000000000000001"
	ethclient.
		EXPECT().
		TransactionByHash(gomock.AssignableToTypeOf(ctx), txHashes[1]).
		Return(types.NewTx(&types.LegacyTx{
			Data: common.Hex2Bytes(methodID + startAddr + offset + z + alpha + prime + numberOfValues + values),
		}), false, nil).
		Times(1)
	encodedDiff, err := c.EncodedStateDiff(ctx, txHashes)
	require.NoError(t, err)
	require.Len(t, encodedDiff, 1)
	require.Equal(t, hexToBigInt(t, values), encodedDiff[0])
}
