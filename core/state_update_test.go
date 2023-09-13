package core_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateDiffCommitment(t *testing.T) {
	client := feeder.NewTestClient(t, utils.INTEGRATION)
	gw := adaptfeeder.New(client)

	for _, test := range []struct {
		blockNum uint64
		expected string
	}{
		{
			blockNum: 0,
			expected: "0x689f2581f7956808e3e5589e8d984290a5d362a187111ffc3f6ef9fe839149e",
		},
		{
			blockNum: 283364,
			expected: "0x85894cd1d031ed87d7cba9b5eebd44beb7e9ec5b578d19e052844f4a2561ee",
		},
		{
			blockNum: 283428,
			expected: "0x1277857da9a95a3eb65bb2d4b1d9749adb6916cb460d1ab2fcc622ee7cf78f5",
		},
		{
			blockNum: 283746,
			expected: "0x32a531da56a82f993a29b3cfe4102b1589ddbc64bfd7be24706ab2b5ac2dba5",
		},
	} {
		su, err := gw.StateUpdate(context.Background(), test.blockNum)
		require.NoError(t, err)
		commitment := su.StateDiff.Commitment()
		assert.Equal(t, utils.HexToFelt(t, test.expected), commitment)
	}
}
