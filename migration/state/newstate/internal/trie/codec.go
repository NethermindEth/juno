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
	maxNodeKeySize     = 1 + felt.Bytes + 1 + trieutils.MaxBitArraySize

	nonLeafByte byte = 1
	leafByte    byte = 2
)

//
// --- Encoding related helpers ---
//

// encodeBinaryNode writes a binary-node blob into dst.
// dst must have at least binaryNodeBlobSize bytes of capacity.
func encodeBinaryNode(dst []byte, leftEdgeHash, rightEdgeHash *felt.Felt) int {
	dst[0] = binaryNodeTag
	lb := leftEdgeHash.Bytes()
	rb := rightEdgeHash.Bytes()
	copy(dst[1:], lb[:])
	copy(dst[1+felt.Bytes:], rb[:])
	return binaryNodeBlobSize
}

// encodeEdgeNode writes an edge-node blob into dst.
// dst must have at least edgeNodeMaxSize bytes of capacity.
func encodeEdgeNode(dst []byte, childHash *felt.Felt, pathSeg *trieutils.Path) int {
	encoded := pathSeg.EncodedBytes()
	dst[0] = edgeNodeTag
	h := childHash.Bytes()
	copy(dst[1:], h[:])
	copy(dst[1+felt.Bytes:], encoded)
	return 1 + felt.Bytes + len(encoded)
}

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

//
// --- Path related helpers ---
//

func parseDeprecatedPath(val []byte) (trie.BitArray, error) {
	if len(val) == 0 {
		return trie.BitArray{}, nil
	}
	var ba trie.BitArray
	if err := ba.UnmarshalBinary(val); err != nil {
		return trie.BitArray{}, err
	}
	return ba, nil
}

func toNewPath(old *trie.BitArray) trieutils.Path {
	b := old.Bytes()
	var p trieutils.Path
	p.SetBytes(old.Len(), b[:])
	return p
}

func compressedSegment(childFullPath *trie.BitArray, parentLen uint8) trieutils.Path {
	var seg trie.BitArray
	seg.LSBs(childFullPath, parentLen+1)
	return toNewPath(&seg)
}

//
// --- Hash related helpers ---
//

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
