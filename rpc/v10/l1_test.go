package rpcv10_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v10"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetMessageStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSubscriber := mocks.NewMockSubscriber(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	handler := rpc.New(mockReader, mockSyncReader, nil, nil).WithL1Client(mockSubscriber)

	//nolint:lll // Long path name
	rawL1Receipt, err := os.ReadFile(
		"testdata/messageStatus/mainnet_0x5780c6fe46f958a7ebf9308e6db16d819ff9e06b1e88f9e718c50cde10898f38.json",
	)
	require.NoError(t, err)

	var l1TxnReceipt types.Receipt
	require.NoError(t, json.Unmarshal(rawL1Receipt, &l1TxnReceipt))

	//nolint:lll // Long path name
	rawL1ReceiptSepolia, err := os.ReadFile(
		"testdata/messageStatus/sepolia_0xeafadb9958437ef43ce7ed19f8ac0c8071c18f4a55fd778cecc23d8b6f86026f.json",
	)
	require.NoError(t, err)

	var l1TxnReceiptSepolia types.Receipt
	require.NoError(t, json.Unmarshal(rawL1ReceiptSepolia, &l1TxnReceiptSepolia))

	tests := map[string]struct {
		network        utils.Network
		l1TxnHash      common.Hash
		msgs           []rpc.MsgStatus
		msgHashes      []common.Hash
		l1TxnReceipt   types.Receipt
		blockNum       uint64
		l1HeadBlockNum uint64
	}{
		"mainnet 0.13.2.1": {
			network: utils.Mainnet,
			l1TxnHash: common.HexToHash(
				"0x5780c6fe46f958a7ebf9308e6db16d819ff9e06b1e88f9e718c50cde10898f38",
			),
			msgs: []rpc.MsgStatus{{
				L1HandlerHash: felt.NewUnsafeFromString[felt.Felt](
					"0xc470e30f97f64255a62215633e35a7c6ae10332a9011776dde1143ab0202c3",
				),
				FinalityStatus:  rpc.TxnStatusAcceptedOnL1,
				FailureReason:   "",
				ExecutionStatus: rpc.TxnSuccess,
			}},
			msgHashes: []common.Hash{
				common.HexToHash("0xd8824a75a588f0726d7d83b3e9560810c763043e979fdb77b11c1a51a991235d"),
			},
			l1TxnReceipt:   l1TxnReceipt,
			blockNum:       763497,
			l1HeadBlockNum: 763498,
		},
		"sepolia 0.13.4": {
			network: utils.Sepolia,
			l1TxnHash: common.HexToHash(
				"0xeafadb9958437ef43ce7ed19f8ac0c8071c18f4a55fd778cecc23d8b6f86026f",
			),
			msgs: []rpc.MsgStatus{{
				L1HandlerHash: felt.NewUnsafeFromString[felt.Felt](
					"0x304c78cccf0569159d4b2aff2117f060509b7c6d590ae740d2031d1eb507b10",
				),
				FinalityStatus:  rpc.TxnStatusAcceptedOnL2,
				FailureReason:   "",
				ExecutionStatus: rpc.TxnSuccess,
			}},
			msgHashes: []common.Hash{
				common.HexToHash("0x162e74b4ccf7e350a1668de856f892057e0da112e1ad2262603306ee5dffb158"),
			},
			l1TxnReceipt:   l1TxnReceiptSepolia,
			blockNum:       469719,
			l1HeadBlockNum: 469718,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := feeder.NewTestClient(t, &test.network)
			gw := adaptfeeder.New(client)
			block, err := gw.BlockByNumber(t.Context(), test.blockNum)
			require.NoError(t, err)

			preConfirmed := &core.PreConfirmed{
				Block: &core.Block{
					Header: &core.Header{
						Number:           block.Number + 1,
						TransactionCount: 0,
					},
				},
			}
			mockSyncReader.EXPECT().PreConfirmed().Return(preConfirmed, nil).AnyTimes()
			l1handlerTxns := make([]core.Transaction, len(test.msgs))
			for i := range len(test.msgs) {
				//nolint:staticcheck //SA1019: used here to get the stored txs in testdata feeder
				txn, err := gw.Transaction(t.Context(), test.msgs[i].L1HandlerHash)
				require.NoError(t, err)
				l1handlerTxns[i] = txn
			}

			mockSubscriber.EXPECT().TransactionReceipt(
				gomock.Any(),
				gomock.Any(),
			).Return(&test.l1TxnReceipt, nil)
			for i, msg := range test.msgs {
				mockReader.EXPECT().L1HandlerTxnHash(&test.msgHashes[i]).Return(
					*msg.L1HandlerHash,
					nil,
				)
				// Expects for h.TransactionStatus()
				mockReader.EXPECT().BlockNumberAndIndexByTxHash(
					(*felt.TransactionHash)(msg.L1HandlerHash),
				).Return(block.Number, uint64(i), nil)
				mockReader.EXPECT().TransactionByBlockNumberAndIndex(
					block.Number, uint64(i),
				).Return(l1handlerTxns[i], nil)
				mockReader.EXPECT().ReceiptByBlockNumberAndIndex(
					block.Number, uint64(i),
				).Return(*block.Receipts[i], block.Hash, nil)
				mockReader.EXPECT().L1Head().Return(core.L1Head{BlockNumber: test.l1HeadBlockNum}, nil)
			}
			msgStatuses, rpcErr := handler.GetMessageStatus(t.Context(), &test.l1TxnHash)
			require.Nil(t, rpcErr)
			require.Equal(t, test.msgs, msgStatuses)
		})
	}

	t.Run("l1 client not found", func(t *testing.T) {
		handler := rpc.New(nil, nil, nil, nil).WithL1Client(nil)
		msgStatuses, rpcErr := handler.GetMessageStatus(t.Context(), &common.Hash{})
		require.Nil(t, msgStatuses)
		require.NotNil(t, rpcErr)
	})
}
