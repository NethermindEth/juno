package core_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateDiffCommitment(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	tests := []struct {
		blockNum uint64
		expected string
	}{
		{
			blockNum: 0,
			expected: "0x6c4a7559b57caded12ad2275f78c4ac310ff54b2e233d25c9cf4891c251b450",
		},
		{
			blockNum: 1,
			expected: "0x13beed68d79c0ff1d6b465660bcf245a7f0ec11af5e9c6564fba30543705fe3",
		},
		{
			blockNum: 2,
			expected: "0x68e08eb5ae2790c1aeaeeef3c6fddebc27290d6415ef6b8e1e815f87afba5a7",
		},
		{
			blockNum: 21656,
			expected: "0x5f5b2b85f704d609c4f38ddc890f3bb4bd92878c320c55dceb6e01d5319fa00",
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("blockNum=%d", test.blockNum), func(t *testing.T) {
			su, err := gw.StateUpdate(context.Background(), test.blockNum)
			require.NoError(t, err)
			commitment := su.StateDiff.Commitment()
			assert.Equal(t, utils.HexToFelt(t, test.expected), commitment)
		})
	}
}

func TestStateDiffHash(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
	gw := adaptfeeder.New(client)

	for _, test := range []struct {
		blockNum uint64
		expected string
	}{
		{
			blockNum: 37500,
			expected: "0x114e85f23a3dc3febd8dccb01d701220dbf314dd30b2db2c649edcd4bc35b2b",
		},
		{
			blockNum: 35748,
			expected: "0x23587c54d590b57b8e25acbf1e1a422eb4cd104e95ee4a681021a6bb7456afa",
		},
		{
			blockNum: 35749,
			expected: "0x323feeef51cadc14d4a025eb541227b177f69d1e6052854de262ca5e18055a1",
		},
		{
			blockNum: 38748,
			expected: "0x2bb5df3dccd80b8eb8ad3f759b0ba045d467a79f032605d35380c87f8e730be",
		},
	} {
		t.Run(fmt.Sprintf("blockNum_%d", test.blockNum), func(t *testing.T) {
			su, err := gw.StateUpdate(context.Background(), test.blockNum)
			require.NoError(t, err)
			assert.Equal(t, utils.HexToFelt(t, test.expected), su.StateDiff.Hash())
		})
	}
}

func TestStateDiffLength(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	for _, test := range []struct {
		blockNum       uint64
		expectedLength uint64
	}{
		{blockNum: 0, expectedLength: 11},
		{blockNum: 1, expectedLength: 1},
		{blockNum: 2, expectedLength: 1},
	} {
		t.Run(fmt.Sprintf("blockNum=%d", test.blockNum), func(t *testing.T) {
			su, err := gw.StateUpdate(context.Background(), test.blockNum)
			require.NoError(t, err)
			length := su.StateDiff.Length()
			assert.Equal(t, test.expectedLength, length)
		})
	}
}
