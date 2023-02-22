package db

import "bytes"

type Bucket byte

// Pebble does not support buckets to differentiate between groups of
// keys like Bolt or MDBX does. We use a global prefix list as a poor
// man's bucket alternative.
const (
	State Bucket = iota // state metadata (e.g., the state root)
	StateTrie
	ContractRootKey   // contract storage roots
	ContractClassHash // maps contract addresses and class hashes
	ContractStorage   // contract storages
	Class             // maps class hashes to classes
	ContractNonce     // contract nonce
	ChainHeight       // Latest height of the blockchain
	BlockHeaderNumbersByHash
	BlockHeadersByNumber
	TransactionBlockNumbersAndIndicesByHash // maps transaction hashes to block number and index
	TransactionsByBlockNumberAndIndex       // maps block number and index to transaction
	ReceiptsByBlockNumberAndIndex           // maps block number and index to transaction receipt
	StateUpdatesByBlockNumber
)

// Key flattens a prefix and series of byte arrays into a single []byte.
func (b Bucket) Key(key ...[]byte) []byte {
	return append([]byte{byte(b)}, bytes.Join(key, []byte{})...)
}
