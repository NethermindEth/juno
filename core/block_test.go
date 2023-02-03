package core_test

import (
	_ "embed"
	"testing"

	"github.com/NethermindEth/juno/testsource"
	"github.com/stretchr/testify/assert"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils"
)

func TestBlockHash(t *testing.T) {
	tests := []struct {
		blockNumber uint64
		chain       utils.Network
		name        string
		wantErr     bool
	}{
		{
			// block 231579: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockHash=0x40ffdbd9abbc4fc64652c50db94a29bce65c183316f304a95df624de708e746",
			231579,
			utils.GOERLI,
			"goerli network (post 0.7.0 with sequencer address)",
			false,
		},
		{
			// block 156000: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=156000",
			156000,
			utils.GOERLI,
			"goerli network (post 0.7.0 without sequencer address)",
			false,
		},
		{
			// block 1: goerli
			// "https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=1",
			1,
			utils.GOERLI,
			"goerli network (pre 0.7.0 without sequencer address)",
			false,
		},
		{
			// block 16789: mainnet
			// "https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=16789"
			16789,
			utils.MAINNET,
			"mainnet (post 0.7.0 with sequencer address)",
			false,
		},
		{
			// block 1: integration
			// "https://external.integration.starknet.io/feeder_gateway/get_block?blockNumber=1"
			1,
			utils.INTEGRATION,
			"integration network (pre 0.7.0 without sequencer address)",
			true,
		},
		{
			// block 119802: goerli
			// https://alpha4.starknet.io/feeder_gateway/get_block?blockNumber=119802
			119802,
			utils.GOERLI,
			"goerli network (post 0.7.0 without sequencer address)",
			true,
		},
		{
			// block 10: goerli2
			// https://alpha4-2.starknet.io/feeder_gateway/get_block?blockNumber=10
			10,
			utils.GOERLI2,
			"goerli2 network (post 0.7.0 with sequencer address)",
			false,
		},
		{
			// block 0: main
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_block?blockNumber=0
			0,
			utils.MAINNET,
			"mainnet (pre 0.7.0 without sequencer address)",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, closer := testsource.NewTestGateway(tt.chain)
			defer closer.Close()

			block, err := client.BlockByNumber(tt.blockNumber)
			assert.NoError(t, err)

			got, err := core.BlockHash(block, tt.chain)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, want error %v", err, tt.wantErr)
			}
			if !tt.wantErr && !got.Equal(block.Hash) {
				t.Errorf("got %s, want %s", got.Text(16), block.Hash.Text(16))
			}
		})
	}
}
