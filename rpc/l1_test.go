package rpc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
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
	ethClient := mocks.NewMockEthClient(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil).WithETHClient(ethClient)
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	l1Receipt := `{"blockHash":"0x42b045a05a24a1585aa3f2102e238e782e4ec3220a25358c74a29fe5f5a52f47","blockNumber":"0x13e6075","contractAddress":null,"cumulativeGasUsed":"0x83cba1","effectiveGasPrice":"0x42dba7811","from":"0xc3b49b03a6d9d71f8d3fa6582437374e650f3c46","gasUsed":"0x15070","logs":[{"address":"0xc662c410c0ecf747543f5ba90660f6abebd9c8c4","blockHash":"0x42b045a05a24a1585aa3f2102e238e782e4ec3220a25358c74a29fe5f5a52f47","blockNumber":"0x13e6075","data":"0x00000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000195c3c0000000000000000000000000000000000000000000000000000048c273950000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000c3b49b03a6d9d71f8d3fa6582437374e650f3c4603a1bf949fa7424b4bd48661a62ded82bc6f6e3c5f5c6d5904c07e6143187d1b0000000000000000000000000000000000000000000000000000000000000061","logIndex":"0x11e","removed":false,"topics":["0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b","0x0000000000000000000000007ad94e71308bb65c6bc9df35cc69cc9f953d69e5","0x038862e1b15526eda31ed6fd26805c40748458db8e420cb3be3bc65c332c023b","0x03593216f3a8b22f4cf375e5486e3d13bfde9d0f26976d20ac6f653c73f7e507"],"transactionHash":"0x5780c6fe46f958a7ebf9308e6db16d819ff9e06b1e88f9e718c50cde10898f38","transactionIndex":"0x42"},{"address":"0x7ad94e71308bb65c6bc9df35cc69cc9f953d69e5","blockHash":"0x42b045a05a24a1585aa3f2102e238e782e4ec3220a25358c74a29fe5f5a52f47","blockNumber":"0x13e6075","data":"0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000003000000000000000000000000c3b49b03a6d9d71f8d3fa6582437374e650f3c4603a1bf949fa7424b4bd48661a62ded82bc6f6e3c5f5c6d5904c07e6143187d1b0000000000000000000000000000000000000000000000000000000000000061","logIndex":"0x11f","removed":false,"topics":["0x6956d5f0b9182eedf6e4d4cde0f4c961c33d12daa74e00ed363bf9ab1123bb0a","0x000000000000000000000000c3b49b03a6d9d71f8d3fa6582437374e650f3c46","0x03a1bf949fa7424b4bd48661a62ded82bc6f6e3c5f5c6d5904c07e6143187d1b"],"transactionHash":"0x5780c6fe46f958a7ebf9308e6db16d819ff9e06b1e88f9e718c50cde10898f38","transactionIndex":"0x42"}],"logsBloom":"0x00000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000010000000180000000000000000000002000000000000000000000001000000000000000000000100000000000100000000080001000000020008000000000000000000000020000000010000001000000000000000100000000000000000000000000000000000000000000000020000000000000100000000000000000002000000000000000000000000000000000000100000000000000000000040000000000000000000000000100000000000000000000000100010000000000000100000000","status":"0x1","to":"0x7ad94e71308bb65c6bc9df35cc69cc9f953d69e5","transactionHash":"0x5780c6fe46f958a7ebf9308e6db16d819ff9e06b1e88f9e718c50cde10898f38","transactionIndex":"0x42","type":"0x2"}`
	var l1TxnReceipt types.Receipt
	err := json.Unmarshal([]byte(l1Receipt), &l1TxnReceipt)
	require.NoError(t, err)

	tests := map[string]struct {
		l1TxnHash    common.Hash
		msgs         []rpc.MsgStatus
		msgHashes    []common.Hash
		l1TxnReceipt types.Receipt
		blockNum     uint
	}{"mainnet 0.13.2.1": {
		l1TxnHash: common.HexToHash("0x5780c6fe46f958a7ebf9308e6db16d819ff9e06b1e88f9e718c50cde10898f38"),
		msgs: []rpc.MsgStatus{{
			L1HandlerHash:  utils.HexToFelt(t, "0xc470e30f97f64255a62215633e35a7c6ae10332a9011776dde1143ab0202c3"),
			FinalityStatus: rpc.TxnAcceptedOnL1,
			FailureReason:  "",
		}},
		msgHashes:    []common.Hash{common.HexToHash("0x618402cb4ba8206046d99e0b128b2a65a7a592546ad239df8fa0eeee18848d37")},
		l1TxnReceipt: l1TxnReceipt,
		blockNum:     763497,
	}}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			block, err := gw.BlockByNumber(context.Background(), uint64(test.blockNum))
			require.NoError(t, err)

			l1handlerTxns := make([]core.Transaction, len(test.msgs))
			for i := range len(test.msgs) {
				txn, err := gw.Transaction(context.Background(), test.msgs[i].L1HandlerHash)
				require.NoError(t, err)
				l1handlerTxns[i] = txn
			}

			ethClient.EXPECT().TransactionReceipt(gomock.Any(), gomock.Any()).Return(&test.l1TxnReceipt, nil)
			for i, msg := range test.msgs {
				mockReader.EXPECT().L1HandlerTxnHash(&test.msgHashes[i]).Return(msg.L1HandlerHash, nil)
				// Expects for h.TransactionStatus()
				mockReader.EXPECT().TransactionByHash(msg.L1HandlerHash).Return(l1handlerTxns[i], nil)
				mockReader.EXPECT().Receipt(msg.L1HandlerHash).Return(block.Receipts[0], block.Hash, block.Number, nil)
				mockReader.EXPECT().L1Head().Return(&core.L1Head{BlockNumber: uint64(test.blockNum) + 1}, nil)
			}
			msgStatuses, rpcErr := handler.GetMessageStatus(context.Background(), &test.l1TxnHash)
			require.Nil(t, rpcErr)
			require.Equal(t, test.msgs, msgStatuses)
		})
	}
}
