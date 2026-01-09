package trieutils

type Path = BitArray // Represents the path from the root to the node in the trie

const PathSize = MaxBitArraySize

type leafType uint8

const (
	nonLeaf leafType = iota + 1
	leaf
)

func (l leafType) Byte() byte {
	return byte(l)
}
