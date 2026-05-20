package trie

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

const (
	binaryNodeTag byte = 0x01
	edgeNodeTag   byte = 0x02

	valueNodeBlobSize  = felt.Bytes
	binaryNodeBlobSize = 1 + 2*felt.Bytes
	edgeNodeMinSize    = 1 + felt.Bytes + 1
	edgeNodeMaxSize    = 1 + felt.Bytes + trieutils.MaxBitArraySize

	// nonLeafByte and leafByte mirror the on-disk byte values of trieutils'
	// unexported leafType enum (core/trie2/trieutils/types.go). They're part
	// of the persisted trie format so they cannot change; replicated locally
	// rather than exposed publicly from trieutils. TestMigrationEndToEnd's
	// byte-for-byte equality with a natively-built trie2 guards against drift.
	nonLeafByte byte = 1
	leafByte    byte = 2

	// maxNodeKeySize is the maximum byte length of a new-format trie node key:
	// 1 (prefix) + 32 (owner, optional) + 1 (nodeType) + MaxBitArraySize (path).
	maxNodeKeySize = 1 + 32 + 1 + trieutils.MaxBitArraySize
)

func toNewPath(old *trie.BitArray) trieutils.Path {
	b := old.Bytes()
	var p trieutils.Path
	p.SetBytes(old.Len(), b[:])
	return p
}

func encodeBinaryNode(leftEdgeHash, rightEdgeHash *felt.Felt) [binaryNodeBlobSize]byte {
	var blob [binaryNodeBlobSize]byte
	blob[0] = binaryNodeTag
	lb := leftEdgeHash.Bytes()
	rb := rightEdgeHash.Bytes()
	copy(blob[1:], lb[:])
	copy(blob[1+felt.Bytes:], rb[:])
	return blob
}

func encodeEdgeNodeInto(dst []byte, childHash *felt.Felt, pathSeg *trieutils.Path) int {
	encoded := pathSeg.EncodedBytes()
	dst[0] = edgeNodeTag
	h := childHash.Bytes()
	copy(dst[1:], h[:])
	copy(dst[1+felt.Bytes:], encoded)
	return 1 + felt.Bytes + len(encoded)
}

func computeEdgeHash(childHash *felt.Felt, path *trieutils.Path, hashFn crypto.HashFn) felt.Felt {
	if path.Len() == 0 {
		return *childHash
	}
	pathFelt := path.Felt()
	h := hashFn(childHash, &pathFelt)
	lenFelt := felt.FromUint64[felt.Felt](uint64(path.Len()))
	h.Add(&h, &lenFelt)
	return h
}

func compressedSegment(childFullPath *trie.BitArray, parentLen uint8) trieutils.Path {
	var seg trie.BitArray
	seg.LSBs(childFullPath, parentLen+1)
	return toNewPath(&seg)
}

// encodeNodeKey writes the node key into dst and returns the number of bytes
// written. dst must have at least maxNodeKeySize bytes of capacity. Mirrors
// trieutils.nodeKeyByPath without the per-call allocation.
func encodeNodeKey(
	dst []byte,
	bucket db.Bucket,
	owner *felt.Address,
	path *trieutils.Path,
	isLeaf bool,
) int {
	n := 0
	dst[n] = byte(bucket)
	n++

	if !felt.IsZero(owner) {
		ownerBytes := owner.Bytes()
		copy(dst[n:], ownerBytes[:])
		n += 32
	}

	if isLeaf {
		dst[n] = leafByte
	} else {
		dst[n] = nonLeafByte
	}
	n++

	pathBytes := path.EncodedBytes()
	copy(dst[n:], pathBytes)
	n += len(pathBytes)

	return n
}
