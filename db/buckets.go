package db

import "slices"

// Pebble does not support buckets to differentiate between groups of
// keys like Bolt or MDBX does. We use a global prefix list as a poor
// man's bucket alternative.
type Bucket innerBucket

// Bucket constants grouped by domain. Each constant wraps its
// corresponding innerBucket value, preserving the underlying byte
// used as a key prefix in the database.
//
// When adding a new bucket, use the same name as its innerBucket
// counterpart (in PascalCase), or rename the innerBucket to match,
// for better readability, since the innerBucket name is what the
// String() method outputs.

const (
	// maps peer ID to peer multiaddresses
	Peer = Bucket(peer)

	// TrieJournal -> journal
	TrieJournal = Bucket(trieJournal)

	// maps block range to AggregatedBloomFilter
	AggregatedBloomFilters = Bucket(aggregatedBloomFilters)
	// aggregated filter not full yet
	RunningEventFilter = Bucket(runningEventFilter)

	// TODO: remove this variable when removing the deprecated migrations
	Unused = Bucket(unused)

	/****************************************************
			Block
	*****************************************************/
	BlockCommitments         = Bucket(blockCommitments)
	BlockHeaderNumbersByHash = Bucket(blockHeaderNumbersByHash)
	BlockHeadersByNumber     = Bucket(blockHeadersByNumber)
	// maps block number to transactions and receipts
	BlockTransactions = Bucket(blockTransactions)
	// Latest height of the blockchain
	ChainHeight = Bucket(chainHeight)
	// maps l1 handler msg hash to l1 handler txn hash
	L1HandlerTxnHashByMsgHash = Bucket(l1HandlerTxnHashByMsgHash)
	L1Height                  = Bucket(l1Height)
	// maps block number and index to transaction receipt
	ReceiptsByBlockNumberAndIndex = Bucket(receiptsByBlockNumberAndIndex)
	// maps transaction hashes to block number and index
	TransactionBlockNumbersAndIndicesByHash = Bucket(transactionBlockNumbersAndIndicesByHash)
	// maps block number and index to transaction
	TransactionsByBlockNumberAndIndex = Bucket(transactionsByBlockNumberAndIndex)

	/****************************************************
			Class
	*****************************************************/
	// maps class hashes to classes
	Class = Bucket(class)
	// Class CASM hash metadata (declaration and migration info)
	ClassCasmHashMetadata = Bucket(classCasmHashMetadata)
	ClassesTrie           = Bucket(classesTrie)
	// ClassTrie + nodetype + path + pathlength -> Trie Node
	ClassTrie = Bucket(classTrie)

	/****************************************************
			Contract
	*****************************************************/
	// Contract + ContractAddr -> Contract
	Contract = Bucket(contract)
	// maps contract addresses and class hashes
	ContractClassHash = Bucket(contractClassHash)
	// contract nonce
	ContractNonce = Bucket(contractNonce)
	// contract storages
	ContractStorage = Bucket(contractStorage)

	// maps contract addresses to their deployment block number
	ContractDeploymentHeight = Bucket(contractDeploymentHeight)
	// ContractTrieContract + nodetype + path + pathlength -> Trie Node
	ContractTrieContract = Bucket(contractTrieContract)
	// ContractTrieStorage + owner + nodetype + path + pathlength -> Trie Node
	ContractTrieStorage = Bucket(contractTrieStorage)

	// For these three history buckets, the block number is when the current value was set, and
	// the value is the old value before that.

	// ContractClassHashHistory + Contract address + block number -> old class hash.
	ContractClassHashHistory = Bucket(contractClassHashHistory)
	// ContractNonceHistory + Contract address + block number -> old nonce.
	ContractNonceHistory = Bucket(contractNonceHistory)
	// ContractStorageHistory + Contract address + storage location + block number -> old value.
	ContractStorageHistory = Bucket(contractStorageHistory)

	/****************************************************
			Mempool
	*****************************************************/
	// key of the head node
	MempoolHead = Bucket(mempoolHead)
	// number of transactions
	MempoolLength = Bucket(mempoolLength)
	MempoolNode   = Bucket(mempoolNode)
	// key of the tail node
	MempoolTail = Bucket(mempoolTail)

	/****************************************************
			Migration
	*****************************************************/
	// used temporarily for migrations
	Temporary                         = Bucket(temporary)
	SchemaIntermediateState           = Bucket(schemaIntermediateState)
	SchemaMetadata                    = Bucket(schemaMetadata)
	DeprecatedSchemaIntermediateState = Bucket(deprecatedSchemaIntermediateState)
	DeprecatedSchemaVersion           = Bucket(deprecatedSchemaVersion)

	/****************************************************
			State
	*****************************************************/
	// PersistedStateID -> state id
	PersistedStateID = Bucket(persistedStateID)
	// StateHash -> ClassRootHash + ContractRootHash
	StateHashToTrieRoots = Bucket(stateHashToTrieRoots)
	// StateID + root hash -> state id
	StateID = Bucket(stateID)
	// state metadata (e.g., the state root)
	StateTrie                 = Bucket(stateTrie)
	StateUpdatesByBlockNumber = Bucket(stateUpdatesByBlockNumber)
)

// Key flattens a prefix and series of byte arrays into a single []byte.
func (b Bucket) Key(key ...[]byte) []byte {
	return append([]byte{byte(b)}, slices.Concat(key...)...)
}

func (b Bucket) String() string {
	return innerBucket(b).String()
}

// BucketValues returns all bucket values, derived from the generated
// innerBucket enum, wrapped in the Bucket type.
func BucketValues() []Bucket {
	ib := innerBucketValues()
	buckets := make([]Bucket, len(ib))
	for i, b := range ib {
		buckets[i] = Bucket(b)
	}
	return buckets
}

//go:generate enumer -type=innerBucket -transform=title -output=buckets_enumer.go
type innerBucket byte

// innerBucket defines the actual byte values for each bucket using iota.
// The order of these constants must NOT be changed, as their ordinal
// values are persisted in the database.
const (
	stateTrie innerBucket = iota
	peer
	contractClassHash
	contractStorage
	class
	contractNonce
	chainHeight
	blockHeaderNumbersByHash
	blockHeadersByNumber
	transactionBlockNumbersAndIndicesByHash
	transactionsByBlockNumberAndIndex
	receiptsByBlockNumberAndIndex
	stateUpdatesByBlockNumber
	classesTrie
	contractStorageHistory
	contractNonceHistory
	contractClassHashHistory
	contractDeploymentHeight
	l1Height
	deprecatedSchemaVersion
	unused // Previously used for storing Pending Block
	blockCommitments
	temporary
	deprecatedSchemaIntermediateState
	l1HandlerTxnHashByMsgHash
	mempoolHead
	mempoolTail
	mempoolLength
	mempoolNode
	classTrie
	contractTrieContract
	contractTrieStorage
	contract
	stateHashToTrieRoots
	stateID
	persistedStateID
	trieJournal
	aggregatedBloomFilters
	runningEventFilter
	classCasmHashMetadata
	blockTransactions
	schemaMetadata
	schemaIntermediateState
)
