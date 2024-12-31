package trie2

import (
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestUpdate(t *testing.T) {
	tr, records := nonRandomTrie(t, 1000)

	for _, record := range records {
		err := tr.Update(record.key, record.value)
		require.NoError(t, err)

		got, err := tr.Get(record.key)
		require.NoError(t, err)
		require.Equal(t, record.value, got)
	}
}

func TestUpdateRandom(t *testing.T) {
	tr, records := randomTrie(t, 1000)

	for _, record := range records {
		got, err := tr.Get(record.key)
		require.NoError(t, err)

		if !got.Equal(record.value) {
			t.Fatalf("expected %s, got %s", record.value, got)
		}
	}
}

func TestDelete(t *testing.T) {
	tr, records := nonRandomTrie(t, 10000)

	for _, record := range records {
		err := tr.Delete(record.key)
		require.NoError(t, err)

		got, err := tr.Get(record.key)
		require.NoError(t, err)
		require.Equal(t, got, &felt.Zero)
	}
}

func TestDeleteRandom(t *testing.T) {
	tr, records := randomTrie(t, 10000)

	for i := len(records) - 1; i >= 0; i-- {
		err := tr.Delete(records[i].key)
		require.NoError(t, err)

		got, err := tr.Get(records[i].key)
		require.NoError(t, err)
		require.Equal(t, got, &felt.Zero)
	}
}

type keyValue struct {
	key   *felt.Felt
	value *felt.Felt
}

func nonRandomTrie(t *testing.T, numKeys int) (*Trie, []*keyValue) {
	tr := NewTrie(251)
	records := make([]*keyValue, numKeys)

	for i := 1; i < numKeys+1; i++ {
		key := new(felt.Felt).SetUint64(uint64(i))
		records[i-1] = &keyValue{key: key, value: key}
		err := tr.Update(key, key)
		require.NoError(t, err)
	}

	return tr, records
}

func randomTrie(t testing.TB, n int) (*Trie, []*keyValue) {
	rrand := rand.New(rand.NewSource(3))

	tr := NewTrie(251)
	records := make([]*keyValue, n)

	for i := 0; i < n; i++ {
		key := new(felt.Felt).SetUint64(uint64(rrand.Uint32() + 1))
		records[i] = &keyValue{key: key, value: key}
		err := tr.Update(key, key)
		require.NoError(t, err)
	}

	return tr, records
}

func build4KeysTrieD(t *testing.T) (*Trie, []*keyValue) {
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(4)},
		{key: new(felt.Felt).SetUint64(4), value: new(felt.Felt).SetUint64(5)},
		{key: new(felt.Felt).SetUint64(6), value: new(felt.Felt).SetUint64(6)},
		{key: new(felt.Felt).SetUint64(7), value: new(felt.Felt).SetUint64(7)},
	}

	return buildTrie(t, records), records
}

func buildTrie(t *testing.T, records []*keyValue) *Trie {
	if len(records) == 0 {
		t.Fatal("records must have at least one element")
	}

	tempTrie := NewTrie(251)

	for _, record := range records {
		err := tempTrie.Update(record.key, record.value)
		require.NoError(t, err)
	}

	return tempTrie
}
