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
	mockSubscriber := mocks.NewMockSubscriber(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil).WithL1Client(mockSubscriber)

	l1Receipt := `{"blockHash":"0x42b045a05a24a1585aa3f2102e238e782e4ec3220a25358c74a29fe5f5a52f47","blockNumber":"0x13e6075","contractAddress":null,"cumulativeGasUsed":"0x83cba1","effectiveGasPrice":"0x42dba7811","from":"0xc3b49b03a6d9d71f8d3fa6582437374e650f3c46","gasUsed":"0x15070","logs":[{"address":"0xc662c410c0ecf747543f5ba90660f6abebd9c8c4","blockHash":"0x42b045a05a24a1585aa3f2102e238e782e4ec3220a25358c74a29fe5f5a52f47","blockNumber":"0x13e6075","data":"0x00000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000195c3c0000000000000000000000000000000000000000000000000000048c273950000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000c3b49b03a6d9d71f8d3fa6582437374e650f3c4603a1bf949fa7424b4bd48661a62ded82bc6f6e3c5f5c6d5904c07e6143187d1b0000000000000000000000000000000000000000000000000000000000000061","logIndex":"0x11e","removed":false,"topics":["0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b","0x0000000000000000000000007ad94e71308bb65c6bc9df35cc69cc9f953d69e5","0x038862e1b15526eda31ed6fd26805c40748458db8e420cb3be3bc65c332c023b","0x03593216f3a8b22f4cf375e5486e3d13bfde9d0f26976d20ac6f653c73f7e507"],"transactionHash":"0x5780c6fe46f958a7ebf9308e6db16d819ff9e06b1e88f9e718c50cde10898f38","transactionIndex":"0x42"},{"address":"0x7ad94e71308bb65c6bc9df35cc69cc9f953d69e5","blockHash":"0x42b045a05a24a1585aa3f2102e238e782e4ec3220a25358c74a29fe5f5a52f47","blockNumber":"0x13e6075","data":"0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000003000000000000000000000000c3b49b03a6d9d71f8d3fa6582437374e650f3c4603a1bf949fa7424b4bd48661a62ded82bc6f6e3c5f5c6d5904c07e6143187d1b0000000000000000000000000000000000000000000000000000000000000061","logIndex":"0x11f","removed":false,"topics":["0x6956d5f0b9182eedf6e4d4cde0f4c961c33d12daa74e00ed363bf9ab1123bb0a","0x000000000000000000000000c3b49b03a6d9d71f8d3fa6582437374e650f3c46","0x03a1bf949fa7424b4bd48661a62ded82bc6f6e3c5f5c6d5904c07e6143187d1b"],"transactionHash":"0x5780c6fe46f958a7ebf9308e6db16d819ff9e06b1e88f9e718c50cde10898f38","transactionIndex":"0x42"}],"logsBloom":"0x00000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000010000000180000000000000000000002000000000000000000000001000000000000000000000100000000000100000000080001000000020008000000000000000000000020000000010000001000000000000000100000000000000000000000000000000000000000000000020000000000000100000000000000000002000000000000000000000000000000000000100000000000000000000040000000000000000000000000100000000000000000000000100010000000000000100000000","status":"0x1","to":"0x7ad94e71308bb65c6bc9df35cc69cc9f953d69e5","transactionHash":"0x5780c6fe46f958a7ebf9308e6db16d819ff9e06b1e88f9e718c50cde10898f38","transactionIndex":"0x42","type":"0x2"}`
	var l1TxnReceipt types.Receipt
	require.NoError(t, json.Unmarshal([]byte(l1Receipt), &l1TxnReceipt))

	l1ReceiptSepolia := `{"blockHash":"0xf7512a945f9f289eeaf4d473977379f0c1cddaab1f095654ba8b671a02b45822","blockNumber":"0x6af309","contractAddress":null,"cumulativeGasUsed":"0x4a1e7d","effectiveGasPrice":"0x368b9414c","from":"0xd1a4ef529b9d6682da7b6dcd8cb2cfe0a856928d","gasUsed":"0x16adb","logs":[{"address":"0xe2bb56ee936fd6433dc0f6e7e3b8365c906aa057","blockHash":"0xf7512a945f9f289eeaf4d473977379f0c1cddaab1f095654ba8b671a02b45822","blockNumber":"0x6af309","data":"0x000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000026e90000000000000000000000000000000000000000000000000000bbc470e72e5500000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000455448000000000000000000000000d1a4ef529b9d6682da7b6dcd8cb2cfe0a856928d05affe94e35ad8a526cd41ebfc0246f40f4693900bc1f6f44a1291f88f74634a00000000000000000000000000000000000000000000000002c68af0bb1400000000000000000000000000000000000000000000000000000000000000000000","logIndex":"0x2c","removed":false,"topics":["0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b","0x0000000000000000000000008453fc6cd1bcfe8d4dfc069c400b433054d47bdc","0x04c5772d1914fe6ce891b64eb35bf3522aeae1315647314aac58b01137607f3f","0x01b64b1b3b690b43b9b514fb81377518f4039cd3e4f4914d8a6bdf01d679fb19"],"transactionHash":"0x1dbeb9ff738382669c2d27d572f02e8b04c455c25f71d086e8d07570e0d41f20","transactionIndex":"0xe"},{"address":"0x8453fc6cd1bcfe8d4dfc069c400b433054d47bdc","blockHash":"0xf7512a945f9f289eeaf4d473977379f0c1cddaab1f095654ba8b671a02b45822","blockNumber":"0x6af309","data":"0x00000000000000000000000000000000000000000000000002c68af0bb14000000000000000000000000000000000000000000000000000000000000000026e90000000000000000000000000000000000000000000000000000bbc470e72e55","logIndex":"0x2d","removed":false,"topics":["0x5f971bd00bf3ffbca8a6d72cdd4fd92cfd4f62636161921d1e5a64f0b64ccb6d","0x000000000000000000000000d1a4ef529b9d6682da7b6dcd8cb2cfe0a856928d","0x0000000000000000000000000000000000000000000000000000000000455448","0x05affe94e35ad8a526cd41ebfc0246f40f4693900bc1f6f44a1291f88f74634a"],"transactionHash":"0x1dbeb9ff738382669c2d27d572f02e8b04c455c25f71d086e8d07570e0d41f20","transactionIndex":"0xe"}],"logsBloom":"0x00000000000000000200000000020000000000000000000000000400000000400000000000000000000000000000000000400000000000200001000008000000000000000000000000000200000000040000000000000040101000010000000000000000100000000000000000000000000000000080000000000000000000000000000000000000000000000400000000000000000000000000020000000030000000000000000000000020000000000200000000000020000000000000000000000000000000002000800002000000000000000000000000000000100000002000000000000000000000000080000000000000000000000000000200000000","status":"0x1","to":"0x8453fc6cd1bcfe8d4dfc069c400b433054d47bdc","transactionHash":"0x1dbeb9ff738382669c2d27d572f02e8b04c455c25f71d086e8d07570e0d41f20","transactionIndex":"0xe","type":"0x2"}`
	var l1TxnReceiptSepolia types.Receipt
	require.NoError(t, json.Unmarshal([]byte(l1ReceiptSepolia), &l1TxnReceiptSepolia))

	tests := map[string]struct {
		network        utils.Network
		l1TxnHash      common.Hash
		msgs           []rpc.MsgStatus
		msgHashes      []common.Hash
		l1TxnReceipt   types.Receipt
		blockNum       uint
		l1HeadBlockNum uint
	}{
		"mainnet 0.13.2.1": {
			network:   utils.Mainnet,
			l1TxnHash: common.HexToHash("0x5780c6fe46f958a7ebf9308e6db16d819ff9e06b1e88f9e718c50cde10898f38"),
			msgs: []rpc.MsgStatus{{
				L1HandlerHash:  utils.HexToFelt(t, "0xc470e30f97f64255a62215633e35a7c6ae10332a9011776dde1143ab0202c3"),
				FinalityStatus: rpc.TxnStatusAcceptedOnL1,
				FailureReason:  "",
			}},
			msgHashes:      []common.Hash{common.HexToHash("0xd8824a75a588f0726d7d83b3e9560810c763043e979fdb77b11c1a51a991235d")},
			l1TxnReceipt:   l1TxnReceipt,
			blockNum:       763497,
			l1HeadBlockNum: 763498,
		},
		"sepolia 0.13.2.1": {
			network:   utils.Sepolia,
			l1TxnHash: common.HexToHash("0x1dbeb9ff738382669c2d27d572f02e8b04c455c25f71d086e8d07570e0d41f20"),
			msgs: []rpc.MsgStatus{{
				L1HandlerHash:  utils.HexToFelt(t, "0x1b886bc9a147d3f31aa9071c72e3aaba3d4c709e338f316b0b6f3b1f74176d4"),
				FinalityStatus: rpc.TxnStatusAcceptedOnL2,
				FailureReason:  "",
			}},
			msgHashes:      []common.Hash{common.HexToHash("0x4bce24fddbc380266493f5e2b1f5625e606fb4286dd08a4f3a625032b3dd474b")}, // todo check against starkli
			l1TxnReceipt:   l1TxnReceiptSepolia,
			blockNum:       284801,
			l1HeadBlockNum: 284800,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := feeder.NewTestClient(t, &test.network)
			gw := adaptfeeder.New(client)
			block, err := gw.BlockByNumber(context.Background(), uint64(test.blockNum))
			require.NoError(t, err)

			l1handlerTxns := make([]core.Transaction, len(test.msgs))
			for i := range len(test.msgs) {
				txn, err := gw.Transaction(context.Background(), test.msgs[i].L1HandlerHash)
				require.NoError(t, err)
				l1handlerTxns[i] = txn
			}

			mockSubscriber.EXPECT().TransactionReceipt(gomock.Any(), gomock.Any()).Return(&test.l1TxnReceipt, nil)
			for i, msg := range test.msgs {
				mockReader.EXPECT().L1HandlerTxnHash(&test.msgHashes[i]).Return(msg.L1HandlerHash, nil)
				// Expects for h.TransactionStatus()
				mockReader.EXPECT().TransactionByHash(msg.L1HandlerHash).Return(l1handlerTxns[i], nil)
				mockReader.EXPECT().Receipt(msg.L1HandlerHash).Return(block.Receipts[0], block.Hash, block.Number, nil)
				mockReader.EXPECT().L1Head().Return(&core.L1Head{BlockNumber: uint64(test.l1HeadBlockNum)}, nil)
			}
			msgStatuses, rpcErr := handler.GetMessageStatus(context.Background(), &test.l1TxnHash)
			require.Nil(t, rpcErr)
			require.Equal(t, test.msgs, msgStatuses)
		})
	}
}
