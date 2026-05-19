package trie

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeOldPath(length uint8, val uint64) trie.BitArray {
	var ba trie.BitArray
	ba.SetUint64(length, val)
	return ba
}

func makeNewPath(length uint8, val uint64) trieutils.Path {
	old := makeOldPath(length, val)
	return toNewPath(&old)
}

func TestToNewPath_PreservesLengthAndBits(t *testing.T) {
	for _, tc := range []struct {
		name string
		len  uint8
		val  uint64
	}{
		{"zero length", 0, 0},
		{"single bit 0", 1, 0},
		{"single bit 1", 1, 1},
		{"8 bits", 8, 0xAB},
		{"251 bits", 251, 0xDEADBEEF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := makeOldPath(tc.len, tc.val)
			np := toNewPath(&old)
			assert.Equal(t, tc.len, np.Len())
			assert.Equal(t, old.Bytes(), np.Bytes())
		})
	}
}

func TestEncodeValueNode(t *testing.T) {
	var v felt.Felt
	v.SetUint64(0xCAFEBABE)
	blob := encodeValueNode(&v)
	assert.Equal(t, valueNodeBlobSize, len(blob))
	assert.Equal(t, v.Bytes(), blob)
}

func TestEncodeBinaryNode(t *testing.T) {
	var l, r felt.Felt
	l.SetUint64(1)
	r.SetUint64(2)
	blob := encodeBinaryNode(&l, &r)
	assert.Equal(t, binaryNodeBlobSize, len(blob))
	assert.Equal(t, binaryNodeTag, blob[0])
	lb := l.Bytes()
	rb := r.Bytes()
	assert.Equal(t, lb[:], blob[1:33])
	assert.Equal(t, rb[:], blob[33:65])
}

func TestEncodeEdgeNode(t *testing.T) {
	for _, pathLen := range []uint8{0, 1, 10, 250} {
		t.Run(fmt.Sprintf("pathLen=%d", pathLen), func(t *testing.T) {
			var childHash felt.Felt
			childHash.SetUint64(42)
			seg := makeNewPath(pathLen, 0b101)
			blob := encodeEdgeNode(&childHash, &seg)

			require.Greater(t, len(blob), 0)
			assert.Equal(t, edgeNodeTag, blob[0])

			var got felt.Felt
			got.SetBytes(blob[1:33])
			assert.Equal(t, childHash, got)

			activeBytes := (int(pathLen) + 7) / 8
			assert.Equal(t, 1+felt.Bytes+activeBytes+1, len(blob))
		})
	}
}

func TestComputeEdgeHash_ZeroLengthPath_ReturnChildHashUnchanged(t *testing.T) {
	var childHash felt.Felt
	childHash.SetUint64(99)
	var path trieutils.Path
	result := computeEdgeHash(&childHash, &path, crypto.Pedersen)
	assert.True(t, result.Equal(&childHash))
}

func TestComputeEdgeHash_MatchesEdgeNodeHash(t *testing.T) {
	for _, hashFn := range []crypto.HashFn{crypto.Pedersen, crypto.Poseidon} {
		for _, pathLen := range []uint8{1, 10, 250} {
			var childHash felt.Felt
			childHash.SetUint64(123456)
			seg := makeNewPath(pathLen, 0b1011)

			got := computeEdgeHash(&childHash, &seg, hashFn)

			hashNode := trienode.HashNode(childHash)
			edge := &trienode.EdgeNode{Child: &hashNode, Path: &seg}
			want := edge.Hash(hashFn)

			assert.True(t, got.Equal(&want), "pathLen=%d", pathLen)
		}
	}
}

func TestCompressedSegment_Length(t *testing.T) {
	for _, tc := range []struct {
		parentLen uint8
		segLen    uint8
	}{
		{0, 0},
		{0, 5},
		{10, 20},
		{100, 50},
	} {
		childLen := tc.parentLen + 1 + tc.segLen
		child := makeOldPath(childLen, 0b111)
		seg := compressedSegment(&child, tc.parentLen)
		assert.Equal(t, tc.segLen, seg.Len(), "parentLen=%d segLen=%d", tc.parentLen, tc.segLen)
	}
}

func TestOldTriePrefix_GlobalTrie(t *testing.T) {
	desc := TrieDesc{OldBucket: db.ClassesTrie, Owner: felt.Address{}}
	prefix := oldTriePrefix(desc)
	assert.Equal(t, []byte{byte(db.ClassesTrie)}, prefix)
}

func TestOldTriePrefix_StorageTrie(t *testing.T) {
	var ownerFelt felt.Felt
	ownerFelt.SetUint64(42)
	owner := felt.Address(ownerFelt)
	desc := TrieDesc{OldBucket: db.ContractStorage, Owner: owner}
	prefix := oldTriePrefix(desc)
	assert.Equal(t, byte(db.ContractStorage), prefix[0])
	assert.Equal(t, 1+felt.Bytes, len(prefix))
}

func TestParseOldPath_RoundTrip(t *testing.T) {
	original := makeOldPath(17, 0b10101)
	var buf bytes.Buffer
	_, err := original.Write(&buf)
	require.NoError(t, err)
	var parsed trie.BitArray
	require.NoError(t, parsed.UnmarshalBinary(buf.Bytes()))
	assert.Equal(t, original.Len(), parsed.Len())
	assert.Equal(t, original.Bytes(), parsed.Bytes())
}
