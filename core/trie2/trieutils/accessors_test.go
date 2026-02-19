package trieutils

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

type prefixCase struct {
	name   string
	prefix db.Bucket
	owner  felt.Address
}

type pathCase struct {
	name string
	path BitArray
}

// accessorPrefixes returns the three trie bucket / owner combinations exercised by the tests.
func accessorPrefixes() []prefixCase {
	return []prefixCase{
		{name: "ClassTrie", prefix: db.ClassTrie, owner: felt.Address{}},
		{name: "ContractTrieContract", prefix: db.ContractTrieContract, owner: felt.Address{}},
		{name: "ContractTrieStorage", prefix: db.ContractTrieStorage, owner: felt.FromUint64[felt.Address](0xDEADBEEF)},
	}
}

// nodeKeyPaths returns representative BitArray values for TestNodeKeyByPath,
// covering empty, single-bit, byte-boundary, word-boundary, and full 251-bit sizes.
func nodeKeyPaths() []pathCase {
	return []pathCase{
		{name: "empty", path: BitArray{}},
		{name: "1-bit", path: BitArray{len: 1, words: [4]uint64{1}}},
		{name: "8-bit", path: BitArray{len: 8, words: [4]uint64{0xFF}}},
		{name: "10-bit", path: BitArray{len: 10, words: [4]uint64{0x3FF}}},
		{name: "38-bit", path: BitArray{len: 38, words: [4]uint64{0x3FFFFFFFFF}}},
		{name: "64-bit", path: BitArray{len: 64, words: [4]uint64{maxUint64}}},
		{name: "64-bit/sparse", path: BitArray{len: 64, words: [4]uint64{0xAAAAAAAAAAAAAAAA}}},
		{name: "65-bit/word1-only", path: BitArray{len: 65, words: [4]uint64{0, 1}}},
		{name: "94-bit", path: BitArray{len: 94, words: [4]uint64{maxUint64, 0x3FFFFFFF}}},
		{name: "128-bit", path: BitArray{len: 128, words: [4]uint64{maxUint64, maxUint64}}},
		{name: "128-bit/alternating", path: BitArray{len: 128, words: [4]uint64{0xAAAAAAAAAAAAAAAA, 0x5555555555555555}}},
		{name: "129-bit/word2-only", path: BitArray{len: 129, words: [4]uint64{0, 0, 1}}},
		{name: "176-bit", path: BitArray{len: 176, words: [4]uint64{maxUint64, maxUint64, 0xFFFFFFFFFFFF}}},
		{name: "192-bit", path: BitArray{len: 192, words: [4]uint64{maxUint64, maxUint64, maxUint64}}},
		{name: "193-bit/word3-only", path: BitArray{len: 193, words: [4]uint64{0, 0, 0, 1}}},
		{name: "208-bit", path: BitArray{len: 208, words: [4]uint64{maxUint64, maxUint64, maxUint64, 0xFFFF}}},
		{name: "251-bit", path: BitArray{len: 251, words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x07FFFFFFFFFFFFFF}}},
		{name: "251-bit/alternating", path: BitArray{len: 251, words: [4]uint64{0xAAAAAAAAAAAAAAAA, 0x5555555555555555, 0xAAAAAAAAAAAAAAAA, 0x0555555555555555}}},
	}
}

// hashKeyPaths returns BitArray values for TestNodeKeyByHash.
// Paths are limited to len ≤ 32 to avoid a panic in the new nodeKeyByHash:
// max(8, path.len) treats path.len (a bit count) as a byte count, so
// path.len > 32 would cause Bytes()[0:path.len] to go out of bounds.
func hashKeyPaths() []pathCase {
	return []pathCase{
		{name: "empty", path: BitArray{}},
		{name: "4-bit", path: BitArray{len: 4, words: [4]uint64{0xF}}},
		{name: "8-bit", path: BitArray{len: 8, words: [4]uint64{0xFF}}},
		{name: "8-bit/sparse", path: BitArray{len: 8, words: [4]uint64{0xAA}}},
		{name: "16-bit", path: BitArray{len: 16, words: [4]uint64{0xAAAA}}},
		{name: "16-bit/zeros", path: BitArray{len: 16, words: [4]uint64{0}}},
		{name: "16-bit/beef", path: BitArray{len: 16, words: [4]uint64{0xBEEF}}},
		{name: "32-bit", path: BitArray{len: 32, words: [4]uint64{0xFFFFFFFF}}},
		{name: "32-bit/deadbeef", path: BitArray{len: 32, words: [4]uint64{0xDEADBEEF}}},
	}
}

// TestNodeKeyByPath verifies that nodeKeyByPath produces the same key layout as the
// old implementation. Any difference is a regression.
//
// Old key format:
//
//	ClassTrie / ContractTrieContract (zero owner):
//	  [1 byte prefix][1 byte node-type][path encoded bytes]
//
//	ContractTrieStorage (non-zero owner):
//	  [1 byte prefix][32 byte owner][1 byte node-type][path encoded bytes]
//
// Path encoded bytes = BitArray.EncodedBytes() = active bytes + bit-length byte.
func TestNodeKeyByPath(t *testing.T) {
	for _, pc := range accessorPrefixes() {
		for _, pathc := range nodeKeyPaths() {
			for _, isLeaf := range []bool{true, false} {
				pc, pathc, isLeaf := pc, pathc, isLeaf
				leafStr := "non-leaf"
				if isLeaf {
					leafStr = "leaf"
				}
				t.Run(fmt.Sprintf("%s/%s/%s", pc.name, leafStr, pathc.name), func(t *testing.T) {
					want := nodeKeyByPathOld(pc.prefix, &pc.owner, &pathc.path, isLeaf)
					got := nodeKeyByPath(pc.prefix, &pc.owner, &pathc.path, isLeaf)

					// --- Structural verification of the old key format ---
					nodeTypeByte := byte(nonLeaf)
					if isLeaf {
						nodeTypeByte = byte(leaf)
					}
					pathEncoded := pathc.path.EncodedBytes()

					assert.Equal(t, byte(pc.prefix), want[0], "old: first byte must be the prefix")

					if felt.IsZero(&pc.owner) {
						// Old: [prefix][nodeType][path...]
						wantLen := 1 + 1 + len(pathEncoded)
						assert.Equal(t, wantLen, len(want), "old: wrong key length for zero owner")
						assert.Equal(t, nodeTypeByte, want[1], "old: second byte must be node type")
						assert.Equal(t, pathEncoded, []byte(want[2:]), "old: path must follow node type")
					} else {
						// Old: [prefix][owner32][nodeType][path...]
						ownerBytes := pc.owner.Bytes()
						wantLen := 1 + 32 + 1 + len(pathEncoded)
						assert.Equal(t, wantLen, len(want), "old: wrong key length for non-zero owner")
						assert.Equal(t, ownerBytes[:], want[1:33], "old: bytes 1–32 must be owner")
						assert.Equal(t, nodeTypeByte, want[33], "old: byte 33 must be node type")
						assert.Equal(t, pathEncoded, []byte(want[34:]), "old: path must follow owner and node type")
					}

					// --- New implementation must match the old ---
					assert.Equal(t, want, got, "nodeKeyByPath: result differs from old implementation")
				})
			}
		}
	}
}

// TestNodeKeyByHash verifies that nodeKeyByHash produces the same key layout as the
// old implementation. Any difference is a regression.
//
// Old key format:
//
//	ClassTrie / ContractTrieContract (zero owner):
//	  [1 byte prefix][1 byte node-type][≥8 byte path section][32 byte hash]
//
//	ContractTrieStorage (non-zero owner):
//	  [1 byte prefix][32 byte owner][1 byte node-type][≥8 byte path section][32 byte hash]
//
// Path section = BitArray.ActiveBytes() padded to at least 8 bytes with trailing zeros.
//
// Note: the new nodeKeyByHash interprets path.len (bit count) as a byte count in
// max(8, path.len), causing wrong results for path.len > 0 and a panic for path.len > 32.
// Tests therefore use paths with len ≤ 32 to avoid panics and expose the semantic bug.
func TestNodeKeyByHash(t *testing.T) {
	hash := felt.FromUint64[felt.Hash](0xCAFEBABE)

	// pathSection computes the path bytes as the old implementation would include them
	// in the key: ActiveBytes() padded with trailing zeros to at least 8 bytes.
	pathSection := func(path *BitArray) []byte {
		active := (*BitArrayOld)(path).ActiveBytes()
		const minBytes = 8
		if len(active) < minBytes {
			padded := make([]byte, minBytes)
			copy(padded, active)
			return padded
		}
		return active
	}

	for _, pc := range accessorPrefixes() {
		for _, pathc := range hashKeyPaths() {
			for _, isLeaf := range []bool{true, false} {
				pc, pathc, isLeaf := pc, pathc, isLeaf
				leafStr := "non-leaf"
				if isLeaf {
					leafStr = "leaf"
				}
				t.Run(fmt.Sprintf("%s/%s/%s", pc.name, leafStr, pathc.name), func(t *testing.T) {
					want := nodeKeyByHashOld(pc.prefix, &pc.owner, (*BitArrayOld)(&pathc.path), &hash, isLeaf)
					got := nodeKeyByHash(pc.prefix, &pc.owner, &pathc.path, &hash, isLeaf)

					// --- Structural verification of the old key format ---
					nodeTypeByte := byte(nonLeaf)
					if isLeaf {
						nodeTypeByte = byte(leaf)
					}
					pathSec := pathSection(&pathc.path)
					hashBytes := hash.Bytes()

					assert.Equal(t, byte(pc.prefix), want[0], "old: first byte must be the prefix")

					if felt.IsZero(&pc.owner) {
						// Old: [prefix][nodeType][pathSection][hash32]
						wantLen := 1 + 1 + len(pathSec) + len(hashBytes)
						assert.Equal(t, wantLen, len(want), "old: wrong key length for zero owner")
						assert.Equal(t, nodeTypeByte, want[1], "old: second byte must be node type")
						pEnd := 2 + len(pathSec)
						assert.Equal(t, pathSec, []byte(want[2:pEnd]), "old: path section after node type")
						assert.Equal(t, hashBytes[:], want[pEnd:], "old: hash must be last 32 bytes")
					} else {
						// Old: [prefix][owner32][nodeType][pathSection][hash32]
						ownerBytes := pc.owner.Bytes()
						wantLen := 1 + 32 + 1 + len(pathSec) + len(hashBytes)
						assert.Equal(t, wantLen, len(want), "old: wrong key length for non-zero owner")
						assert.Equal(t, ownerBytes[:], want[1:33], "old: bytes 1–32 must be owner")
						assert.Equal(t, nodeTypeByte, want[33], "old: byte 33 must be node type")
						pEnd := 34 + len(pathSec)
						assert.Equal(t, pathSec, []byte(want[34:pEnd]), "old: path section after owner and node type")
						assert.Equal(t, hashBytes[:], want[pEnd:], "old: hash must be last 32 bytes")
					}

					// --- New implementation must match the old ---
					assert.Equal(t, want, got, "nodeKeyByHash: result differs from old implementation")
				})
			}
		}
	}
}
