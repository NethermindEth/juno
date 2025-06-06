package core_test

import (
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := feeder.NewTestClient(t, &tc.chain)
			gw := adaptfeeder.New(client)

			block, err := gw.BlockByNumber(t.Context(), tc.number)
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
		mainnetBlock1, err := mainnetGW.BlockByNumber(t.Context(), 1)
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
		block119802, err := goerliGW.BlockByNumber(t.Context(), 119802)
		require.NoError(t, err)

		commitments, err := core.VerifyBlockHash(block119802, &utils.Goerli, nil)
		assert.NoError(t, err)
		assert.NotNil(t, commitments)
	})

	t.Run("error if len of transactions do not match len of receipts", func(t *testing.T) {
		mainnetBlock1, err := mainnetGW.BlockByNumber(t.Context(), 1)
		require.NoError(t, err)

		mainnetBlock1.Transactions = mainnetBlock1.Transactions[:len(mainnetBlock1.Transactions)-1]

		expectedErr := fmt.Sprintf("len of transactions: %v do not match len of receipts: %v",
			len(mainnetBlock1.Transactions), len(mainnetBlock1.Receipts))

		commitments, err := core.VerifyBlockHash(mainnetBlock1, &utils.Mainnet, nil)
		assert.EqualError(t, err, expectedErr)
		assert.Nil(t, commitments)
	})

	t.Run("error if hash of transaction doesn't match corresponding receipt hash", func(t *testing.T) {
		mainnetBlock1, err := mainnetGW.BlockByNumber(t.Context(), 1)
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

//nolint:dupl
func Test0132BlockHash(t *testing.T) {
	t.Parallel()
	client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
	gw := adaptfeeder.New(client)

	for _, test := range []struct {
		blockNum uint64
	}{
		{blockNum: 35748}, {blockNum: 35749}, {blockNum: 37500}, {blockNum: 38748},
	} {
		t.Run(fmt.Sprintf("blockNum=%v", test.blockNum), func(t *testing.T) {
			t.Parallel()
			b, err := gw.BlockByNumber(t.Context(), test.blockNum)
			require.NoError(t, err)

			su, err := gw.StateUpdate(t.Context(), test.blockNum)
			require.NoError(t, err)

			c, err := core.VerifyBlockHash(b, &utils.SepoliaIntegration, su.StateDiff)
			require.NoError(t, err)
			assert.NotNil(t, c)
		})
	}
}

func Test0134BlockHash(t *testing.T) {
	t.Parallel()
	client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
	gw := adaptfeeder.New(client)

	for _, test := range []struct {
		blockNum uint64
	}{
		{blockNum: 64164},
	} { //nolint:dupl
		t.Run(fmt.Sprintf("blockNum=%v", test.blockNum), func(t *testing.T) {
			t.Parallel()
			b, err := gw.BlockByNumber(t.Context(), test.blockNum)
			require.NoError(t, err)

			su, err := gw.StateUpdate(t.Context(), test.blockNum)
			require.NoError(t, err)

			c, err := core.VerifyBlockHash(b, &utils.SepoliaIntegration, su.StateDiff)
			require.NoError(t, err)
			assert.NotNil(t, c)
		})
	}
}
