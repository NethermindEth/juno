package rpccore_test

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/stretchr/testify/require"
)

func TestPreConfirmedDeduper(t *testing.T) {
	key1 := "key1"
	key2 := "key2"
	const id1, id2 = "round-1", "round-2"

	t.Run("dedups within the same round", func(t *testing.T) {
		d := rpccore.NewPreConfirmedDeduper[string]()

		require.True(t, d.MarkSent(100, id1, &key1))  // first send
		require.False(t, d.MarkSent(100, id1, &key1)) // duplicate in same round
		require.True(t, d.MarkSent(100, id1, &key2))  // unseen key
	})

	t.Run("advancing the block discards the previous tip", func(t *testing.T) {
		d := rpccore.NewPreConfirmedDeduper[string]()

		require.True(t, d.MarkSent(100, id1, &key1))
		require.True(t, d.MarkSent(101, id1, &key2)) // new block, clears block 100
		// block 100 is no longer tracked, so its key is forgotten
		require.True(t, d.MarkSent(100, id1, &key1))
	})

	t.Run("a new round identifier at the same height re-emits", func(t *testing.T) {
		d := rpccore.NewPreConfirmedDeduper[string]()

		require.True(t, d.MarkSent(100, id1, &key1))
		require.False(t, d.MarkSent(100, id1, &key1)) // same round, deduped
		// same height, replaced round: identifier changed, so the key is re-sent
		require.True(t, d.MarkSent(100, id2, &key1))
		require.False(t, d.MarkSent(100, id2, &key1)) // now deduped within the new round
	})

	t.Run("Clear forgets the current tip", func(t *testing.T) {
		d := rpccore.NewPreConfirmedDeduper[string]()

		require.True(t, d.MarkSent(100, id1, &key1))
		require.False(t, d.MarkSent(100, id1, &key1))
		d.Clear()
		require.True(t, d.MarkSent(100, id1, &key1))
	})

	t.Run("block zero behaves like any other block", func(t *testing.T) {
		d := rpccore.NewPreConfirmedDeduper[string]()

		require.True(t, d.MarkSent(0, id1, &key1))
		require.False(t, d.MarkSent(0, id1, &key1))
		require.True(t, d.MarkSent(1, id1, &key1)) // advance off block 0
		require.True(t, d.MarkSent(0, id1, &key1)) // block 0 forgotten
	})
}

func BenchmarkPreConfirmedDeduper_MarkSent(b *testing.B) {
	d := rpccore.NewPreConfirmedDeduper[string]()
	const id = "round"
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		key := strconv.Itoa(i % 256)
		d.MarkSent(uint64(i/64), id, &key) // advance tip every 64 inserts
	}
}
