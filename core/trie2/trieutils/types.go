package trieutils

type Path = BitArray // Represents the path from the root to the node in the trie

type leafType uint8

const (
	nonLeaf leafType = iota + 1
	leaf
)

func (l leafType) Bytes() []byte {
	return []byte{byte(l)}
}
