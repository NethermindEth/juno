package core_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateDiffCommitment(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
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
		t.Run(fmt.Sprintf("blockNum=%d", test.blockNum), func(t *testing.T) {
			su, err := gw.StateUpdate(t.Context(), test.blockNum)
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
			su, err := gw.StateUpdate(t.Context(), test.blockNum)
			require.NoError(t, err)
			assert.Equal(t, utils.HexToFelt(t, test.expected), su.StateDiff.Hash())
		})
	}
}

func BenchmarkStateDiffHash(b *testing.B) {
	client := feeder.NewTestClient(b, &utils.SepoliaIntegration)
	gw := adaptfeeder.New(client)
	su, err := gw.StateUpdate(b.Context(), 38748)
	require.NoError(b, err)

	b.ResetTimer()
	for range b.N {
		su.StateDiff.Hash()
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
			su, err := gw.StateUpdate(t.Context(), test.blockNum)
			require.NoError(t, err)
			length := su.StateDiff.Length()
			assert.Equal(t, test.expectedLength, length)
		})
	}
}
