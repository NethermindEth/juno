package trie

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

const (
	binaryNodeTag byte = 0x01
	edgeNodeTag   byte = 0x02

	valueNodeBlobSize  = felt.Bytes
	binaryNodeBlobSize = 1 + 2*felt.Bytes
	edgeNodeMinSize    = 1 + felt.Bytes + 1
	edgeNodeMaxSize    = 1 + felt.Bytes + trieutils.MaxBitArraySize
)

func toNewPath(old *trie.BitArray) trieutils.Path {
	b := old.Bytes()
	var p trieutils.Path
	p.SetBytes(old.Len(), b[:])
	return p
}

func encodeValueNode(value *felt.Felt) [valueNodeBlobSize]byte {
	return value.Bytes()
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
