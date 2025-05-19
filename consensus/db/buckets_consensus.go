package db

import "slices"

// The consensus service uses a separate DB with its own buckets
//
//go:generate go run github.com/dmarkham/enumer -type=BucketConsensus -output=buckets_consensus_enumer.go
type BucketConsensus byte

// Pebble does not support buckets to differentiate between groups of
// keys like Bolt or MDBX does. We use a global prefix list as a poor
// man's bucket alternative.
const (
	WALEntryBucket BucketConsensus = iota // key: WAL_prefix + Height + MsgIndex. Val: Encoded Tendermint consensus message.
)

// Key flattens a prefix and series of byte arrays into a single []byte.
func (b BucketConsensus) Key(key ...[]byte) []byte {
	return append([]byte{byte(b)}, slices.Concat(key...)...)
}
