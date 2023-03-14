package core_test

import (
	"context"
	"errors"
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
	tests := []struct {
		number uint64
		chain  utils.Network
		name   string
	}{
		{
			// block 231579: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockHash=0x40ffdbd9abbc4fc64652c50db94a29bce65c183316f304a95df624de708e746",
			number: 231579,
			chain:  utils.GOERLI,
			name:   "goerli network (post 0.7.0 with sequencer address)",
		},
		{
			// block 156000: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=156000",
			number: 156000,
			chain:  utils.GOERLI,
			name:   "goerli network (post 0.7.0 without sequencer address)",
		},
		{
			// block 1: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=1",
			number: 1,
			chain:  utils.GOERLI,
			name:   "goerli network (pre 0.7.0 without sequencer address)",
		},
		{
			// block 16789: mainnet
			// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16789"
			number: 16789,
			chain:  utils.MAINNET,
			name:   "mainnet (post 0.7.0 with sequencer address)",
		},
		{
			// block 1: integration
			// "https://external.integration.starknet.io/feeder_gateway/get_block?blockNumber=1"
			number: 1,
			chain:  utils.INTEGRATION,
			name:   "integration network (pre 0.7.0 without sequencer address)",
		},
		{
			// block 119802: goerli
			// https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=119802
			number: 119802,
			chain:  utils.GOERLI,
			name:   "goerli network (post 0.7.0 without sequencer address)",
		},
		{
			// block 10: goerli2
			// https://alpha4-2.starknet.io/feeder_gateway/get_block?blockNumber=10
			number: 10,
			chain:  utils.GOERLI2,
			name:   "goerli2 network (post 0.7.0 with sequencer address)",
		},
		{
			// https://alpha4-2.starknet.io/feeder_gateway/get_block?blockNumber=49
			number: 49,
			chain:  utils.GOERLI2,
			name:   "Block with multiple failed transaction hash",
		},
		{
			// block 0: main
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=0
			number: 0,
			chain:  utils.MAINNET,
			name:   "mainnet (pre 0.7.0 without sequencer address)",
		},
		{
			// block 833: main
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=833
			number: 833,
			chain:  utils.MAINNET,
			name:   "mainnet 833 (post 0.7.0 without sequencer address)",
		},

		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16259"
		{
			number: 16259,
			chain:  utils.MAINNET,
			name:   "Block 16259 with Deploy transaction version 0",
		},
		// https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=485004"
		{
			number: 485004,
			chain:  utils.GOERLI,
			name:   "Block 485004 with Deploy transaction version 1",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=8"
		{
			number: 8,
			chain:  utils.MAINNET,
			name:   "Block 8 with Invoke transaction version 0",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16730"
		{
			number: 16730,
			chain:  utils.MAINNET,
			name:   "Block 16730 with Invoke transaction version 1",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=2889"
		{
			number: 2889,
			chain:  utils.MAINNET,
			name:   "Block 2889 with Declare transaction version 0",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=2889"
		{
			number: 16697,
			chain:  utils.MAINNET,
			name:   "Block 16697 with Declare transaction version 1",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=1059"
		{
			number: 1059,
			chain:  utils.MAINNET,
			name:   "Block 1059 with L1Handler transaction version 0",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=7320"
		{
			number: 7320,
			chain:  utils.MAINNET,
			name:   "Block 7320 with Deploy account transaction version 1",
		},
		// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=192"
		{
			number: 192,
			chain:  utils.MAINNET,
			name:   "Block 192 with Failing l1 handler transaction version 1",
		},
	}

	for _, testcase := range tests {
		tc := testcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, closeFn := feeder.NewTestClient(tc.chain)
			defer closeFn()
			gw := adaptfeeder.New(client)

			block, err := gw.BlockByNumber(context.Background(), tc.number)
			require.NoError(t, err)

			err = core.VerifyBlockHash(block, tc.chain)
			if err != nil {
				if errors.As(err, new(core.ErrCantVerifyTransactionHash)) {
					for ; err != nil; err = errors.Unwrap(err) {
						t.Log(err)
					}
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}

	h1, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	defer closeFn()
	mainnetGW := adaptfeeder.New(client)
	t.Run("error if block hash has not being calculated properly", func(t *testing.T) {
		mainnetBlock1, err := mainnetGW.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		mainnetBlock1.Hash = h1

		expectedErr := "can not verify hash in block header"
		assert.EqualError(t, core.VerifyBlockHash(mainnetBlock1, utils.MAINNET), expectedErr)
	})

	t.Run("no error if block is unverifiable", func(t *testing.T) {
		client, closeFn := feeder.NewTestClient(utils.GOERLI)
		defer closeFn()
		goerliGW := adaptfeeder.New(client)
		block119802, err := goerliGW.BlockByNumber(context.Background(), 119802)
		require.NoError(t, err)

		assert.NoError(t, core.VerifyBlockHash(block119802, utils.GOERLI))
	})

	t.Run("error if len of transactions do not match len of receipts", func(t *testing.T) {
		mainnetBlock1, err := mainnetGW.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		mainnetBlock1.Transactions = mainnetBlock1.Transactions[:len(mainnetBlock1.Transactions)-1]

		expectedErr := fmt.Sprintf("len of transactions: %v do not match len of receipts: %v",
			len(mainnetBlock1.Transactions), len(mainnetBlock1.Receipts))

		assert.EqualError(t, core.VerifyBlockHash(mainnetBlock1, utils.MAINNET), expectedErr)
	})

	t.Run("error if hash of transaction doesn't match corresponding receipt hash",
		func(t *testing.T) {
			mainnetBlock1, err := mainnetGW.BlockByNumber(context.Background(), 1)
			require.NoError(t, err)

			mainnetBlock1.Receipts[1].TransactionHash = h1
			expectedErr := fmt.Sprintf(
				"transaction hash (%v) at index: %v does not match receipt's hash (%v)",
				mainnetBlock1.Transactions[1].Hash().String(), 1,
				mainnetBlock1.Receipts[1].TransactionHash)
			assert.EqualError(t, core.VerifyBlockHash(mainnetBlock1, utils.MAINNET), expectedErr)
		})
}
