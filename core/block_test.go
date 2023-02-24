package core_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/testsource"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockHash(t *testing.T) {
	tests := []struct {
		blockNumber uint64
		chain       utils.Network
		name        string
	}{
		{
			// block 231579: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockHash=0x40ffdbd9abbc4fc64652c50db94a29bce65c183316f304a95df624de708e746",
			231579,
			utils.GOERLI,
			"goerli network (post 0.7.0 with sequencer address)",
		},
		{
			// block 156000: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=156000",
			156000,
			utils.GOERLI,
			"goerli network (post 0.7.0 without sequencer address)",
		},
		{
			// block 1: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=1",
			1,
			utils.GOERLI,
			"goerli network (pre 0.7.0 without sequencer address)",
		},
		{
			// block 16789: mainnet
			// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16789"
			16789,
			utils.MAINNET,
			"mainnet (post 0.7.0 with sequencer address)",
		},
		{
			// block 1: integration
			// "https://external.integration.starknet.io/feeder_gateway/get_block?blockNumber=1"
			1,
			utils.INTEGRATION,
			"integration network (pre 0.7.0 without sequencer address)",
		},
		{
			// block 119802: goerli
			// https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=119802
			119802,
			utils.GOERLI,
			"goerli network (post 0.7.0 without sequencer address)",
		},
		{
			// block 10: goerli2
			// https://alpha4-2.starknet.io/feeder_gateway/get_block?blockNumber=10
			10,
			utils.GOERLI2,
			"goerli2 network (post 0.7.0 with sequencer address)",
		},
		{
			// block 0: main
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=0
			0,
			utils.MAINNET,
			"mainnet (pre 0.7.0 without sequencer address)",
		},
		{
			// block 833: main
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=833
			833,
			utils.MAINNET,
			"mainnet 833 (post 0.7.0 without sequencer address)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client, closer := testsource.NewTestGateway(tt.chain)
			defer closer.Close()

			block, err := client.BlockByNumber(context.Background(), tt.blockNumber)
			require.NoError(t, err)
			assert.NoError(t, core.VerifyBlockHash(block, tt.chain))
		})
	}

	t.Run("no error if block is unverifiable", func(t *testing.T) {
		goerliGW, closer := testsource.NewTestGateway(utils.GOERLI)
		defer closer.Close()
		block119802, err := goerliGW.BlockByNumber(context.Background(), 119802)
		require.NoError(t, err)

		err = core.VerifyBlockHash(block119802, utils.GOERLI)
		assert.NoError(t, err)
	})
}
