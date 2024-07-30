package core_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockHash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		number uint64
		chain  utils.Network
		name   string
	}{
		{
			// block 231579: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockHash=0x40ffdbd9abbc4fc64652c50db94a29bce65c183316f304a95df624de708e746",
			number: 231579,
			chain:  utils.Goerli,
			name:   "goerli network (post 0.7.0 with sequencer address)",
		},
		{
			// block 156000: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=156000",
			number: 156000,
			chain:  utils.Goerli,
			name:   "goerli network (post 0.7.0 without sequencer address)",
		},
		{
			// block 1: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=1",
			number: 1,
			chain:  utils.Goerli,
			name:   "goerli network (pre 0.7.0 without sequencer address)",
		},
		{
			// block 16789: mainnet
			// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16789"
			number: 16789,
			chain:  utils.Mainnet,
			name:   "mainnet (post 0.7.0 with sequencer address)",
		},
		{
			// block 1: integration
			// "https://external.integration.starknet.io/feeder_gateway/get_block?blockNumber=1"
			number: 1,
			chain:  utils.Integration,
			name:   "integration network (pre 0.7.0 without sequencer address)",
		},
		{
			// block 119802: goerli
			// https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=119802
			number: 119802,
			chain:  utils.Goerli,
			name:   "goerli network (post 0.7.0 without sequencer address)",
		},
		{
			// block 10: goerli2
			// https://alpha4-2.starknet.io/feeder_gateway/get_block?blockNumber=10
			number: 10,
			chain:  utils.Goerli2,
			name:   "goerli2 network (post 0.7.0 with sequencer address)",
		},
		{
			// https://alpha4-2.starknet.io/feeder_gateway/get_block?blockNumber=49
			number: 49,
			chain:  utils.Goerli2,
			name:   "Block with multiple failed transaction hash",
		},
		{
			// block 0: main
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=0
			number: 0,
			chain:  utils.Mainnet,
			name:   "mainnet (pre 0.7.0 without sequencer address)",
		},
		{
			// block 833: main
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=833
			number: 833,
			chain:  utils.Mainnet,
			name:   "mainnet 833 (post 0.7.0 without sequencer address)",
		},

		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16259"
		{
			number: 16259,
			chain:  utils.Mainnet,
			name:   "Block 16259 with Deploy transaction version 0",
		},
		// https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=485004"
		{
			number: 485004,
			chain:  utils.Goerli,
			name:   "Block 485004 with Deploy transaction version 1",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=8"
		{
			number: 8,
			chain:  utils.Mainnet,
			name:   "Block 8 with Invoke transaction version 0",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16730"
		{
			number: 16730,
			chain:  utils.Mainnet,
			name:   "Block 16730 with Invoke transaction version 1",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=2889"
		{
			number: 2889,
			chain:  utils.Mainnet,
			name:   "Block 2889 with Declare transaction version 0",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=2889"
		{
			number: 16697,
			chain:  utils.Mainnet,
			name:   "Block 16697 with Declare transaction version 1",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=1059"
		{
			number: 1059,
			chain:  utils.Mainnet,
			name:   "Block 1059 with L1Handler transaction version 0",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=7320"
		{
			number: 7320,
			chain:  utils.Mainnet,
			name:   "Block 7320 with Deploy account transaction version 1",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=192"
		{
			number: 192,
			chain:  utils.Mainnet,
			name:   "Block 192 with Failing l1 handler transaction version 1",
		},
		// "https://external.integration.starknet.io/feeder_gateway/get_block?blockNumber=283364"
		{
			number: 283364,
			chain:  utils.Integration,
			name:   "Block 283364 with Declare v2",
		},
		// "https://external.integration.starknet.io/feeder_gateway/get_block?blockNumber=286310"
		{
			number: 286310,
			chain:  utils.Integration,
			name:   "Block 286310 with version 0.11.1",
		},
		// "https://alpha4-2.starknet.io/feeder_gateway/get_block?blockNumber=110238"
		{
			number: 110238,
			chain:  utils.Goerli2,
			name:   "Block 110238 with version 0.11.1",
		},
		// https://external.integration.starknet.io/feeder_gateway/get_block?blockNumber=330363
		{
			number: 330363,
			chain:  utils.Integration,
			name:   "Block 330363 with version 0.13.1",
		},
	}

	for _, testcase := range tests {
		tc := testcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := feeder.NewTestClient(t, &tc.chain)
			gw := adaptfeeder.New(client)

			block, err := gw.BlockByNumber(context.Background(), tc.number)
			require.NoError(t, err)

			commitments, err := core.VerifyBlockHash(block, &tc.chain, nil)
			assert.NoError(t, err)
			assert.NotNil(t, commitments)
		})
	}

	h1, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	client := feeder.NewTestClient(t, &utils.Mainnet)
	mainnetGW := adaptfeeder.New(client)
	t.Run("error if block hash has not being calculated properly", func(t *testing.T) {
		mainnetBlock1, err := mainnetGW.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		mainnetBlock1.Hash = h1

		expectedErr := "can not verify hash in block header"
		commitments, err := core.VerifyBlockHash(mainnetBlock1, &utils.Mainnet, nil)
		assert.EqualError(t, err, expectedErr)
		assert.Nil(t, commitments)
	})

	t.Run("no error if block is unverifiable", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Goerli)
		goerliGW := adaptfeeder.New(client)
		block119802, err := goerliGW.BlockByNumber(context.Background(), 119802)
		require.NoError(t, err)

		commitments, err := core.VerifyBlockHash(block119802, &utils.Goerli, nil)
		assert.NoError(t, err)
		assert.NotNil(t, commitments)
	})

	t.Run("error if len of transactions do not match len of receipts", func(t *testing.T) {
		mainnetBlock1, err := mainnetGW.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		mainnetBlock1.Transactions = mainnetBlock1.Transactions[:len(mainnetBlock1.Transactions)-1]

		expectedErr := fmt.Sprintf("len of transactions: %v do not match len of receipts: %v",
			len(mainnetBlock1.Transactions), len(mainnetBlock1.Receipts))

		commitments, err := core.VerifyBlockHash(mainnetBlock1, &utils.Mainnet, nil)
		assert.EqualError(t, err, expectedErr)
		assert.Nil(t, commitments)
	})

	t.Run("error if hash of transaction doesn't match corresponding receipt hash", func(t *testing.T) {
		mainnetBlock1, err := mainnetGW.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		mainnetBlock1.Receipts[1].TransactionHash = h1
		expectedErr := fmt.Sprintf(
			"transaction hash (%v) at index: %v does not match receipt's hash (%v)",
			mainnetBlock1.Transactions[1].Hash().String(), 1,
			mainnetBlock1.Receipts[1].TransactionHash)
		commitments, err := core.VerifyBlockHash(mainnetBlock1, &utils.Mainnet, nil)
		assert.EqualError(t, err, expectedErr)
		assert.Nil(t, commitments)
	})
}

// type txData struct {
//	hash       *felt.Felt
//	version    *core.TransactionVersion
//	signatures []*felt.Felt
//}
//
// func (t txData) Hash() *felt.Felt {
//	return t.hash
//}
//
// func (t txData) Signature() []*felt.Felt {
//	return t.signatures
//}
//
// func (t txData) TxVersion() *core.TransactionVersion {
//	return t.version
//}
//
// func TestPost0132Hash(t *testing.T) {
//	txHash := new(felt.Felt).SetUint64(1)
//
//	block := &core.Block{
//		Header: &core.Header{
//			Number:           1,
//			GlobalStateRoot:  new(felt.Felt).SetUint64(2),
//			SequencerAddress: new(felt.Felt).SetUint64(3),
//			Timestamp:        4,
//			L1DAMode:         core.Blob,
//			ProtocolVersion:  "10",
//			GasPrice:         new(felt.Felt).SetUint64(7),
//			GasPriceSTRK:     new(felt.Felt).SetUint64(6),
//			L1DataGasPrice: &core.GasPrice{
//				PriceInFri: new(felt.Felt).SetUint64(10),
//				PriceInWei: new(felt.Felt).SetUint64(9),
//			},
//			ParentHash:       new(felt.Felt).SetUint64(11),
//			TransactionCount: 1,
//			EventCount:       0,
//		},
//		Transactions: []core.Transaction{
//			txData{
//				hash: txHash,
//				signatures: []*felt.Felt{
//					new(felt.Felt).SetUint64(2),
//					new(felt.Felt).SetUint64(3),
//				},
//			},
//		},
//		Receipts: []*core.TransactionReceipt{
//			{
//				Fee: new(felt.Felt).SetUint64(99804),
//				L2ToL1Message: []*core.L2ToL1Message{
//					createL2ToL1Message(34),
//					createL2ToL1Message(56),
//				},
//				TransactionHash: txHash,
//				Reverted:        true,
//				RevertReason:    "aborted",
//				ExecutionResources:
//				TotalGasConsumed: &core.GasConsumed{
//					L1Gas:     16580,
//					L1DataGas: 32,
//				},
//			},
//		},
//	}
//
//	h, err := core.Post0132Hash(block, 10, utils.HexToFelt(t, "0x281f5966e49ad7dad9323826d53d1d27c0c4e6ebe5525e2e2fbca549bfa0a67"))
//	require.NoError(t, err)
//
//	expected := utils.HexToFelt(t, "0x061e4998d51a248f1d0288d7e17f6287757b0e5e6c5e1e58ddf740616e312134")
//	assert.Equal(t, expected, h)
//}

func TestBlockHashP2P(t *testing.T) {
	mainnetGW := adaptfeeder.New(feeder.NewTestClient(t, &utils.Mainnet))

	t.Run("error if block.SequencerAddress is nil", func(t *testing.T) {
		mainnetBlock1, err := mainnetGW.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		mainnetBlock1.SequencerAddress = nil

		_, err = core.BlockHash(mainnetBlock1)
		assert.EqualError(t, err, "block.SequencerAddress is nil")
	})
}

// func TestConcatCounts(t *testing.T) {
//	result := core.ConcatCounts(4, 3, 2, core.Blob)
//	expected := utils.HexToFelt(t, "0x0000000000000004000000000000000300000000000000028000000000000000")
//
//	assert.Equal(t, expected, result)
//}
//
// func createL2ToL1Message(seed uint64) *core.L2ToL1Message {
//	addrBytes := make([]byte, 8)
//	binary.BigEndian.PutUint64(addrBytes, seed+1)
//
//	return &core.L2ToL1Message{
//		From: new(felt.Felt).SetUint64(seed),
//		To:   common.BytesToAddress(addrBytes),
//		Payload: []*felt.Felt{
//			new(felt.Felt).SetUint64(seed + 2),
//			new(felt.Felt).SetUint64(seed + 3),
//		},
//	}
//}
