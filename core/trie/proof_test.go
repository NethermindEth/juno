package trie_test

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

func TestProve(t *testing.T) {
	t.Parallel()

	n := 1000
	tempTrie, records := nonRandomTrie(t, n)

	for _, record := range records {
		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(record.key, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Hash()
		require.NoError(t, err)

		val, err := trie.VerifyProof(&root, record.key, proofSet, crypto.Pedersen)
		if err != nil {
			t.Fatalf("failed for key %s", record.key.String())
		}
		require.Equal(t, *record.value, val)
	}
}

func TestProveNonExistent(t *testing.T) {
	t.Parallel()

	n := 1000
	tempTrie, _ := nonRandomTrie(t, n)

	for i := 1; i < n+1; i++ {
		keyFelt := new(felt.Felt).SetUint64(uint64(i + n))

		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(keyFelt, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Hash()
		require.NoError(t, err)

		val, err := trie.VerifyProof(&root, keyFelt, proofSet, crypto.Pedersen)
		if err != nil {
			t.Fatalf("failed for key %s", keyFelt.String())
		}
		require.Equal(t, felt.Zero, val)
	}
}

func TestProveRandom(t *testing.T) {
	t.Parallel()
	tempTrie, records := randomTrie(t, 1000)

	for _, record := range records {
		proofSet := trie.NewProofNodeSet()
		err := tempTrie.Prove(record.key, proofSet)
		require.NoError(t, err)

		root, err := tempTrie.Hash()
		require.NoError(t, err)

		val, err := trie.VerifyProof(&root, record.key, proofSet, crypto.Pedersen)
		require.NoError(t, err)
		require.Equal(t, *record.value, val)
	}
}

func TestProveCustom(t *testing.T) {
	t.Parallel()

	tests := []testTrie{
		{
			name:    "simple binary",
			buildFn: buildSimpleTrie,
			testKeys: []testKey{
				{
					name:     "prove existing key",
					key:      new(felt.Felt).SetUint64(1),
					expected: new(felt.Felt).SetUint64(3),
				},
			},
		},
		{
			name:    "simple double binary",
			buildFn: buildSimpleDoubleBinaryTrie,
			testKeys: []testKey{
				{
					name:     "prove existing key 0",
					key:      new(felt.Felt).SetUint64(0),
					expected: new(felt.Felt).SetUint64(2),
				},
				{
					name:     "prove existing key 3",
					key:      new(felt.Felt).SetUint64(3),
					expected: new(felt.Felt).SetUint64(5),
				},
				{
					name:     "prove non-existent key 2",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(0),
				},
				{
					name:     "prove non-existent key 123",
					key:      new(felt.Felt).SetUint64(123),
					expected: new(felt.Felt).SetUint64(0),
				},
			},
		},
		{
			name:    "simple binary root",
			buildFn: buildSimpleBinaryRootTrie,
			testKeys: []testKey{
				{
					name:     "prove existing key",
					key:      new(felt.Felt).SetUint64(0),
					expected: felt.NewUnsafeFromString[felt.Felt]("0xcc"),
				},
			},
		},
		{
			name: "left-right edge",
			buildFn: func(t *testing.T) (*trie.Trie, []*keyValue) {
				memdb := memory.New()
				txn := memdb.NewIndexedBatch()

				tr, err := trie.NewTriePedersen(txn, []byte{1}, 251)
				require.NoError(t, err)

				records := []*keyValue{
					{key: felt.NewUnsafeFromString[felt.Felt]("0xff"), value: felt.NewUnsafeFromString[felt.Felt]("0xaa")},
				}

				for _, record := range records {
					_, err = tr.Put(record.key, record.value)
					require.NoError(t, err)
				}
				require.NoError(t, tr.Commit())
				return tr, records
			},
			testKeys: []testKey{
				{
					name:     "prove existing key",
					key:      felt.NewUnsafeFromString[felt.Felt]("0xff"),
					expected: felt.NewUnsafeFromString[felt.Felt]("0xaa"),
				},
			},
		},
		{
			name:    "three key trie",
			buildFn: build3KeyTrie,
			testKeys: []testKey{
				{
					name:     "prove existing key",
					key:      new(felt.Felt).SetUint64(2),
					expected: new(felt.Felt).SetUint64(6),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tr, _ := test.buildFn(t)

			for _, tc := range test.testKeys {
				t.Run(tc.name, func(t *testing.T) {
					proofSet := trie.NewProofNodeSet()
					err := tr.Prove(tc.key, proofSet)
					require.NoError(t, err)

					root, err := tr.Hash()
					require.NoError(t, err)

					val, err := trie.VerifyProof(&root, tc.key, proofSet, crypto.Pedersen)
					require.NoError(t, err)
					require.Equal(t, *tc.expected, val)
				})
			}
		})
	}
}

// TestRangeProof tests normal range proof with both edge proofs
func TestRangeProof(t *testing.T) {
	t.Parallel()

	n := 500
	tr, records := randomTrie(t, n)
	root, err := tr.Hash()
	require.NoError(t, err)

	for range 100 {
		start := rand.Intn(n)
		end := rand.Intn(n-start) + start + 1

		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(records[start].key, records[end-1].key, proof)
		require.NoError(t, err)

		keys := []*felt.Felt{}
		values := []*felt.Felt{}
		for i := start; i < end; i++ {
			keys = append(keys, records[i].key)
			values = append(values, records[i].value)
		}

		_, err = trie.VerifyRangeProof(&root, records[start].key, keys, values, proof)
		require.NoError(t, err)
	}
}

// TestRangeProofWithNonExistentProof tests normal range proof with non-existent proofs
func TestRangeProofWithNonExistentProof(t *testing.T) {
	t.Parallel()

	n := 500
	tr, records := randomTrie(t, n)
	root, err := tr.Hash()
	require.NoError(t, err)

	for range 100 {
		start := rand.Intn(n)
		end := rand.Intn(n-start) + start + 1

		first := decrementFelt(records[start].key)
		if start != 0 && first.Equal(records[start-1].key) {
			continue
		}

		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(first, records[end-1].key, proof)
		require.NoError(t, err)

		keys := make([]*felt.Felt, end-start)
		values := make([]*felt.Felt, end-start)
		for i := start; i < end; i++ {
			keys[i-start] = records[i].key
			values[i-start] = records[i].value
		}

		_, err = trie.VerifyRangeProof(&root, first, keys, values, proof)
		require.NoError(t, err)
	}
}

// TestRangeProofWithInvalidNonExistentProof tests range proof with invalid non-existent proofs.
// One scenario is when there is a gap between the first element and the left edge proof.
func TestRangeProofWithInvalidNonExistentProof(t *testing.T) {
	t.Parallel()

	n := 500
	tr, records := randomTrie(t, n)
	root, err := tr.Hash()
	require.NoError(t, err)

	start, end := 100, 200
	first := decrementFelt(records[start].key)

	proof := trie.NewProofNodeSet()
	err = tr.GetRangeProof(first, records[end-1].key, proof)
	require.NoError(t, err)

	start = 105 // Gap created
	keys := make([]*felt.Felt, end-start)
	values := make([]*felt.Felt, end-start)
	for i := start; i < end; i++ {
		keys[i-start] = records[i].key
		values[i-start] = records[i].value
	}

	_, err = trie.VerifyRangeProof(&root, first, keys, values, proof)
	require.Error(t, err)
}

func TestOneElementRangeProof(t *testing.T) {
	t.Parallel()

	n := 1000
	tr, records := randomTrie(t, n)
	root, err := tr.Hash()
	require.NoError(t, err)

	t.Run("both edge proofs with the same key", func(t *testing.T) {
		t.Parallel()

		start := 100
		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(records[start].key, records[start].key, proof)
		require.NoError(t, err)

		_, err = trie.VerifyRangeProof(
			&root,
			records[start].key,
			[]*felt.Felt{records[start].key},
			[]*felt.Felt{records[start].value},
			proof,
		)
		require.NoError(t, err)
	})

	t.Run("left non-existent edge proof", func(t *testing.T) {
		t.Parallel()

		start := 100
		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(decrementFelt(records[start].key), records[start].key, proof)
		require.NoError(t, err)

		_, err = trie.VerifyRangeProof(
			&root,
			decrementFelt(records[start].key),
			[]*felt.Felt{records[start].key},
			[]*felt.Felt{records[start].value},
			proof,
		)
		require.NoError(t, err)
	})

	t.Run("right non-existent edge proof", func(t *testing.T) {
		t.Parallel()

		end := 100
		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(records[end].key, incrementFelt(records[end].key), proof)
		require.NoError(t, err)

		_, err = trie.VerifyRangeProof(
			&root,
			records[end].key,
			[]*felt.Felt{records[end].key},
			[]*felt.Felt{records[end].value},
			proof,
		)
		require.NoError(t, err)
	})

	t.Run("both non-existent edge proofs", func(t *testing.T) {
		t.Parallel()

		start := 100
		first, last := decrementFelt(records[start].key), incrementFelt(records[start].key)
		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(first, last, proof)
		require.NoError(t, err)

		_, err = trie.VerifyRangeProof(
			&root,
			first,
			[]*felt.Felt{records[start].key},
			[]*felt.Felt{records[start].value},
			proof,
		)
		require.NoError(t, err)
	})

	t.Run("1 key trie", func(t *testing.T) {
		t.Parallel()

		tr, records := build1KeyTrie(t)
		root, err := tr.Hash()
		require.NoError(t, err)

		proof := trie.NewProofNodeSet()
		err = tr.GetRangeProof(&felt.Zero, records[0].key, proof)
		require.NoError(t, err)

		_, err = trie.VerifyRangeProof(
			&root,
			records[0].key,
			[]*felt.Felt{records[0].key},
			[]*felt.Felt{records[0].value},
			proof,
		)
		require.NoError(t, err)
	})
}

// TestAllElementsRangeProof tests the range proof with all elements and nil proof.
func TestAllElementsRangeProof(t *testing.T) {
	t.Parallel()

	n := 1000
	tr, records := randomTrie(t, n)
	root, err := tr.Hash()
	require.NoError(t, err)

	keys := make([]*felt.Felt, n)
	values := make([]*felt.Felt, n)
	for i, record := range records {
		keys[i] = record.key
		values[i] = record.value
	}

	_, err = trie.VerifyRangeProof(&root, nil, keys, values, nil)
	require.NoError(t, err)

	// Should also work with proof
	proof := trie.NewProofNodeSet()
	err = tr.GetRangeProof(records[0].key, records[n-1].key, proof)
	require.NoError(t, err)

	_, err = trie.VerifyRangeProof(&root, keys[0], keys, values, proof)
	require.NoError(t, err)
}

// TestSingleSideRangeProof tests the range proof starting with zero.
func TestSingleSideRangeProof(t *testing.T) {
	t.Parallel()

	tr, records := randomTrie(t, 1000)
	root, err := tr.Hash()
	require.NoError(t, err)

	for i := 0; i < len(records); i += 100 {
		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(&felt.Zero, records[i].key, proof)
		require.NoError(t, err)

		keys := make([]*felt.Felt, i+1)
		values := make([]*felt.Felt, i+1)
		for j := range i + 1 {
			keys[j] = records[j].key
			values[j] = records[j].value
		}

		_, err = trie.VerifyRangeProof(&root, &felt.Zero, keys, values, proof)
		require.NoError(t, err)
	}
}

func TestGappedRangeProof(t *testing.T) {
	t.Parallel()
	t.Skip("gapped keys will sometimes succeed, the current proof format is not able to handle this")

	tr, records := nonRandomTrie(t, 5)
	root, err := tr.Hash()
	require.NoError(t, err)

	first, last := 1, 4
	proof := trie.NewProofNodeSet()
	err = tr.GetRangeProof(records[first].key, records[last].key, proof)
	require.NoError(t, err)

	keys := []*felt.Felt{}
	values := []*felt.Felt{}
	for i := first; i <= last; i++ {
		if i == (first+last)/2 {
			continue
		}

		keys = append(keys, records[i].key)
		values = append(values, records[i].value)
	}

	_, err = trie.VerifyRangeProof(&root, records[first].key, keys, values, proof)
	require.Error(t, err)
}

func TestEmptyRangeProof(t *testing.T) {
	t.Parallel()

	tr, records := randomTrie(t, 1000)
	root, err := tr.Hash()
	require.NoError(t, err)

	cases := []struct {
		pos int
		err bool
	}{
		{len(records) - 1, false},
		{500, true},
	}

	for _, c := range cases {
		proof := trie.NewProofNodeSet()
		first := incrementFelt(records[c.pos].key)
		err = tr.GetRangeProof(first, first, proof)
		require.NoError(t, err)

		_, err := trie.VerifyRangeProof(&root, first, nil, nil, proof)
		if c.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}

func TestHasRightElement(t *testing.T) {
	t.Parallel()

	tr, records := randomTrie(t, 500)
	root, err := tr.Hash()
	require.NoError(t, err)

	cases := []struct {
		start   int
		end     int
		hasMore bool
	}{
		{-1, 1, true},                           // single element with non-existent left proof
		{0, 1, true},                            // single element with existent left proof
		{0, 100, true},                          // start to middle
		{50, 100, true},                         // middle only
		{50, len(records), false},               // middle to end
		{len(records) - 1, len(records), false}, // Single last element with two existent proofs(point to same key)
		{0, len(records), false},                // The whole set with existent left proof
		{-1, len(records), false},               // The whole set with non-existent left proof
	}

	for _, c := range cases {
		var (
			first *felt.Felt
			start = c.start
			end   = c.end
			proof = trie.NewProofNodeSet()
		)
		if start == -1 {
			first = &felt.Zero
			start = 0
		} else {
			first = records[start].key
		}

		err := tr.GetRangeProof(first, records[end-1].key, proof)
		require.NoError(t, err)

		keys := []*felt.Felt{}
		values := []*felt.Felt{}
		for i := start; i < end; i++ {
			keys = append(keys, records[i].key)
			values = append(values, records[i].value)
		}

		hasMore, err := trie.VerifyRangeProof(&root, first, keys, values, proof)
		require.NoError(t, err)
		require.Equal(t, c.hasMore, hasMore)
	}
}

// TestBadRangeProof generates random bad proof scenarios and verifies that the proof is invalid.
func TestBadRangeProof(t *testing.T) {
	t.Parallel()

	tr, records := randomTrie(t, 1000)
	root, err := tr.Hash()
	require.NoError(t, err)

	for range 100 {
		start := rand.Intn(len(records))
		end := rand.Intn(len(records)-start) + start + 1

		proof := trie.NewProofNodeSet()
		err := tr.GetRangeProof(records[start].key, records[end-1].key, proof)
		require.NoError(t, err)

		keys := []*felt.Felt{}
		values := []*felt.Felt{}
		for j := start; j < end; j++ {
			keys = append(keys, records[j].key)
			values = append(values, records[j].value)
		}

		first := keys[0]
		testCase := rand.Intn(5)

		index := rand.Intn(end - start)
		switch testCase {
		case 0: // modified key
			keys[index] = new(felt.Felt).SetUint64(rand.Uint64())
		case 1: // modified value
			values[index] = new(felt.Felt).SetUint64(rand.Uint64())
		case 2: // out of order
			index2 := rand.Intn(end - start)
			if index2 == index {
				continue
			}
			keys[index], keys[index2] = keys[index2], keys[index]
			values[index], values[index2] = values[index2], values[index]
		case 3: // set random key to empty
			keys[index] = &felt.Zero
		case 4: // set random value to empty
			values[index] = &felt.Zero
			// TODO(weiihann): gapped proof will fail sometimes
			// case 5: // gapped
			// 	if end-start < 100 || index == 0 || index == end-start-1 {
			// 		continue
			// 	}
			// 	keys = append(keys[:index], keys[index+1:]...)
			// 	values = append(values[:index], values[index+1:]...)
		}
		_, err = trie.VerifyRangeProof(&root, first, keys, values, proof)
		if err == nil {
			t.Fatalf("expected error for test case %d, index %d, start %d, end %d", testCase, index, start, end)
		}
	}
}

func BenchmarkProve(b *testing.B) {
	tr, records := randomTrie(b, 1000)
	b.ResetTimer()
	for i := range b.N {
		proof := trie.NewProofNodeSet()
		key := records[i%len(records)].key
		if err := tr.Prove(key, proof); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyProof(b *testing.B) {
	tr, records := randomTrie(b, 1000)
	root, err := tr.Hash()
	require.NoError(b, err)

	proofs := make([]*trie.ProofNodeSet, 0, len(records))
	for _, record := range records {
		proof := trie.NewProofNodeSet()
		if err := tr.Prove(record.key, proof); err != nil {
			b.Fatal(err)
		}
		proofs = append(proofs, proof)
	}

	b.ResetTimer()
	for i := range b.N {
		index := i % len(records)
		if _, err := trie.VerifyProof(
			&root,
			records[index].key,
			proofs[index],
			crypto.Pedersen,
		); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyRangeProof(b *testing.B) {
	tr, records := randomTrie(b, 1000)
	root, err := tr.Hash()
	require.NoError(b, err)

	start := 2
	end := start + 500

	proof := trie.NewProofNodeSet()
	err = tr.GetRangeProof(records[start].key, records[end-1].key, proof)
	require.NoError(b, err)

	keys := make([]*felt.Felt, end-start)
	values := make([]*felt.Felt, end-start)
	for i := start; i < end; i++ {
		keys[i-start] = records[i].key
		values[i-start] = records[i].value
	}

	b.ResetTimer()
	for range b.N {
		_, err := trie.VerifyRangeProof(&root, keys[0], keys, values, proof)
		require.NoError(b, err)
	}
}

func buildTrie(t *testing.T, records []*keyValue) *trie.Trie {
	if len(records) == 0 {
		t.Fatal("records must have at least one element")
	}

	memdb := memory.New()
	txn := memdb.NewIndexedBatch()

	tempTrie, err := trie.NewTriePedersen(txn, []byte{0}, 251)
	require.NoError(t, err)

	for _, record := range records {
		_, err = tempTrie.Put(record.key, record.value)
		require.NoError(t, err)
	}

	require.NoError(t, tempTrie.Commit())

	return tempTrie
}

func build1KeyTrie(t *testing.T) (*trie.Trie, []*keyValue) {
	return nonRandomTrie(t, 1)
}

func buildSimpleTrie(t *testing.T) (*trie.Trie, []*keyValue) {
	//   (250, 0, x1)		edge
	//        |
	//     (0,0,x1)			binary
	//      /    \
	//     (2)  (3)
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(0), value: new(felt.Felt).SetUint64(2)},
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(3)},
	}

	return buildTrie(t, records), records
}

func buildSimpleBinaryRootTrie(t *testing.T) (*trie.Trie, []*keyValue) {
	// PF
	//           (0, 0, x)
	//    /                    \
	// (250, 0, cc)     (250, 11111.., dd)
	//    |                     |
	//   (cc)                  (dd)

	//	JUNO
	//           (0, 0, x)
	//    /                    \
	// (251, 0, cc)     (251, 11111.., dd)
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(0), value: felt.NewUnsafeFromString[felt.Felt]("0xcc")},
		{key: felt.NewUnsafeFromString[felt.Felt]("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), value: felt.NewUnsafeFromString[felt.Felt]("0xdd")},
	}
	return buildTrie(t, records), records
}

//nolint:dupl
func buildSimpleDoubleBinaryTrie(t *testing.T) (*trie.Trie, []*keyValue) {
	//           (249,0,x3)         // Edge
	//               |
	//           (0, 0, x3)         // Binary
	//         /            \
	//     (0,0,x1) // B  (1, 1, 5) // Edge leaf
	//      /    \             |
	//     (2)  (3)           (5)
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(0), value: new(felt.Felt).SetUint64(2)},
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(3)},
		{key: new(felt.Felt).SetUint64(3), value: new(felt.Felt).SetUint64(5)},
	}
	return buildTrie(t, records), records
}

//nolint:dupl
func build3KeyTrie(t *testing.T) (*trie.Trie, []*keyValue) {
	// 			Starknet
	//			--------
	//
	//			Edge
	//			|
	//			Binary with len 249				 parent
	//		 /				\
	//	Binary (250)	Edge with len 250
	//	/	\				/
	// 0x4	0x5			0x6						 child

	//			 Juno
	//			 ----
	//
	//		Node (path 249)
	//		/			\
	//  Node (binary)	 \
	//	/	\			 /
	// 0x4	0x5		   0x6
	records := []*keyValue{
		{key: new(felt.Felt).SetUint64(0), value: new(felt.Felt).SetUint64(4)},
		{key: new(felt.Felt).SetUint64(1), value: new(felt.Felt).SetUint64(5)},
		{key: new(felt.Felt).SetUint64(2), value: new(felt.Felt).SetUint64(6)},
	}

	return buildTrie(t, records), records
}

func nonRandomTrie(t *testing.T, numKeys int) (*trie.Trie, []*keyValue) {
	memdb := memory.New()
	txn := memdb.NewIndexedBatch()

	tempTrie, err := trie.NewTriePedersen(txn, []byte{0}, 251)
	require.NoError(t, err)

	records := make([]*keyValue, numKeys)
	for i := 1; i < numKeys+1; i++ {
		key := new(felt.Felt).SetUint64(uint64(i))
		records[i-1] = &keyValue{key: key, value: key}
		_, err := tempTrie.Put(key, key)
		require.NoError(t, err)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].key.Cmp(records[j].key) < 0
	})

	require.NoError(t, tempTrie.Commit())

	return tempTrie, records
}

func randomTrie(t testing.TB, n int) (*trie.Trie, []*keyValue) {
	rrand := rand.New(rand.NewSource(3))

	memdb := memory.New()
	txn := memdb.NewIndexedBatch()

	tempTrie, err := trie.NewTriePedersen(txn, []byte{0}, 251)
	require.NoError(t, err)

	records := make([]*keyValue, n)
	for i := range n {
		key := new(felt.Felt).SetUint64(uint64(rrand.Uint32() + 1))
		records[i] = &keyValue{key: key, value: key}
		_, err := tempTrie.Put(key, key)
		require.NoError(t, err)
	}

	require.NoError(t, tempTrie.Commit())

	// Sort records by key
	sort.Slice(records, func(i, j int) bool {
		return records[i].key.Cmp(records[j].key) < 0
	})

	return tempTrie, records
}

func decrementFelt(f *felt.Felt) *felt.Felt {
	return new(felt.Felt).Sub(f, new(felt.Felt).SetUint64(1))
}

func incrementFelt(f *felt.Felt) *felt.Felt {
	return new(felt.Felt).Add(f, new(felt.Felt).SetUint64(1))
}

type testKey struct {
	name     string
	key      *felt.Felt
	expected *felt.Felt
}

type testTrie struct {
	name     string
	buildFn  func(*testing.T) (*trie.Trie, []*keyValue)
	testKeys []testKey
}

type keyValue struct {
	key   *felt.Felt
	value *felt.Felt
}
