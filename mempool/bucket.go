package mempool

import "slices"

//go:generate go run github.com/dmarkham/enumer -type=Bucket -output=buckets_enumer.go
type Bucket byte

// Pebble does not support buckets to differentiate between groups of
// keys like Bolt or MDBX does. We use a global prefix list as a poor
// man's bucket alternative.
const (
	Head   Bucket = iota // key of the head node
	Tail                 // key of the tail node
	Length               // number of transactions
	Node
)

// Key flattens a prefix and series of byte arrays into a single []byte.
func (b Bucket) Key(key ...[]byte) []byte {
	return append([]byte{byte(b)}, slices.Concat(key...)...)
}
