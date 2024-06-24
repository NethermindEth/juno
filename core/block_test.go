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
			// block 16789: mainnet
			// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16789"
			number: 16789,
			chain:  utils.Mainnet,
			name:   "mainnet (post 0.7.0 with sequencer address)",
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
	}

	for _, testcase := range tests {
		tc := testcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := feeder.NewTestClient(t, &tc.chain)
			gw := adaptfeeder.New(client)

			block, err := gw.BlockByNumber(context.Background(), tc.number)
			require.NoError(t, err)

			commitments, err := core.VerifyBlockHash(block, &tc.chain)
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
		commitments, err := core.VerifyBlockHash(mainnetBlock1, &utils.Mainnet)
		assert.EqualError(t, err, expectedErr)
		assert.Nil(t, commitments)
	})

	t.Run("no error if block is unverifiable", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Sepolia)
		gw := adaptfeeder.New(client)
		block, err := gw.BlockByNumber(context.Background(), 56377)
		require.NoError(t, err)

		commitments, err := core.VerifyBlockHash(block, &utils.Sepolia)
		assert.NoError(t, err)
		assert.NotNil(t, commitments)
	})

	t.Run("error if len of transactions do not match len of receipts", func(t *testing.T) {
		mainnetBlock1, err := mainnetGW.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		mainnetBlock1.Transactions = mainnetBlock1.Transactions[:len(mainnetBlock1.Transactions)-1]

		expectedErr := fmt.Sprintf("len of transactions: %v do not match len of receipts: %v",
			len(mainnetBlock1.Transactions), len(mainnetBlock1.Receipts))

		commitments, err := core.VerifyBlockHash(mainnetBlock1, &utils.Mainnet)
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
		commitments, err := core.VerifyBlockHash(mainnetBlock1, &utils.Mainnet)
		assert.EqualError(t, err, expectedErr)
		assert.Nil(t, commitments)
	})
}

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
