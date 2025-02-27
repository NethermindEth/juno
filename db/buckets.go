package db

import "slices"

//go:generate go run github.com/dmarkham/enumer -type=Bucket -output=buckets_enumer.go
type Bucket byte

// Pebble does not support buckets to differentiate between groups of
// keys like Bolt or MDBX does. We use a global prefix list as a poor
// man's bucket alternative.
const (
	StateTrie         Bucket = iota // LEGACY state metadata (e.g., the state root)
	Peer                            // maps peer ID to peer multiaddresses
	ContractClassHash               // LEGACY maps contract addresses and class hashes
	ContractStorage                 // LEGACY contract storages
	Class                           // maps class hashes to classes
	ContractNonce                   // LEGACY contract nonce
	ChainHeight                     // Latest height of the blockchain
	BlockHeaderNumbersByHash
	BlockHeadersByNumber
	TransactionBlockNumbersAndIndicesByHash // maps transaction hashes to block number and index
	TransactionsByBlockNumberAndIndex       // maps block number and index to transaction
	ReceiptsByBlockNumberAndIndex           // maps block number and index to transaction receipt
	StateUpdatesByBlockNumber
	ClassesTrie
	ContractStorageHistory   // [ContractStorageHistory] + ContractAddr + StorageLocation + BlockHeight -> StorageValue
	ContractNonceHistory     // [ContractNonceHistory] + ContractAddr + BlockHeight -> ContractNonce
	ContractClassHashHistory // [ContractClassHashHistory] + ContractAddr + BlockHeight -> ContractClassHash
	ContractDeploymentHeight // LEGACY
	L1Height
	SchemaVersion
	Unused // Previously used for storing Pending Block
	BlockCommitments
	Temporary // used temporarily for migrations
	SchemaIntermediateState
	L1HandlerTxnHashByMsgHash // maps l1 handler msg hash to l1 handler txn hash
	MempoolHead               // key of the head node
	MempoolTail               // key of the tail node
	MempoolLength             // number of transactions
	MempoolNode
	ClassTrie            // ClassTrie + nodetype + path + pathlength -> Trie Node
	ContractTrieContract // ContractTrieContract + nodetype + path + pathlength -> Trie Node
	ContractTrieStorage  // ContractTrieStorage + nodetype + path + pathlength -> Trie Node
	Contract             // Contract + ContractAddr -> Contract
)

// Key flattens a prefix and series of byte arrays into a single []byte.
func (b Bucket) Key(key ...[]byte) []byte {
	return append([]byte{byte(b)}, slices.Concat(key...)...)
}
