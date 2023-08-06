package trie_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/NethermindEth/juno/core/crypto"
	"golang.org/x/exp/rand"
	"math"
	"math/big"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Todo: Refactor:
//
//   - [*] Test names should not have "_"
//   - [*] Table test are being used incorrectly: they should be separated into subsets, see node_test.go
//   - [*] Functions such as Path and findCommonKey don't need to be public. Thus,
//     they don't need to be tested explicitly.
//   - [*] There are warning which ignore returned errors, returned errors should not be ignored.
//   - [ ] Add more test cases with different heights
//   - [*] Add more complicated Put and Delete scenarios
func TestTriePut(t *testing.T) {
	t.Run("put zero to empty trie", func(t *testing.T) {
		require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
			key := new(felt.Felt).SetUint64(1)
			zeroVal := new(felt.Felt).SetUint64(0)

			oldVal, err := tempTrie.Put(key, zeroVal)
			require.NoError(t, err)

			assert.Nil(t, oldVal)

			return nil
		}))
	})

	t.Run("put zero value", func(t *testing.T) {
		require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
			keyNum, err := strconv.ParseUint("1101", 2, 64)
			require.NoError(t, err)

			key := new(felt.Felt).SetUint64(keyNum)
			zeroVal := new(felt.Felt).SetUint64(0)

			_, err = tempTrie.Put(key, zeroVal)
			require.NoError(t, err)

			value, err := tempTrie.Get(key)
			assert.NoError(t, err)
			assert.Equal(t, &felt.Zero, value)
			// Trie's root should be nil
			assert.Nil(t, tempTrie.RootKey())

			return nil
		}))
	})

	t.Run("put to replace an existed value", func(t *testing.T) {
		require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
			keyNum, err := strconv.ParseUint("1101", 2, 64)
			require.NoError(t, err)

			key := new(felt.Felt).SetUint64(keyNum)
			val := new(felt.Felt).SetUint64(1)

			_, err = tempTrie.Put(key, val)
			require.NoError(t, err)

			newVal := new(felt.Felt).SetUint64(2)

			_, err = tempTrie.Put(key, newVal)
			require.NoError(t, err, "update a new value at an exist key")

			value, err := tempTrie.Get(key)
			require.NoError(t, err)

			assert.Equal(t, newVal, value)

			return nil
		}))
	})
}

func TestTrieDeleteBasic(t *testing.T) {
	// left branch
	leftKeyNum, err := strconv.ParseUint("100", 2, 64)
	require.NoError(t, err)

	leftKey := new(felt.Felt).SetUint64(leftKeyNum)
	leftVal := new(felt.Felt).SetUint64(12)

	// right branch
	rightKeyNum, err := strconv.ParseUint("111", 2, 64)
	require.NoError(t, err)

	rightKey := new(felt.Felt).SetUint64(rightKeyNum)
	rightVal := new(felt.Felt).SetUint64(22)

	// Zero value
	zeroVal := new(felt.Felt).SetUint64(0)

	tests := [...]struct {
		name          string
		deleteKeys    []*felt.Felt
		expectRootKey *felt.Felt
	}{
		{
			name:          "delete left child",
			deleteKeys:    []*felt.Felt{leftKey},
			expectRootKey: rightKey,
		},
		{
			name:          "delete right child",
			deleteKeys:    []*felt.Felt{rightKey},
			expectRootKey: leftKey,
		},
		{
			name:          "delete both children",
			deleteKeys:    []*felt.Felt{leftKey, rightKey},
			expectRootKey: (*felt.Felt)(nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
				// Build a basic trie
				_, err := tempTrie.Put(leftKey, leftVal)
				require.NoError(t, err)

				_, err = tempTrie.Put(rightKey, rightVal)
				require.NoError(t, err)

				for _, key := range test.deleteKeys {
					_, err := tempTrie.Put(key, zeroVal)
					require.NoError(t, err)

					val, err := tempTrie.Get(key)

					assert.NoError(t, err, "shouldnt return an error when access a deleted key")
					assert.Equal(t, &felt.Zero, val, "should return zero value when access a deleted key")
				}

				// Check the final rootKey

				if test.expectRootKey != nil {
					expectKeyBytes := test.expectRootKey.Bits()
					assert.Equal(t, expectKeyBytes[:], tempTrie.RootKey().Bytes())
				} else {
					assert.Nil(t, tempTrie.RootKey())
				}

				return nil
			}))
		})
	}
}

func TestPutZero(t *testing.T) {
	require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
		emptyRoot, err := tempTrie.Root()
		require.NoError(t, err)
		var roots []*felt.Felt
		var keys []*felt.Felt

		// put random 64 keys and record roots
		for i := 0; i < 64; i++ {
			key, value := new(felt.Felt), new(felt.Felt)

			_, err = key.SetRandom()
			require.NoError(t, err)

			_, err = value.SetRandom()
			require.NoError(t, err)

			_, err = tempTrie.Put(key, value)
			require.NoError(t, err)

			keys = append(keys, key)

			var root *felt.Felt
			root, err = tempTrie.Root()
			require.NoError(t, err)

			roots = append(roots, root)
		}

		t.Run("adding a zero value to a non-existent key should not change Trie", func(t *testing.T) {
			var key, root *felt.Felt
			key, err = new(felt.Felt).SetRandom()
			require.NoError(t, err)

			_, err = tempTrie.Put(key, new(felt.Felt))
			require.NoError(t, err)

			root, err = tempTrie.Root()
			require.NoError(t, err)

			assert.Equal(t, true, root.Equal(roots[len(roots)-1]))
		})

		t.Run("remove keys one by one, check roots", func(t *testing.T) {
			var gotRoot *felt.Felt
			// put zero in reverse order and check roots still match
			for i := 0; i < 64; i++ {
				root := roots[len(roots)-1-i]

				gotRoot, err = tempTrie.Root()
				require.NoError(t, err)

				assert.Equal(t, root, gotRoot)

				key := keys[len(keys)-1-i]
				_, err = tempTrie.Put(key, new(felt.Felt))
				require.NoError(t, err)
			}
		})

		t.Run("empty roots should match", func(t *testing.T) {
			actualEmptyRoot, err := tempTrie.Root()
			require.NoError(t, err)

			assert.Equal(t, true, actualEmptyRoot.Equal(emptyRoot))
		})
		return nil
	}))
}

func TestOldData(t *testing.T) {
	require.NoError(t, trie.RunOnTempTrie(251, func(tempTrie *trie.Trie) error {
		key := new(felt.Felt).SetUint64(12)
		old := new(felt.Felt)

		t.Run("put zero to empty key, expect no change", func(t *testing.T) {
			was, err := tempTrie.Put(key, old)
			require.NoError(t, err)
			assert.Nil(t, was) // no change
		})

		t.Run("put non-zero to empty key, expect zero", func(t *testing.T) {
			was, err := tempTrie.Put(key, old)
			require.NoError(t, err)
			assert.Nil(t, was) // no change

			newVal := new(felt.Felt).SetUint64(1)
			was, err = tempTrie.Put(key, newVal)
			require.NoError(t, err)

			assert.Equal(t, old, was)
			old.Set(newVal)
		})

		t.Run("change value of a key, expect old value", func(t *testing.T) {
			newVal := new(felt.Felt).SetUint64(2)
			was, err := tempTrie.Put(key, newVal)
			require.NoError(t, err)

			assert.Equal(t, old, was)
			old.Set(newVal)
		})

		t.Run("delete key, expect old value", func(t *testing.T) {
			// put zero value to delete current key
			was, err := tempTrie.Put(key, &felt.Zero)
			require.NoError(t, err)

			assert.Equal(t, old, was)
		})

		t.Run("delete non-existent key, expect no change", func(t *testing.T) {
			// put zero again to check old data
			was, err := tempTrie.Put(key, new(felt.Felt))
			require.NoError(t, err)

			// there should no old data to return
			assert.Nil(t, was)
		})

		return nil
	}))
}

func TestMaxTrieHeight(t *testing.T) {
	t.Run("create trie with invalid height", func(t *testing.T) {
		assert.Error(t, trie.RunOnTempTrie(felt.Bits+1, func(_ *trie.Trie) error {
			return nil
		}))
	})

	t.Run("insert invalid key", func(t *testing.T) {
		height := uint64(felt.Bits)
		require.NoError(t, trie.RunOnTempTrie(uint(height), func(tt *trie.Trie) error {
			badKey := new(felt.Felt).Sub(&felt.Zero, new(felt.Felt).SetUint64(1))
			_, err := tt.Put(badKey, new(felt.Felt))
			assert.Error(t, err)
			return nil
		}))
	})
}

func TestRootKeyAlwaysUpdatedOnCommit(t *testing.T) {
	// Not doing what this test requires--always updating the root key on commit--
	// leads to some tricky errors. For example:
	//
	//  1. A trie is created and performs the following operations:
	//     a. Put leaf
	//     b. Commit
	//     c. Delete leaf
	//     d. Commit
	//  2. A second trie is created with the same db transaction and immediately
	//     calls [trie.Root].
	//
	// If the root key is not updated in the db transaction at step 1d,
	// the second trie will initialise its root key to the wrong value
	// (to the value the root key had at step 1b).

	// We simulate the situation described above.

	height := uint(251)

	// The database transaction we will use to create both tries.
	txn := db.NewMemTransaction()
	tTxn := trie.NewTransactionStorage(txn, []byte{1, 2, 3})

	// Step 1: Create first trie
	tempTrie, err := trie.NewTriePedersen(tTxn, height)
	require.NoError(t, err)

	// Step 1a: Put
	key := new(felt.Felt).SetUint64(1)
	_, err = tempTrie.Put(key, new(felt.Felt).SetUint64(1))
	require.NoError(t, err)

	// Step 1b: Commit
	require.NoError(t, tempTrie.Commit())

	// Step 1c: Delete
	_, err = tempTrie.Put(key, new(felt.Felt)) // Inserting zero felt is a deletion.
	require.NoError(t, err)

	want := new(felt.Felt)

	// Step 1d: Commit
	got, err := tempTrie.Root()
	require.NoError(t, err)
	// Ensure root value matches expectation.
	assert.Equal(t, want, got)

	// Step 2: Different trie created with the same db transaction and calls [trie.Root].
	tTxn = trie.NewTransactionStorage(txn, []byte{1, 2, 3})
	secondTrie, err := trie.NewTriePedersen(tTxn, height)
	require.NoError(t, err)
	got, err = secondTrie.Root()
	require.NoError(t, err)
	// Ensure root value is the same as the first trie.
	assert.Equal(t, want, got)
}

func BenchmarkTriePut(b *testing.B) {
	keys := make([]*felt.Felt, 0, b.N)
	for i := 0; i < b.N; i++ {
		rnd, err := new(felt.Felt).SetRandom()
		require.NoError(b, err)
		keys = append(keys, rnd)
	}

	one := new(felt.Felt).SetUint64(1)
	require.NoError(b, trie.RunOnTempTrie(251, func(t *trie.Trie) error {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := t.Put(keys[i], one)
			if err != nil {
				return err
			}
		}
		return t.Commit()
	}))
}

func numToFeltMul(num, multiplier int64) *felt.Felt {
	f := felt.Zero
	int1 := big.NewInt(int64(num))
	int2 := big.NewInt(int64(multiplier))

	return f.SetBigInt(int1.Mul(int1, int2))
}

func numToFelt(num int) *felt.Felt {
	return numToFeltBigInt(big.NewInt(int64(num)))
}

func numToFeltBigInt(num *big.Int) *felt.Felt {
	f := felt.Zero
	return f.SetBigInt(num)
}

func TestTrie_Iterate(t *testing.T) {
	db, err := pebble.NewMem()
	assert.Nil(t, err)

	trie, err := trie.NewTriePedersen(trie.NewTransactionStorage(db.NewTransaction(true), []byte{1}), 251)
	assert.Nil(t, err)

	for i := 0; i < 10; i++ {
		_, err = trie.Put(numToFelt(i), numToFelt(i+10))
		assert.Nil(t, err)
	}
	err = trie.Commit()
	assert.Nil(t, err)

	tests := []struct {
		name           string
		startKey       *felt.Felt
		count          int
		expectedKeys   []*felt.Felt
		expectedValues []*felt.Felt
	}{
		{
			name:     "all",
			startKey: numToFelt(0),
			count:    10,
			expectedKeys: []*felt.Felt{
				numToFelt(0),
				numToFelt(1),
				numToFelt(2),
				numToFelt(3),
				numToFelt(4),
				numToFelt(5),
				numToFelt(6),
				numToFelt(7),
				numToFelt(8),
				numToFelt(9),
			},
			expectedValues: []*felt.Felt{
				numToFelt(10),
				numToFelt(11),
				numToFelt(12),
				numToFelt(13),
				numToFelt(14),
				numToFelt(15),
				numToFelt(16),
				numToFelt(17),
				numToFelt(18),
				numToFelt(19),
			},
		},
		{
			name:     "limited",
			startKey: numToFelt(0),
			count:    2,
			expectedKeys: []*felt.Felt{
				numToFelt(0),
				numToFelt(1),
			},
			expectedValues: []*felt.Felt{
				numToFelt(10),
				numToFelt(11),
			},
		},
		{
			name:     "limited with offset",
			startKey: numToFelt(3),
			count:    2,
			expectedKeys: []*felt.Felt{
				numToFelt(3),
				numToFelt(4),
			},
			expectedValues: []*felt.Felt{
				numToFelt(13),
				numToFelt(14),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			keys := make([]*felt.Felt, 0)
			values := make([]*felt.Felt, 0)

			_, err := trie.Iterate(test.startKey, func(key *felt.Felt, value *felt.Felt) (bool, error) {
				keys = append(keys, key)
				values = append(values, value)
				return len(keys) < test.count, nil
			})
			assert.Nil(t, err)

			fmt.Printf("%s vs %s\n", test.expectedKeys[0], keys[0])
			assert.Equal(t, test.expectedKeys, keys)
			assert.Equal(t, test.expectedValues, values)
		})
	}
}

func TestTrie_GenerateProof(t *testing.T) {
	t.Run("with trie of interval 1", func(t *testing.T) {
		testTrie_GenerateProof(t, func() int64 {
			return 1
		})
	})
	t.Run("with trie of gap 1", func(t *testing.T) {
		testTrie_GenerateProof(t, func() int64 {
			return 2
		})
	})
	t.Run("with trie of gap 2", func(t *testing.T) {
		testTrie_GenerateProof(t, func() int64 {
			return 3
		})
	})
	t.Run("with trie of gap 10", func(t *testing.T) {
		testTrie_GenerateProof(t, func() int64 {
			return 10
		})
	})
	t.Run("with trie of gap 1000000", func(t *testing.T) {
		testTrie_GenerateProof(t, func() int64 {
			return 1000000
		})
	})

	for seednum := 0; seednum < 10; seednum++ {
		t.Run("with trie rand 10", func(t *testing.T) {
			rng := rand.New(rand.NewSource(uint64(seednum)))
			testTrie_GenerateProof(t, func() int64 {
				return rng.Int63n(10) + 1
			})
		})
		t.Run("with trie rand 100", func(t *testing.T) {
			rng := rand.New(rand.NewSource(uint64(seednum)))
			testTrie_GenerateProof(t, func() int64 {
				return rng.Int63n(100) + 1
			})
		})
		t.Run("with trie rand 1000000", func(t *testing.T) {
			rng := rand.New(rand.NewSource(uint64(seednum)))
			testTrie_GenerateProof(t, func() int64 {
				return rng.Int63n(1000000000000) + 1
			})
		})
	}
}

func testTrie_GenerateProof(t *testing.T, gapGen func() int64) {
	db, err := pebble.NewMem()
	assert.Nil(t, err)

	tr1, err := trie.NewTriePedersen(trie.NewTransactionStorage(db.NewTransaction(true), []byte{1}), 251)
	assert.Nil(t, err)

	sourcepaths := make([]*felt.Felt, 0)
	sourcevalues := make([]*felt.Felt, 0)
	curidx := big.NewInt(0)
	for i := 0; i < 10; i++ {
		value := *curidx
		value.Add(&value, big.NewInt(10))
		sourcepaths = append(sourcepaths, numToFeltBigInt(curidx))
		sourcevalues = append(sourcevalues, numToFeltBigInt(&value))
		curidx.Add(curidx, big.NewInt(gapGen()))
	}

	for i, sourcepath := range sourcepaths {
		_, err = tr1.Put(sourcepath, sourcevalues[i])
		assert.Nil(t, err)
	}

	tr1root, err := tr1.Root()
	assert.Nil(t, err)

	tests := []struct {
		name     string
		startIdx int
		count    int
		hasNext  bool
	}{
		{
			name:     "single value proof",
			startIdx: 5,
			count:    1,
			hasNext:  true,
		},
		{
			name:     "single value proof at start",
			startIdx: 0,
			count:    1,
			hasNext:  true,
		},
		{
			name:     "single value proof at end",
			startIdx: 9,
			count:    1,
			hasNext:  false,
		},
		{
			name:     "multi value proof",
			startIdx: 5,
			count:    4,
			hasNext:  true,
		},
		{
			name:     "multi value proof at start",
			startIdx: 0,
			count:    4,
			hasNext:  true,
		},
		{
			name:     "multi value proof at end",
			startIdx: 6,
			count:    4,
			hasNext:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths := sourcepaths[test.startIdx : test.startIdx+test.count]
			values := sourcevalues[test.startIdx : test.startIdx+test.count]

			proofs, err := tr1.ProofTo(paths[0])
			assert.Nil(t, err)

			if test.count > 1 {
				proof2, err := tr1.ProofTo(paths[len(paths)-1])
				assert.Nil(t, err)

				proofs = append(proofs, proof2...)
			}

			for _, proof := range proofs {
				assert.NotEqual(t, tr1root, proof.Hash)
			}

			hasNext, err := trie.VerifyTrie(tr1root, paths, values, proofs, crypto.Pedersen)
			assert.NoError(t, err)
			assert.Equal(t, test.hasNext, hasNext)
		})
	}

	/*
			// There can be cases where the proof overlap and this fail anyway
		t.Run("additional leaves without proof would fail", func(t *testing.T) {
			paths := sourcepaths[3:8]
			values := sourcevalues[3:8]

			proof, err := tr1.ProofTo(paths[0])
			assert.Nil(t, err)
			proof2, err := tr1.ProofTo(paths[len(paths)-3])
			assert.Nil(t, err)

			proofs := append(proof, proof2...)

			_, err = trie.VerifyTrie(tr1root, paths, values, proofs, crypto.Pedersen)
			assert.Error(t, err)
		})
	*/
}

func TestTrie_GenerateProof_SingleValue(t *testing.T) {
	db, err := pebble.NewMem()
	assert.Nil(t, err)

	tr1, err := trie.NewTriePedersen(trie.NewTransactionStorage(db.NewTransaction(true), []byte{1}), 251)
	assert.Nil(t, err)

	for i := 0; i < 1; i++ {
		_, err = tr1.Put(numToFelt(i), numToFelt(i+10))
		assert.Nil(t, err)
	}

	err = tr1.Commit()
	assert.Nil(t, err)

	tr1root, err := tr1.Root()
	assert.Nil(t, err)

	t.Run("test single value proof", func(t *testing.T) {
		db2, err := pebble.NewMem()
		assert.Nil(t, err)

		tr2, err := trie.NewTriePedersen(trie.NewTransactionStorage(db2.NewTransaction(true), []byte{1}), 251)
		assert.Nil(t, err)

		_, _ = tr2.Put(numToFelt(0), numToFelt(10))

		proof, err := tr1.ProofTo(numToFelt(0))
		assert.Nil(t, err)
		for _, node := range proof {
			err := tr2.SetProofNode(node.Key, node.Hash)
			assert.Nil(t, err)
		}

		tr2root, err := tr2.Root()

		assert.Equal(t, tr1root, tr2root)
	})
}

func Test_isBitsetHigher(t *testing.T) {
	tests := []struct {
		n1       int
		n2       int
		isHigher bool
	}{
		{
			n1:       10,
			n2:       0,
			isHigher: true,
		},
		{
			n1:       5,
			n2:       0,
			isHigher: true,
		},
		{
			n1:       5,
			n2:       4,
			isHigher: true,
		},
		{
			n1:       5,
			n2:       5,
			isHigher: false,
		},
		{
			n1:       4,
			n2:       5,
			isHigher: false,
		},
		{
			n1:       0,
			n2:       5,
			isHigher: false,
		},
		{
			n1:       300,
			n2:       1,
			isHigher: true,
		},
		{
			n1:       1,
			n2:       300,
			isHigher: false,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%d %d %v", test.n1, test.n2, test.isHigher), func(t *testing.T) {
			assert.Equal(t, trie.IsBitsetHigher(
				trie.FeltToBitSet(numToFelt(test.n1), 251),
				trie.FeltToBitSet(numToFelt(test.n2), 251)),
				test.isHigher)
			assert.Equal(t, trie.IsBitsetHigher(
				trie.FeltToBitSet(numToFeltMul(int64(test.n1), math.MaxInt64), 251),
				trie.FeltToBitSet(numToFeltMul(int64(test.n2), math.MaxInt64), 251)),
				test.isHigher)
		})
	}
}

func Test_FieldOrder(t *testing.T) {

	for i := 0; i < 10; i++ {
		bignum := big.NewInt(int64(i))
		thefelt := numToFeltBigInt(bignum)
		thebitset := trie.FeltToBitSet(thefelt, 251)

		buff := bytes.NewBuffer(make([]byte, 0))
		thebitset.WriteTo(buff)

		ashex := hex.EncodeToString(buff.Bytes())

		fmt.Printf("%s %s\n", thefelt.String(), ashex)
	}
}
