package trieutils

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

// used to avoid the compiler optimising away the results
var sink []byte

type testCase struct {
	name   string
	prefix db.Bucket
	owner  *felt.Address
	path   *BitArray
	isLeaf bool
}

type prefixCase struct {
	name   string
	prefix db.Bucket
}

type ownerCase struct {
	name  string
	owner felt.Address
}

type pathCase struct {
	name string
	path BitArray
}

// prefixes returns the three trie bucket / owner combinations exercised by the tests.
func prefixes() []prefixCase {
	return []prefixCase{
		{name: "ClassTrie", prefix: db.ClassTrie},
		{name: "ContractTrieContract", prefix: db.ContractTrieContract},
		{name: "ContractTrieStorage", prefix: db.ContractTrieStorage},
	}
}

func owners() []ownerCase {
	// they're all real Sepolia addresses
	return []ownerCase{
		{
			name:  "one leading zero",
			owner: felt.UnsafeFromString[felt.Address]("0x012b66312a967681ca775a20bb3445883b82477888c76790091c0b59593c5f9e"),
		},
		{
			name:  "two leading zeros",
			owner: felt.UnsafeFromString[felt.Address]("0x009f3a76d3f076c79adb925bfb322d4da753bf32f34f620a36b614a1860c3c18"),
		},
		{
			name:  "three leading zeros",
			owner: felt.UnsafeFromString[felt.Address]("0x0004fb2ab0254101b7424d9fa99679f6d46a33b999360bce3d8d7f7c9db275ec"),
		},
	}
}

// paths returns representative BitArray values for TestNodeKeyByPath,
// covering empty, single-bit, byte-boundary, word-boundary, and full 251-bit sizes.
func paths() []pathCase {
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

func testCases(t *testing.T) []testCase {
	t.Helper()
	var cases []testCase

	for _, pc := range prefixes() {
		for _, oc := range owners() {
			for _, pathc := range paths() {
				for _, isLeaf := range []bool{true, false} {
					leafName := "non-leaf"
					if isLeaf {
						leafName = "leaf"
					}

					var owner felt.Address
					if pc.prefix == db.ContractTrieStorage {
						// only ContractTrieStorage uses the owner for the key
						owner = oc.owner
					}

					cases = append(cases, testCase{
						name:   fmt.Sprintf("%s/%s/%s/%s", pc.name, oc.name, pathc.name, leafName),
						prefix: pc.prefix,
						owner:  &owner,
						path:   &pathc.path,
						isLeaf: isLeaf,
					})
				}
			}
		}
	}
	return cases
}

// TestNodeKeyByPath tests the key format of the nodeKeyByPath function.
//
// Format:
//
//	ClassTrie / ContractTrieContract (zero owner):
//	  [1 byte prefix][1 byte node-type][path encoded bytes]
//
//	ContractTrieStorage (non-zero owner):
//	  [1 byte prefix][32 byte owner][1 byte node-type][path encoded bytes]
//
// Path encoded bytes = BitArray.EncodedBytes() = active bytes + bit-length byte.
func TestNodeKeyByPath(t *testing.T) {
	for _, tc := range testCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			key := nodeKeyByPath(tc.prefix, tc.owner, tc.path, tc.isLeaf)

			nodeTypeByte := byte(nonLeaf)
			if tc.isLeaf {
				nodeTypeByte = byte(leaf)
			}
			pathEncoded := tc.path.EncodedBytes()

			const (
				// numbers in bytes
				prefixSize   = 1
				ownerSize    = 32
				nodeTypeSize = 1
			)
			assert.Equal(t, byte(tc.prefix), key[0], "first byte must be the prefix")

			if felt.IsZero(tc.owner) {
				// ClassTrie/ContractTrie:
				// [1 byte prefix][1 byte node-type][path]
				wantLen := prefixSize + nodeTypeSize + len(pathEncoded)
				assert.Equal(t, wantLen, len(key), "wrong key length for zero owner")
				assert.Equal(t, nodeTypeByte, key[1], "second byte must be node type")
				assert.Equal(t, pathEncoded, key[2:], "path must follow node type")
			} else {
				// StorageTrie of a Contract :
				// [1 byte prefix][32 bytes owner][1 byte node-type][path]
				ownerBytes := tc.owner.Bytes()
				wantLen := prefixSize + ownerSize + nodeTypeSize + len(pathEncoded)
				assert.Equal(t, wantLen, len(key), "wrong key length for non-zero owner")
				assert.Equal(t, ownerBytes[:], key[1:33], "bytes 1-32 must be owner")
				assert.Equal(t, nodeTypeByte, key[33], "byte 33 must be node type")
				assert.Equal(t, pathEncoded, key[34:], "path must follow owner and node type")
			}
		})
	}
}

// BenchmarkNodeKeyByPath compares the old and new nodeKeyByPath implementations
// across the full matrix of prefixes, owners, paths, and leaf types.
func BenchmarkNodeKeyByPath(b *testing.B) {
	type benchCase struct {
		prefix db.Bucket
		owner  felt.Address
		path   BitArray
		isLeaf bool
	}

	var cases []benchCase
	for _, pc := range prefixes() {
		for _, oc := range owners() {
			for _, pathc := range paths() {
				for _, isLeaf := range []bool{true, false} {
					var owner felt.Address
					if pc.prefix == db.ContractTrieStorage {
						owner = oc.owner
					}
					cases = append(cases, benchCase{
						prefix: pc.prefix,
						owner:  owner,
						path:   pathc.path,
						isLeaf: isLeaf,
					})
				}
			}
		}
	}

	b.Run("benchmark", func(b *testing.B) {
		b.ReportAllocs()
		for i := range b.N {
			tc := &cases[i%len(cases)]
			sink = nodeKeyByPath(tc.prefix, &tc.owner, &tc.path, tc.isLeaf)
		}
	})
}

// TestNodeKeyByHash tests the key format of the nodeKeyByHash function.
//
// Format:
//
//	ClassTrie / ContractTrieContract (zero owner):
//	  [1 byte prefix][1 byte node-type][≥8 byte path section][32 byte hash]
//
//	ContractTrieStorage (non-zero owner):
//	  [1 byte prefix][32 byte owner][1 byte node-type][≥8 byte path section][32 byte hash]
//
// Path section = The first 8 bytes of the path bytes. If the path is less than 8 bytes,
// it is padded with trailing zeros.
func TestNodeKeyByHash(t *testing.T) {
	hash := felt.FromUint64[felt.Hash](0xCAFEBABE)

	const (
		// numbers in bytes
		prefixSize           = 1
		ownerSize            = 32
		nodeTypeSize         = 1
		pathSignificantBytes = 8
		hashSize             = 32
	)

	// pathSection computes the path bytes as it should be in the key.
	pathSection := func(path *BitArray) []byte {
		bytes32 := path.Bytes()
		activeBytes := bytes32[path.inactiveBytes():]

		if len(activeBytes) <= pathSignificantBytes {
			tempSlice := make([]byte, pathSignificantBytes)
			copy(tempSlice, activeBytes)
			return tempSlice
		}
		return activeBytes[:pathSignificantBytes]
	}

	for _, tc := range testCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			key := nodeKeyByHash(tc.prefix, tc.owner, tc.path, &hash, tc.isLeaf)

			nodeTypeByte := byte(nonLeaf)
			if tc.isLeaf {
				nodeTypeByte = byte(leaf)
			}
			pathSec := pathSection(tc.path)
			hashBytes := hash.Bytes()

			assert.Equal(t, byte(tc.prefix), key[0], "first byte must be the prefix")

			if felt.IsZero(tc.owner) {
				// [prefix][nodeType][pathSection][hash32]
				wantLen := prefixSize + nodeTypeSize + pathSignificantBytes + hashSize
				assert.Equal(t, wantLen, len(key), "wrong key length for zero owner")
				assert.Equal(t, nodeTypeByte, key[1], "second byte must be node type")
				pEnd := prefixSize + nodeTypeSize + pathSignificantBytes
				assert.Equal(t, pathSec, key[2:pEnd], "path section after node type")
				assert.Equal(t, hashBytes[:], key[pEnd:], "hash must be last 32 bytes")
			} else {
				// [prefix][owner32][nodeType][pathSection][hash32]
				ownerBytes := tc.owner.Bytes()
				wantLen := prefixSize + ownerSize + nodeTypeSize + pathSignificantBytes + hashSize
				assert.Equal(t, wantLen, len(key), "wrong key length for non-zero owner")
				assert.Equal(t, ownerBytes[:], key[1:33], "bytes 1-32 must be owner")
				assert.Equal(t, nodeTypeByte, key[33], "byte 33 must be node type")
				pEnd := prefixSize + ownerSize + nodeTypeSize + pathSignificantBytes
				assert.Equal(t, pathSec, key[34:pEnd], "path section after owner and node type")
				assert.Equal(t, hashBytes[:], key[pEnd:], "hash must be last 32 bytes")
			}
		})
	}
}

// BenchmarkNodeKeyByHash compares the old and new nodeKeyByHash implementations
// across the full matrix of prefixes, owners, paths, and leaf types.
func BenchmarkNodeKeyByHash(b *testing.B) {
	type benchCase struct {
		prefix db.Bucket
		owner  felt.Address
		path   BitArray
		hash   felt.Hash
		isLeaf bool
	}

	var cases []benchCase
	for _, pc := range prefixes() {
		for _, oc := range owners() {
			for _, pathc := range paths() {
				for _, isLeaf := range []bool{true, false} {
					var owner felt.Address
					if pc.prefix == db.ContractTrieStorage {
						owner = oc.owner
					}
					cases = append(cases, benchCase{
						prefix: pc.prefix,
						owner:  owner,
						path:   pathc.path,
						isLeaf: isLeaf,
						hash:   felt.Hash(owner),
					})
				}
			}
		}
	}

	b.Run("benchmark", func(b *testing.B) {
		b.ReportAllocs()
		for i := range b.N {
			tc := &cases[i%len(cases)]
			sink = nodeKeyByHash(tc.prefix, &tc.owner, &tc.path, &tc.hash, tc.isLeaf)
		}
	})
}
