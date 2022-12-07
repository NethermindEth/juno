package db

import "bytes"

// Badger does not support buckets like Bolt or MDBX does.
// So, one needs to keep track of a global prefix list and use them for queries
// to differentiate between different groups of Keys as a poor man's bucket alternative.
const (
	State             byte = 0 // state metadata
	StateTrie         byte = 1 // state trie
	ContractRootPath  byte = 2 // contract storage roots
	ContractClassHash byte = 3 // mapping between contract addresses and class hashes
)

// Key flattens a prefix and series of byte arrays into a single []byte
func Key(prefix byte, key ...[]byte) []byte {
	return append([]byte{prefix}, bytes.Join(key, []byte{})...)
}
