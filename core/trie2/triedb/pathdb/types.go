package pathdb

type leafType uint8

const (
	nonLeaf leafType = iota + 1
	leaf
)

func (l leafType) Bytes() []byte {
	return []byte{byte(l)}
}
