package trieutils

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

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
	zeroOwner := felt.Address{}
	nonZeroOwner := felt.FromUint64[felt.Address](0xDEADBEEF)

	tests := []struct {
		name   string
		prefix db.Bucket
		owner  felt.Address
		path   BitArray
		isLeaf bool
	}{
		// --- Zero-owner buckets (ClassTrie, ContractTrieContract) ---
		// Old: owner bytes are omitted entirely.
		// New: always includes 32 zero bytes for the owner → key is 32 bytes longer.
		{
			name:   "ClassTrie/non-leaf/empty path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{},
			isLeaf: false,
		},
		{
			name:   "ClassTrie/leaf/1-bit path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{len: 1, words: [4]uint64{1}},
			isLeaf: true,
		},
		{
			name:   "ClassTrie/non-leaf/8-bit path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{len: 8, words: [4]uint64{0xFF}},
			isLeaf: false,
		},
		{
			name:   "ClassTrie/leaf/10-bit path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{len: 10, words: [4]uint64{0x3FF}},
			isLeaf: true,
		},
		{
			name:   "ClassTrie/non-leaf/64-bit path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{len: 64, words: [4]uint64{maxUint64}},
			isLeaf: false,
		},
		{
			name:   "ClassTrie/leaf/128-bit path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{len: 128, words: [4]uint64{maxUint64, maxUint64}},
			isLeaf: true,
		},
		{
			name:   "ClassTrie/non-leaf/251-bit path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{len: 251, words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x07FFFFFFFFFFFFFF}},
			isLeaf: false,
		},
		{
			name:   "ContractTrieContract/leaf/64-bit sparse path",
			prefix: db.ContractTrieContract,
			owner:  zeroOwner,
			path:   BitArray{len: 64, words: [4]uint64{0xAAAAAAAAAAAAAAAA}},
			isLeaf: true,
		},
		{
			name:   "ContractTrieContract/non-leaf/251-bit path",
			prefix: db.ContractTrieContract,
			owner:  zeroOwner,
			path:   BitArray{len: 251, words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x07FFFFFFFFFFFFFF}},
			isLeaf: false,
		},
		// --- Non-zero owner (ContractTrieStorage) ---
		// Both old and new include the 32-byte owner.
		{
			name:   "ContractTrieStorage/non-leaf/empty path",
			prefix: db.ContractTrieStorage,
			owner:  nonZeroOwner,
			path:   BitArray{},
			isLeaf: false,
		},
		{
			name:   "ContractTrieStorage/leaf/8-bit path",
			prefix: db.ContractTrieStorage,
			owner:  nonZeroOwner,
			path:   BitArray{len: 8, words: [4]uint64{0xAA}},
			isLeaf: true,
		},
		{
			name:   "ContractTrieStorage/non-leaf/64-bit path",
			prefix: db.ContractTrieStorage,
			owner:  nonZeroOwner,
			path:   BitArray{len: 64, words: [4]uint64{maxUint64}},
			isLeaf: false,
		},
		{
			name:   "ContractTrieStorage/leaf/128-bit path",
			prefix: db.ContractTrieStorage,
			owner:  nonZeroOwner,
			path:   BitArray{len: 128, words: [4]uint64{0xAAAAAAAAAAAAAAAA, maxUint64}},
			isLeaf: true,
		},
		{
			name:   "ContractTrieStorage/non-leaf/251-bit path",
			prefix: db.ContractTrieStorage,
			owner:  nonZeroOwner,
			path:   BitArray{len: 251, words: [4]uint64{maxUint64, maxUint64, maxUint64, 0x07FFFFFFFFFFFFFF}},
			isLeaf: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := nodeKeyByPathOld(tt.prefix, &tt.owner, &tt.path, tt.isLeaf)
			got := nodeKeyByPath(tt.prefix, &tt.owner, &tt.path, tt.isLeaf)

			// --- Structural verification of the old key format ---
			nodeTypeByte := byte(nonLeaf)
			if tt.isLeaf {
				nodeTypeByte = byte(leaf)
			}
			pathEncoded := tt.path.EncodedBytes()

			assert.Equal(t, byte(tt.prefix), want[0], "old: first byte must be the prefix")

			if felt.IsZero(&tt.owner) {
				// Old: [prefix][nodeType][path...]
				wantLen := 1 + 1 + len(pathEncoded)
				assert.Equal(t, wantLen, len(want), "old: wrong key length for zero owner")
				assert.Equal(t, nodeTypeByte, want[1], "old: second byte must be node type")
				assert.Equal(t, pathEncoded, []byte(want[2:]), "old: path must follow node type")
			} else {
				// Old: [prefix][owner32][nodeType][path...]
				ownerBytes := tt.owner.Bytes()
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
	zeroOwner := felt.Address{}
	nonZeroOwner := felt.FromUint64[felt.Address](0xDEADBEEF)
	hash := felt.FromUint64[felt.Hash](0xCAFEBABE)

	tests := []struct {
		name   string
		prefix db.Bucket
		owner  felt.Address
		path   BitArray
		hash   felt.Hash
		isLeaf bool
	}{
		// --- Zero-owner buckets ---

		// len=0: ActiveBytes()=nil → 8 zero bytes. Both old and new happen to agree here.
		{
			name:   "ClassTrie/non-leaf/empty path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{},
			hash:   hash,
			isLeaf: false,
		},
		// len=4: ActiveBytes()=[0x0F] (1 byte) → padded to 8 bytes.
		// New: pathSize=max(8,4)=8, takes Bytes()[0:8] = 8 leading zeros → wrong.
		{
			name:   "ClassTrie/leaf/4-bit path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{len: 4, words: [4]uint64{0xF}},
			hash:   hash,
			isLeaf: true,
		},
		// len=8: ActiveBytes()=[0xFF] (1 byte) → padded to 8 bytes.
		// New: pathSize=max(8,8)=8, takes Bytes()[0:8] = 8 leading zeros → wrong.
		{
			name:   "ClassTrie/non-leaf/8-bit path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{len: 8, words: [4]uint64{0xFF}},
			hash:   hash,
			isLeaf: false,
		},
		// len=16: ActiveBytes()=[0xAA,0xAA] (2 bytes) → padded to 8 bytes.
		// New: pathSize=max(8,16)=16, takes Bytes()[0:16] = 16 leading zeros AND different length → wrong.
		{
			name:   "ClassTrie/leaf/16-bit path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{len: 16, words: [4]uint64{0xAAAA}},
			hash:   hash,
			isLeaf: true,
		},
		// len=32: ActiveBytes()=[0xFF,0xFF,0xFF,0xFF] (4 bytes) → padded to 8 bytes.
		// New: pathSize=max(8,32)=32, takes Bytes()[0:32] = 28 leading zeros + 4 active bytes → wrong length and content.
		{
			name:   "ClassTrie/non-leaf/32-bit path",
			prefix: db.ClassTrie,
			owner:  zeroOwner,
			path:   BitArray{len: 32, words: [4]uint64{0xFFFFFFFF}},
			hash:   hash,
			isLeaf: false,
		},
		{
			name:   "ContractTrieContract/leaf/8-bit sparse path",
			prefix: db.ContractTrieContract,
			owner:  zeroOwner,
			path:   BitArray{len: 8, words: [4]uint64{0xAA}},
			hash:   hash,
			isLeaf: true,
		},
		{
			name:   "ContractTrieContract/non-leaf/16-bit all-zeros path",
			prefix: db.ContractTrieContract,
			owner:  zeroOwner,
			path:   BitArray{len: 16, words: [4]uint64{0}},
			hash:   hash,
			isLeaf: false,
		},
		// --- Non-zero owner (ContractTrieStorage) ---
		{
			name:   "ContractTrieStorage/leaf/empty path",
			prefix: db.ContractTrieStorage,
			owner:  nonZeroOwner,
			path:   BitArray{},
			hash:   hash,
			isLeaf: true,
		},
		{
			name:   "ContractTrieStorage/non-leaf/8-bit path",
			prefix: db.ContractTrieStorage,
			owner:  nonZeroOwner,
			path:   BitArray{len: 8, words: [4]uint64{0xFF}},
			hash:   hash,
			isLeaf: false,
		},
		{
			name:   "ContractTrieStorage/leaf/16-bit path",
			prefix: db.ContractTrieStorage,
			owner:  nonZeroOwner,
			path:   BitArray{len: 16, words: [4]uint64{0xBEEF}},
			hash:   hash,
			isLeaf: true,
		},
		{
			name:   "ContractTrieStorage/non-leaf/32-bit path",
			prefix: db.ContractTrieStorage,
			owner:  nonZeroOwner,
			path:   BitArray{len: 32, words: [4]uint64{0xDEADBEEF}},
			hash:   hash,
			isLeaf: false,
		},
	}

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := nodeKeyByHashOld(tt.prefix, &tt.owner, (*BitArrayOld)(&tt.path), &tt.hash, tt.isLeaf)
			got := nodeKeyByHash(tt.prefix, &tt.owner, &tt.path, &tt.hash, tt.isLeaf)

			// --- Structural verification of the old key format ---
			nodeTypeByte := byte(nonLeaf)
			if tt.isLeaf {
				nodeTypeByte = byte(leaf)
			}
			pathSec := pathSection(&tt.path)
			hashBytes := tt.hash.Bytes()

			assert.Equal(t, byte(tt.prefix), want[0], "old: first byte must be the prefix")

			if felt.IsZero(&tt.owner) {
				// Old: [prefix][nodeType][pathSection][hash32]
				wantLen := 1 + 1 + len(pathSec) + len(hashBytes)
				assert.Equal(t, wantLen, len(want), "old: wrong key length for zero owner")
				assert.Equal(t, nodeTypeByte, want[1], "old: second byte must be node type")
				pEnd := 2 + len(pathSec)
				assert.Equal(t, pathSec, []byte(want[2:pEnd]), "old: path section after node type")
				assert.Equal(t, hashBytes[:], want[pEnd:], "old: hash must be last 32 bytes")
			} else {
				// Old: [prefix][owner32][nodeType][pathSection][hash32]
				ownerBytes := tt.owner.Bytes()
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
