package trie2

import (
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"testing/quick"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/davecgh/go-spew/spew"
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
	t.Run("empty", func(t *testing.T) {
		tr, _ := NewEmptyPedersen()
		hash := tr.Hash()
		require.Equal(t, felt.Zero, hash)
	})

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

func TestMissingRoot(t *testing.T) {
	var root felt.Felt
	root.SetUint64(1)

	tr, err := New(TrieID(root), contractClassTrieHeight, crypto.Pedersen, db.NewMemTransaction())
	require.Nil(t, tr)
	require.Error(t, err)
}

func TestCommit(t *testing.T) {
	verifyCommit := func(t *testing.T, records []*keyValue) {
		t.Helper()
		db := db.NewMemTransaction()
		tr, err := New(TrieID(felt.Zero), contractClassTrieHeight, crypto.Pedersen, db)
		require.NoError(t, err)

		for _, record := range records {
			err := tr.Update(record.key, record.value)
			require.NoError(t, err)
		}

		root, err := tr.Commit()
		require.NoError(t, err)

		tr2, err := New(TrieID(root), contractClassTrieHeight, crypto.Pedersen, db)
		require.NoError(t, err)

		for _, record := range records {
			got, err := tr2.Get(record.key)
			require.NoError(t, err)
			require.True(t, got.Equal(record.value), "expected %v, got %v", record.value, got)
		}
	}

	t.Run("sequential", func(t *testing.T) {
		_, records := nonRandomTrie(t, 10000)
		verifyCommit(t, records)
	})

	t.Run("random", func(t *testing.T) {
		_, records := randomTrie(t, 10000)
		verifyCommit(t, records)
	})
}

func TestRandom(t *testing.T) {
	if err := quick.Check(runRandTestBool, nil); err != nil {
		if cerr, ok := err.(*quick.CheckError); ok {
			t.Fatalf("random test iteration %d failed: %s", cerr.Count, spew.Sdump(cerr.In))
		}
		t.Fatal(err)
	}
}

func TestSpecificRandomFailure(t *testing.T) {
	// Create test steps that match the failing sequence
	key1 := utils.HexToFelt(t, "0x5c0d77d04056ae1e0c49bce8a3223b0373ccf94c1b04935d")
	key2 := utils.HexToFelt(t, "0x19bc6d500358f3046b0743c903")
	key3 := utils.HexToFelt(t, "0xf7d2180a5a138b325ea522e2b65b58a5dffb859b1fbbdab0e5efb125e8a840")

	steps := []randTestStep{
		{op: opProve, key: key1},
		{op: opCommit},
		{op: opDelete, key: key2},
		{op: opHash},
		{op: opCommit},
		{op: opCommit},
		{op: opGet, key: key2},
		{op: opUpdate, key: key2, value: new(felt.Felt).SetUint64(7)},
		{op: opHash},
		{op: opDelete, key: key1},
		{op: opUpdate, key: key1, value: new(felt.Felt).SetUint64(10)},
		{op: opHash},
		{op: opGet, key: key2},
		{op: opUpdate, key: key1, value: new(felt.Felt).SetUint64(13)},
		{op: opCommit},
		{op: opHash},
		{op: opProve, key: key1},
		{op: opProve, key: key3},
		{op: opUpdate, key: key3, value: new(felt.Felt).SetUint64(18)},
		{op: opCommit},
		{op: opGet, key: key2},
		{op: opUpdate, key: key2, value: new(felt.Felt).SetUint64(21)},
		{op: opHash},
		{op: opHash},
		{op: opGet, key: key2},
		{op: opProve, key: key1},
		{op: opUpdate, key: key2, value: new(felt.Felt).SetUint64(26)},
		{op: opHash},
		{op: opGet, key: key2},
		{op: opCommit},
		{op: opCommit},
		{op: opProve, key: key3}, // This is where the original failure occurred
	}

	// Add debug logging
	t.Log("Starting test sequence")
	err := runRandTest(steps)
	if err != nil {
		t.Logf("Test failed at step: %v", err)
		// Print the state of the trie at failure
		for i, step := range steps {
			if step.err != nil {
				t.Logf("Failed at step %d: %v", i, step)
				break
			}
		}
	}
	require.NoError(t, err, "specific random test sequence should not fail")
}

const (
	opUpdate = iota
	opDelete
	opGet
	opHash
	opCommit
	opProve
	opMax // max number of operations, not an actual operation
)

type randTestStep struct {
	op    int
	key   *felt.Felt // for opUpdate, opDelete, opGet
	value *felt.Felt // for opUpdate
	err   error
}

type randTest []randTestStep

func (randTest) Generate(r *rand.Rand, size int) reflect.Value {
	finishedFn := func() bool {
		size--
		return size == 0
	}
	return reflect.ValueOf(generateSteps(finishedFn, r))
}

func generateSteps(finished func() bool, r io.Reader) randTest {
	var allKeys []*felt.Felt
	random := []byte{0}

	genKey := func() *felt.Felt {
		r.Read(random)
		// Create a new key with 10% probability or when < 2 keys exist
		if len(allKeys) < 2 || random[0]%100 > 90 {
			size := random[0] % 32 // ensure key size is between 1 and 32 bytes
			key := make([]byte, size)
			r.Read(key)
			allKeys = append(allKeys, new(felt.Felt).SetBytes(key))
		}
		// 90% probability to return an existing key
		idx := int(random[0]) % len(allKeys)
		return allKeys[idx]
	}

	var steps randTest
	for !finished() {
		r.Read(random)
		step := randTestStep{op: int(random[0]) % opMax}
		switch step.op {
		case opUpdate:
			step.key = genKey()
			step.value = new(felt.Felt).SetUint64(uint64(len(steps)))
		case opGet, opDelete, opProve:
			step.key = genKey()
		}
		steps = append(steps, step)
	}
	return steps
}

func runRandTestBool(rt randTest) bool {
	return runRandTest(rt) == nil
}

func runRandTest(rt randTest) error {
	txn := db.NewMemTransaction()
	tr, err := New(TrieID(felt.Zero), contractClassTrieHeight, crypto.Pedersen, txn)
	if err != nil {
		return err
	}

	values := make(map[felt.Felt]felt.Felt) // keeps track of the content of the trie

	for i, step := range rt {
		// fmt.Printf("Step %d: %d key=%s value=%s\n", i, step.op, step.key, step.value)
		switch step.op {
		case opUpdate:
			err := tr.Update(step.key, step.value)
			// fmt.Println("--------------------------------")
			// fmt.Println(tr.root.String())
			if err != nil {
				rt[i].err = fmt.Errorf("update failed: key %s, %w", step.key.String(), err)
			}
			values[*step.key] = *step.value
		case opDelete:
			err := tr.Delete(step.key)
			// fmt.Println("--------------------------------")
			// if tr.root != nil {
			// 	fmt.Println("trie", tr.root.String())
			// } else {
			// 	fmt.Println("nil root")
			// }
			if err != nil {
				rt[i].err = fmt.Errorf("delete failed: key %s, %w", step.key.String(), err)
			}
			delete(values, *step.key)
		case opGet:
			got, err := tr.Get(step.key)
			if err != nil {
				rt[i].err = fmt.Errorf("get failed: key %s, %w", step.key.String(), err)
			}
			want := values[*step.key]
			if !got.Equal(&want) {
				rt[i].err = fmt.Errorf("mismatch in get: key %s, expected %v, got %v", step.key.String(), want.String(), got.String())
			}
		case opProve:
			hash := tr.Hash()
			if hash.Equal(&felt.Zero) {
				continue
			}
			proof := NewProofNodeSet()
			err := tr.Prove(step.key, proof)
			if err != nil {
				rt[i].err = fmt.Errorf("prove failed for key %s: %w", step.key.String(), err)
			}
			_, err = VerifyProof(&hash, step.key, proof, crypto.Pedersen)
			if err != nil {
				rt[i].err = fmt.Errorf("verify proof failed for key %s: %w", step.key.String(), err)
			}
		case opHash:
			tr.Hash()
		case opCommit:
			root, err := tr.Commit()
			if err != nil {
				rt[i].err = fmt.Errorf("commit failed: %w", err)
			}
			newtr, err := New(TrieID(root), contractClassTrieHeight, crypto.Pedersen, txn)
			// fmt.Println("--------------------------------")
			// if newtr.root != nil {
			// 	fmt.Println(newtr.root.String())
			// } else {
			// 	fmt.Println("nil root")
			// }
			if err != nil {
				rt[i].err = fmt.Errorf("new trie failed: %w", err)
			}
			tr = newtr
		}

		if rt[i].err != nil {
			return rt[i].err
		}
	}
	return nil
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

	// Sort records by key
	sort.Slice(records, func(i, j int) bool {
		return records[i].key.Cmp(records[j].key) < 0
	})

	return tr, records
}

func build4KeysTrieD(t *testing.T) (*Trie, []*keyValue) {
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(4)},
		{key: new(felt.Felt).SetUint64(4), value: new(felt.Felt).SetUint64(5)},
		{key: new(felt.Felt).SetUint64(6), value: new(felt.Felt).SetUint64(6)},
		{key: new(felt.Felt).SetUint64(7), value: new(felt.Felt).SetUint64(7)},
	}

	return buildTestTrie(t, records), records
}

func buildTestTrie(t *testing.T, records []*keyValue) *Trie {
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
