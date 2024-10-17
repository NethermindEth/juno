package db

import "slices"

//go:generate go run github.com/dmarkham/enumer -type=Bucket -output=buckets_enumer.go
type Bucket byte

// Pebble does not support buckets to differentiate between groups of
// keys like Bolt or MDBX does. We use a global prefix list as a poor
// man's bucket alternative.
const (
	// StateTrie -> Latest state trie's root key
	// StateTrie + ContractAddr -> Contract's commitment value
	// StateTrie + ContractAddr + Trie node path -> Trie node value
	StateTrie         Bucket = iota
	Peer                     // Peer + PeerID bytes -> Encoded peer multiaddresses
	ContractClassHash        // (Legacy) ContractClassHash + ContractAddr -> Contract's class hash value
	// ContractStorage + ContractAddr -> Latest contract storage trie's root key
	// ContractStorage + ContractAddr + Trie node path -> Trie node value
	ContractStorage
	Class                                   // Class + Class hash -> Class object
	ContractNonce                           // (Legacy) ContractNonce + ContractAddr -> Contract's nonce value
	ChainHeight                             // ChainHeight -> Latest height of the blockchain
	BlockHeaderNumbersByHash                // BlockHeaderNumbersByHash + BlockHash -> Block number
	BlockHeadersByNumber                    // BlockHeadersByNumber + BlockNumber -> Block header object
	TransactionBlockNumbersAndIndicesByHash // TransactionBlockNumbersAndIndicesByHash + TransactionHash -> Encoded(BlockNumber, Index)
	TransactionsByBlockNumberAndIndex       // TransactionsByBlockNumberAndIndex + Encoded(BlockNumber, Index) -> Encoded(Transaction)
	ReceiptsByBlockNumberAndIndex           // ReceiptsByBlockNumberAndIndex + Encoded(BlockNumber, Index) -> Encoded(Receipt)
	StateUpdatesByBlockNumber               // StateUpdatesByBlockNumber + BlockNumber -> Encoded(StateUpdate)
	// ClassesTrie -> Latest classes trie's root key
	// ClassesTrie + ClassHash -> PoseidonHash(leafVersion, compiledClassHash)
	ClassesTrie
	ContractStorageHistory   // ContractStorageHistory + ContractAddr + BlockHeight + StorageLocation -> StorageValue
	ContractNonceHistory     // ContractNonceHistory + ContractAddr + BlockHeight -> Contract's nonce value
	ContractClassHashHistory // ContractClassHashHistory + ContractAddr + BlockHeight -> Contract's class hash value
	ContractDeploymentHeight // (Legacy) ContractDeploymentHeight + ContractAddr -> BlockHeight
	L1Height                 // L1Height -> Latest height of the L1 chain
	SchemaVersion            // SchemaVersion -> DB schema version
	Pending                  // Pending -> Pending block
	BlockCommitments         // BlockCommitments + BlockNumber -> Block commitments
	Temporary                // used temporarily for migrations
	SchemaIntermediateState  // used for db schema metadata
	Contract                 // Contract + ContractAddr -> Encoded(Contract)
)

// Key flattens a prefix and series of byte arrays into a single []byte.
func (b Bucket) Key(key ...[]byte) []byte {
	return append([]byte{byte(b)}, slices.Concat(key...)...)
}
