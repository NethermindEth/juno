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
		t.Run("#"+fmt.Sprint(test.blockNum), func(t *testing.T) {
			su, err := gw.StateUpdate(context.Background(), test.blockNum)
			require.NoError(t, err)
			commitment := su.StateDiff.Commitment()
			assert.Equal(t, utils.HexToFelt(t, test.expected), commitment)
		})
	}
}
