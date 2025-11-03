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
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db/memory"
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
		hash, _ := tr.Hash()
		require.Equal(t, felt.Zero, hash)
	})

	t.Run("one leaf", func(t *testing.T) {
		tr, _ := NewEmptyPedersen()
		err := tr.Update(new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(2))
		require.NoError(t, err)
		hash, _ := tr.Hash()

		expected := "0x2ab889bd35e684623df9b4ea4a4a1f6d9e0ef39b67c1293b8a89dd17e351330"
		require.Equal(t, expected, hash.String(), "expected %s, got %s", expected, hash.String())
	})

	t.Run("two leaves", func(t *testing.T) {
		tr, _ := NewEmptyPedersen()
		err := tr.Update(new(felt.Felt).SetUint64(0), new(felt.Felt).SetUint64(2))
		require.NoError(t, err)
		err = tr.Update(new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(3))
		require.NoError(t, err)
		root, _ := tr.Hash()

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
		root, _ := tr.Hash()
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
		root, _ := tr.Hash()
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
			felt.NewUnsafeFromString[felt.Felt]("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
		}

		vals := []*felt.Felt{
			felt.NewUnsafeFromString[felt.Felt]("0xcc"),
			felt.NewUnsafeFromString[felt.Felt]("0xdd"),
		}

		tr, _ := NewEmptyPedersen()
		for i := range keys {
			err := tr.Update(keys[i], vals[i])
			require.NoError(t, err)
		}

		expected := "0x542ced3b6aeef48339129a03e051693eff6a566d3a0a94035fa16ab610dc9e2"
		root, _ := tr.Hash()
		require.Equal(t, expected, root.String(), "expected %s, got %s", expected, root.String())
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
	// Create keys from the failing test case
	key1 := felt.NewUnsafeFromString[felt.Felt]("0x4b004d0b47e75a025540ea685e15b5")

	steps := []randTestStep{
		{op: opUpdate, key: key1, value: new(felt.Felt).SetUint64(6)},
		{op: opCommit},
		{op: opGet, key: key1}, // This is where the original failure occurred
	}

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
		_, _ = r.Read(random)
		// Create a new key with 10% probability or when < 2 keys exist
		if len(allKeys) < 2 || random[0]%100 > 90 {
			size := random[0] % 32 // ensure key size is between 1 and 32 bytes
			key := make([]byte, size)
			_, _ = r.Read(key)
			allKeys = append(allKeys, new(felt.Felt).SetBytes(key))
		}
		// 90% probability to return an existing key
		idx := int(random[0]) % len(allKeys)
		return allKeys[idx]
	}

	var steps randTest
	for !finished() {
		_, _ = r.Read(random)
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

//nolint:gocyclo
func runRandTest(rt randTest) error {
	for _, scheme := range []dbScheme{PathScheme, HashScheme} {
		db := memory.New()
		curRoot := felt.Zero
		trieDB := NewTestNodeDatabase(db, scheme)
		tr, err := New(trieutils.NewContractTrieID(curRoot), contractClassTrieHeight, crypto.Pedersen, &trieDB)
		if err != nil {
			return err
		}

		values := make(map[felt.Felt]felt.Felt) // keeps track of the content of the trie

		for i, step := range rt {
			switch step.op {
			case opUpdate:
				err := tr.Update(step.key, step.value)
				if err != nil {
					rt[i].err = fmt.Errorf("update failed: key %s, %w", step.key.String(), err)
				}
				values[*step.key] = *step.value
			case opDelete:
				err := tr.Delete(step.key)
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
				hash, _ := tr.Hash()
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
				_, err := tr.Hash()
				if err != nil {
					rt[i].err = fmt.Errorf("hash failed: %w", err)
				}
			case opCommit:
				root, nodes := tr.Commit()
				if nodes != nil {
					if err := trieDB.Update(&root, &curRoot, trienode.NewMergeNodeSet(nodes)); err != nil {
						rt[i].err = fmt.Errorf("update failed: %w", err)
					}
				}

				newtr, err := New(trieutils.NewContractTrieID(root), contractClassTrieHeight, crypto.Pedersen, &trieDB)
				if err != nil {
					rt[i].err = fmt.Errorf("new trie failed: %w", err)
				}
				tr = newtr
				curRoot = root
			}

			if rt[i].err != nil {
				return rt[i].err
			}
		}
	}
	return nil
}

type keyValue struct {
	key   *felt.Felt
	value *felt.Felt
}

// Inserts keys in sequential order, so that it's easier to know the trie structure in advance
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

// Inserts keys in random order, making tests more robust and realistic
func randomTrie(t testing.TB, n int) (*Trie, []*keyValue) {
	rrand := rand.New(rand.NewSource(3))

	tr, _ := NewEmptyPedersen()
	records := make([]*keyValue, n)

	for i := range n {
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
