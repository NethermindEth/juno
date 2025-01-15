package trie2

import (
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestUpdate(t *testing.T) {
	verifyRecords := func(t *testing.T, tr *Trie, records []*keyValue) {
		t.Helper()
		for _, record := range records {
			got, err := tr.Get(record.key)
			require.NoError(t, err)
			require.True(t, got.Equal(record.value), "expected %v, got %v", record.value, got)
		}
	}

	t.Run("sequential", func(t *testing.T) {
		tr, records := nonRandomTrie(t, 10000)
		verifyRecords(t, tr, records)
	})

	t.Run("random", func(t *testing.T) {
		tr, records := randomTrie(t, 10000)
		verifyRecords(t, tr, records)
	})
}

func TestDelete(t *testing.T) {
	verifyDelete := func(t *testing.T, tr *Trie, records []*keyValue) {
		t.Helper()
		for _, record := range records {
			err := tr.Delete(record.key)
			require.NoError(t, err)

			got, err := tr.Get(record.key)
			require.NoError(t, err)
			require.True(t, got.Equal(&felt.Zero), "expected %v, got %v", &felt.Zero, got)
		}
	}

	t.Run("sequential", func(t *testing.T) {
		tr, records := nonRandomTrie(t, 10000)
		verifyDelete(t, tr, records)
	})

	t.Run("random", func(t *testing.T) {
		tr, records := randomTrie(t, 10000)
		// Delete in reverse order for random case
		for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
			records[i], records[j] = records[j], records[i]
		}
		verifyDelete(t, tr, records)
	})
}

// The expected hashes are taken from Pathfinder's tests
func TestHash(t *testing.T) {
	t.Run("one leaf", func(t *testing.T) {
		tr, _ := NewEmptyPedersen()
		err := tr.Update(new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(2))
		require.NoError(t, err)
		hash := tr.Hash()

		expected := "0x2ab889bd35e684623df9b4ea4a4a1f6d9e0ef39b67c1293b8a89dd17e351330"
		require.Equal(t, expected, hash.String(), "expected %s, got %s", expected, hash.String())
	})

	t.Run("two leaves", func(t *testing.T) {
		tr, _ := NewEmptyPedersen()
		err := tr.Update(new(felt.Felt).SetUint64(0), new(felt.Felt).SetUint64(2))
		require.NoError(t, err)
		err = tr.Update(new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(3))
		require.NoError(t, err)
		root := tr.Hash()

		expected := "0x79acdb7a3d78052114e21458e8c4aecb9d781ce79308193c61a2f3f84439f66"
		require.Equal(t, expected, root.String(), "expected %s, got %s", expected, root.String())
	})

	t.Run("three leaves", func(t *testing.T) {
		tr, _ := NewEmptyPedersen()

		keys := []*felt.Felt{
			new(felt.Felt).SetUint64(16),
			new(felt.Felt).SetUint64(17),
			new(felt.Felt).SetUint64(19),
		}

		vals := []*felt.Felt{
			new(felt.Felt).SetUint64(10),
			new(felt.Felt).SetUint64(11),
			new(felt.Felt).SetUint64(12),
		}

		for i := range keys {
			err := tr.Update(keys[i], vals[i])
			require.NoError(t, err)
		}

		expected := "0x7e2184e9e1a651fd556b42b4ff10e44a71b1709f641e0203dc8bd2b528e5e81"
		root := tr.Hash()
		require.Equal(t, expected, root.String(), "expected %s, got %s", expected, root.String())
	})

	t.Run("double binary", func(t *testing.T) {
		//           (249,0,x3)
		//               |
		//           (0, 0, x3)
		//         /            \
		//     (0,0,x1)       (1, 1, 5)
		//      /    \             |
		//     (2)  (3)           (5)

		keys := []*felt.Felt{
			new(felt.Felt).SetUint64(0),
			new(felt.Felt).SetUint64(1),
			new(felt.Felt).SetUint64(3),
		}

		vals := []*felt.Felt{
			new(felt.Felt).SetUint64(2),
			new(felt.Felt).SetUint64(3),
			new(felt.Felt).SetUint64(5),
		}

		tr, _ := NewEmptyPedersen()
		for i := range keys {
			err := tr.Update(keys[i], vals[i])
			require.NoError(t, err)
		}

		expected := "0x6a316f09913454294c6b6751dea8449bc2e235fdc04b2ab0e1ac7fea25cc34f"
		root := tr.Hash()
		require.Equal(t, expected, root.String(), "expected %s, got %s", expected, root.String())
	})

	t.Run("binary root", func(t *testing.T) {
		//           (0, 0, x)
		//    /                    \
		// (250, 0, cc)     (250, 11111.., dd)
		//    |                     |
		//   (cc)                  (dd)

		keys := []*felt.Felt{
			new(felt.Felt).SetUint64(0),
			utils.HexToFelt(t, "0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
		}

		vals := []*felt.Felt{
			utils.HexToFelt(t, "0xcc"),
			utils.HexToFelt(t, "0xdd"),
		}

		tr, _ := NewEmptyPedersen()
		for i := range keys {
			err := tr.Update(keys[i], vals[i])
			require.NoError(t, err)
		}

		expected := "0x542ced3b6aeef48339129a03e051693eff6a566d3a0a94035fa16ab610dc9e2"
		root := tr.Hash()
		require.Equal(t, expected, root.String(), "expected %s, got %s", expected, root.String())
	})
}

func TestCommit(t *testing.T) {
	t.Run("sequential", func(t *testing.T) {
		tr, _ := nonRandomTrie(t, 10000)

		_, err := tr.Commit()
		require.NoError(t, err)
	})

	t.Run("random", func(t *testing.T) {
		tr, _ := randomTrie(t, 10000)

		_, err := tr.Commit()
		require.NoError(t, err)
	})
}

func TestTrieOpsRandom(t *testing.T) {
	t.Skip()
	panic("implement me")
}

type keyValue struct {
	key   *felt.Felt
	value *felt.Felt
}

func nonRandomTrie(t *testing.T, numKeys int) (*Trie, []*keyValue) {
	tr, _ := NewEmptyPedersen()
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

	tr, _ := NewEmptyPedersen()
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

	tempTrie, _ := NewEmptyPedersen()

	for _, record := range records {
		err := tempTrie.Update(record.key, record.value)
		require.NoError(t, err)
	}

	return tempTrie
}
