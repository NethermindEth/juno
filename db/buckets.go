package db

import "slices"

type Bucket byte

// Pebble does not support buckets to differentiate between groups of
// keys like Bolt or MDBX does. We use a global prefix list as a poor
// man's bucket alternative.
const (
	StateTrie         Bucket = iota // state metadata (e.g., the state root)
	Unused                          // Previously held contract storage roots and is now unused. May be reused in the future.
	ContractClassHash               // maps contract addresses and class hashes
	ContractStorage                 // contract storages
	Class                           // maps class hashes to classes
	ContractNonce                   // contract nonce
	ChainHeight                     // Latest height of the blockchain
	BlockHeaderNumbersByHash
	BlockHeadersByNumber
	TransactionBlockNumbersAndIndicesByHash // maps transaction hashes to block number and index
	TransactionsByBlockNumberAndIndex       // maps block number and index to transaction
	ReceiptsByBlockNumberAndIndex           // maps block number and index to transaction receipt
	StateUpdatesByBlockNumber
	ClassesTrie
	ContractStorageHistory
	ContractNonceHistory
	ContractClassHashHistory
	ContractDeploymentHeight
	L1Height
	SchemaVersion
	Pending
	BlockCommitments
	Temporary // used temporarily for migrations
	SchemaIntermediateState
)

var bucketStrings = []string{
	"StateTrie",
	"Unused",
	"ContractClassHash",
	"ContractStorage",
	"Class",
	"ContractNonce",
	"ChainHeight",
	"BlockHeaderNumbersByHash",
	"BlockHeadersByNumber",
	"TransactionBlockNumbersAndIndicesByHash",
	"TransactionsByBlockNumberAndIndex",
	"ReceiptsByBlockNumberAndIndex",
	"StateUpdatesByBlockNumber",
	"ClassesTrie",
	"ContractStorageHistory",
	"ContractNonceHistory",
	"ContractClassHashHistory",
	"ContractDeploymentHeight",
	"L1Height",
	"SchemaVersion",
	"Pending",
	"BlockCommitments",
	"Temporary",
	"SchemaIntermediateState",
}

// Key flattens a prefix and series of byte arrays into a single []byte.
func (b Bucket) Key(key ...[]byte) []byte {
	return append([]byte{byte(b)}, slices.Concat(key...)...)
}

func (b Bucket) String() string {
	if int(b) < 0 || int(b) >= len(bucketStrings) {
		return "Unknown"
	}

	return bucketStrings[int(b)]
}
