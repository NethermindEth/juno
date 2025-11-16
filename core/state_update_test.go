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

func TestStateDiffCommitment(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)
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
			assert.Equal(t, felt.UnsafeFromString[felt.Felt](test.expected), commitment)
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
		{
			blockNum: 3077642,
			expected: "0x31dc9b20993d7256c0b407c7f36d9cb77b7d44788b40f40c770785b1ba421c9",
		},
	} {
		t.Run(fmt.Sprintf("blockNum_%d", test.blockNum), func(t *testing.T) {
			su, err := gw.StateUpdate(t.Context(), test.blockNum)
			require.NoError(t, err)
			assert.Equal(t, felt.UnsafeFromString[felt.Felt](test.expected), su.StateDiff.Hash())
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

func TestStateDiffMerge_StorageDiffsDifferentSlotsSameAddress(t *testing.T) {
	addrA := new(felt.Felt).SetUint64(255)
	slotX := new(felt.Felt).SetUint64(127)
	slotY := new(felt.Felt).SetUint64(255)
	v1 := new(felt.Felt).SetUint64(1)
	v2 := new(felt.Felt).SetUint64(2)

	d := core.EmptyStateDiff()

	incomingV1 := core.EmptyStateDiff()
	incomingV1.StorageDiffs[*addrA] = map[felt.Felt]*felt.Felt{*slotX: v1}

	d.Merge(&incomingV1)

	incomingV2 := core.EmptyStateDiff()
	incomingV2.StorageDiffs[*addrA] = map[felt.Felt]*felt.Felt{*slotY: v2}

	d.Merge(&incomingV2)

	inner, ok := d.StorageDiffs[*addrA]
	require.True(t, ok)
	require.Len(t, inner, 2)
	require.Equal(t, inner[*slotX], v1)
	require.Equal(t, inner[*slotY], v2)
	// incoming should remain unchanged
	innerV1 := incomingV1.StorageDiffs[*addrA]
	require.Len(t, innerV1, 1)
	require.Equal(t, innerV1[*slotX], v1)
	innerV2 := incomingV2.StorageDiffs[*addrA]
	require.Len(t, innerV2, 1)
	require.Equal(t, innerV2[*slotY], v2)
}

func TestStateDiffMerge_StorageDiffsSameSlotIncomingWins(t *testing.T) {
	addrA := new(felt.Felt).SetUint64(255)
	slotX := new(felt.Felt).SetUint64(127)
	v1 := new(felt.Felt).SetUint64(1)
	v2 := new(felt.Felt).SetUint64(2)

	d := core.EmptyStateDiff()

	incomingV1 := core.EmptyStateDiff()
	incomingV1.StorageDiffs[*addrA] = map[felt.Felt]*felt.Felt{*slotX: v1}

	d.Merge(&incomingV1)

	incomingV2 := core.EmptyStateDiff()
	incomingV2.StorageDiffs[*addrA] = map[felt.Felt]*felt.Felt{*slotX: v2}

	d.Merge(&incomingV2)

	inner := d.StorageDiffs[*addrA]
	require.Len(t, inner, 1)
	require.Equal(t, inner[*slotX], v2)
	// incoming unchanged
	require.Equal(t, incomingV1.StorageDiffs[*addrA][*slotX], v1)
	require.Equal(t, incomingV2.StorageDiffs[*addrA][*slotX], v2)
}
