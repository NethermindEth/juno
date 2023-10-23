package l1data_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/l1data"
	"github.com/NethermindEth/juno/mocks"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStateDiff(t *testing.T) {
	ctrl := gomock.NewController(t)
	l1 := mocks.NewMockL1Data(ctrl)
	logStateUpdateIndex := uint(1)
	logStateUpdateBlockNumber := uint64(2)

	stateTransitionFact := new(big.Int)
	l1.EXPECT().
		StateTransitionFact(gomock.Any(), logStateUpdateIndex, logStateUpdateBlockNumber).
		Return(stateTransitionFact, nil).
		Times(1)
	pageHash := [32]byte{}
	pageHash[0] = new(big.Int).SetUint64(1).Bytes()[0]
	pagesHashes := [][32]byte{pageHash, pageHash}
	l1.EXPECT().
		MemoryPagesHashesLog(gomock.Any(), stateTransitionFact, logStateUpdateBlockNumber).
		Return(&l1data.LogMemoryPagesHashes{
			ProgramOutputFact: [32]byte{},
			PagesHashes:       pagesHashes,
			Raw: types.Log{
				BlockNumber: logStateUpdateBlockNumber - 1,
			},
		}, nil).
		Times(1)

	methodID := "5578ceae"
	startAddr := "000000000000000000000000000000000000000000000000000000000041c1cb"
	offset := "00000000000000000000000000000000000000000000000000000000000000a0"
	numberOfValues := "0000000000000000000000000000000000000000000000000000000000000001"
	// Technically this is an invalid memory page but it doesn't matter since we aren't decoding it.
	values := "000000000000000000000000000000000000000000000000000000bafa1591b3"
	z := "05a61ae4ef396e0dafbb0a18d837445c54824cb227897309010a89bafa1591b3"
	alpha := "0255b8cf3ae0449b62d0e47bcbbea0b9143b62ac52569f1f1bedcfd9f0383b17"
	prime := "0800000000000011000000000000000000000000000000000000000000000001"
	txHash := common.Hash{}
	l1.EXPECT().
		MemoryPageFactContinuousLogs(gomock.Any(), pagesHashes, logStateUpdateBlockNumber-1).
		Return([]*l1data.LogMemoryPageFactContinuous{
			{
				Raw: types.Log{
					Data:   nil,
					TxHash: txHash,
				},
			},
			{
				Raw: types.Log{
					Data:   common.Hex2Bytes(methodID + startAddr + offset + z + alpha + prime + numberOfValues + values),
					TxHash: txHash,
				},
			},
		}, nil).
		Times(1)
	encodedDiff := []*big.Int{hexToBigInt(t, values)}
	l1.EXPECT().
		EncodedStateDiff(gomock.Any(), []common.Hash{txHash, txHash}).
		Return(encodedDiff, nil).
		Times(1)

	got, err := l1data.NewStateDiffFetcher(l1).StateDiff(context.Background(), logStateUpdateIndex, logStateUpdateBlockNumber)
	require.NoError(t, err)
	require.Equal(t, encodedDiff, got)
}
