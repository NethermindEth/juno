package db

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBucketValues_Snapshot(t *testing.T) {
	// Block
	assert.EqualValues(t, blockCommitments, BlockCommitments)
	assert.EqualValues(t, blockHeaderNumbersByHash, BlockHeaderNumbersByHash)
	assert.EqualValues(t, blockHeadersByNumber, BlockHeadersByNumber)
	assert.EqualValues(t, blockTransactions, BlockTransactions)
	assert.EqualValues(t, chainHeight, ChainHeight)
	assert.EqualValues(t, l1HandlerTxnHashByMsgHash, L1HandlerTxnHashByMsgHash)
	assert.EqualValues(t, l1Height, L1Height)
	assert.EqualValues(t, receiptsByBlockNumberAndIndex, ReceiptsByBlockNumberAndIndex)
	assert.EqualValues(t, transactionBlockNumbersAndIndicesByHash, TransactionBlockNumbersAndIndicesByHash)
	assert.EqualValues(t, transactionsByBlockNumberAndIndex, TransactionsByBlockNumberAndIndex)

	// Class
	assert.EqualValues(t, class, Class)
	assert.EqualValues(t, classCasmHashMetadata, ClassCasmHashMetadata)
	assert.EqualValues(t, classesTrie, ClassesTrie)
	assert.EqualValues(t, classTrie, ClassTrie)

	// Contract
	assert.EqualValues(t, contract, Contract)
	assert.EqualValues(t, contractClassHash, ContractClassHash)
	assert.EqualValues(t, contractNonce, ContractNonce)
	assert.EqualValues(t, contractStorage, ContractStorage)
	assert.EqualValues(t, contractDeploymentHeight, ContractDeploymentHeight)
	assert.EqualValues(t, contractTrieContract, ContractTrieContract)
	assert.EqualValues(t, contractTrieStorage, ContractTrieStorage)
	assert.EqualValues(t, contractClassHashHistory, ContractClassHashHistory)
	assert.EqualValues(t, contractNonceHistory, ContractNonceHistory)
	assert.EqualValues(t, contractStorageHistory, ContractStorageHistory)

	// Mempool
	assert.EqualValues(t, mempoolHead, MempoolHead)
	assert.EqualValues(t, mempoolLength, MempoolLength)
	assert.EqualValues(t, mempoolNode, MempoolNode)
	assert.EqualValues(t, mempoolTail, MempoolTail)

	// Migration
	assert.EqualValues(t, temporary, Temporary)
	assert.EqualValues(t, schemaIntermediateState, SchemaIntermediateState)
	assert.EqualValues(t, schemaMetadata, SchemaMetadata)
	assert.EqualValues(t, deprecatedSchemaIntermediateState, DeprecatedSchemaIntermediateState)
	assert.EqualValues(t, deprecatedSchemaVersion, DeprecatedSchemaVersion)

	// State
	assert.EqualValues(t, persistedStateID, PersistedStateID)
	assert.EqualValues(t, stateHashToTrieRoots, StateHashToTrieRoots)
	assert.EqualValues(t, stateID, StateID)
	assert.EqualValues(t, stateTrie, StateTrie)
	assert.EqualValues(t, stateUpdatesByBlockNumber, StateUpdatesByBlockNumber)

	// Other
	assert.EqualValues(t, peer, Peer)
	assert.EqualValues(t, trieJournal, TrieJournal)
	assert.EqualValues(t, aggregatedBloomFilters, AggregatedBloomFilters)
	assert.EqualValues(t, runningEventFilter, RunningEventFilter)
	assert.EqualValues(t, unused, Unused)
}

func (b innerBucket) Key(key ...[]byte) []byte {
	return append([]byte{byte(b)}, slices.Concat(key...)...)
}

func TestBucketKeys_Snapshot(t *testing.T) {
	// Block
	assert.Equal(t, blockCommitments.Key(), BlockCommitments.Key())
	assert.Equal(t, blockHeaderNumbersByHash.Key(), BlockHeaderNumbersByHash.Key())
	assert.Equal(t, blockHeadersByNumber.Key(), BlockHeadersByNumber.Key())
	assert.Equal(t, blockTransactions.Key(), BlockTransactions.Key())
	assert.Equal(t, chainHeight.Key(), ChainHeight.Key())
	assert.Equal(t, l1HandlerTxnHashByMsgHash.Key(), L1HandlerTxnHashByMsgHash.Key())
	assert.Equal(t, l1Height.Key(), L1Height.Key())
	assert.Equal(t, receiptsByBlockNumberAndIndex.Key(), ReceiptsByBlockNumberAndIndex.Key())
	assert.Equal(t, transactionBlockNumbersAndIndicesByHash.Key(), TransactionBlockNumbersAndIndicesByHash.Key())
	assert.Equal(t, transactionsByBlockNumberAndIndex.Key(), TransactionsByBlockNumberAndIndex.Key())

	// Class
	assert.Equal(t, class.Key(), Class.Key())
	assert.Equal(t, classCasmHashMetadata.Key(), ClassCasmHashMetadata.Key())
	assert.Equal(t, classesTrie.Key(), ClassesTrie.Key())
	assert.Equal(t, classTrie.Key(), ClassTrie.Key())

	// Contract
	assert.Equal(t, contract.Key(), Contract.Key())
	assert.Equal(t, contractClassHash.Key(), ContractClassHash.Key())
	assert.Equal(t, contractNonce.Key(), ContractNonce.Key())
	assert.Equal(t, contractStorage.Key(), ContractStorage.Key())
	assert.Equal(t, contractDeploymentHeight.Key(), ContractDeploymentHeight.Key())
	assert.Equal(t, contractTrieContract.Key(), ContractTrieContract.Key())
	assert.Equal(t, contractTrieStorage.Key(), ContractTrieStorage.Key())
	assert.Equal(t, contractClassHashHistory.Key(), ContractClassHashHistory.Key())
	assert.Equal(t, contractNonceHistory.Key(), ContractNonceHistory.Key())
	assert.Equal(t, contractStorageHistory.Key(), ContractStorageHistory.Key())

	// Mempool
	assert.Equal(t, mempoolHead.Key(), MempoolHead.Key())
	assert.Equal(t, mempoolLength.Key(), MempoolLength.Key())
	assert.Equal(t, mempoolNode.Key(), MempoolNode.Key())
	assert.Equal(t, mempoolTail.Key(), MempoolTail.Key())

	// Migration
	assert.Equal(t, temporary.Key(), Temporary.Key())
	assert.Equal(t, schemaIntermediateState.Key(), SchemaIntermediateState.Key())
	assert.Equal(t, schemaMetadata.Key(), SchemaMetadata.Key())
	assert.Equal(t, deprecatedSchemaIntermediateState.Key(), DeprecatedSchemaIntermediateState.Key())
	assert.Equal(t, deprecatedSchemaVersion.Key(), DeprecatedSchemaVersion.Key())

	// State
	assert.Equal(t, persistedStateID.Key(), PersistedStateID.Key())
	assert.Equal(t, stateHashToTrieRoots.Key(), StateHashToTrieRoots.Key())
	assert.Equal(t, stateID.Key(), StateID.Key())
	assert.Equal(t, stateTrie.Key(), StateTrie.Key())
	assert.Equal(t, stateUpdatesByBlockNumber.Key(), StateUpdatesByBlockNumber.Key())

	// Other
	assert.Equal(t, peer.Key(), Peer.Key())
	assert.Equal(t, trieJournal.Key(), TrieJournal.Key())
	assert.Equal(t, aggregatedBloomFilters.Key(), AggregatedBloomFilters.Key())
	assert.Equal(t, runningEventFilter.Key(), RunningEventFilter.Key())
	assert.Equal(t, unused.Key(), Unused.Key())
}

func TestBucketString_Snapshot(t *testing.T) {
	// Block
	assert.Equal(t, blockCommitments.String(), BlockCommitments.String())
	assert.Equal(t, blockHeaderNumbersByHash.String(), BlockHeaderNumbersByHash.String())
	assert.Equal(t, blockHeadersByNumber.String(), BlockHeadersByNumber.String())
	assert.Equal(t, blockTransactions.String(), BlockTransactions.String())
	assert.Equal(t, chainHeight.String(), ChainHeight.String())
	assert.Equal(t, l1HandlerTxnHashByMsgHash.String(), L1HandlerTxnHashByMsgHash.String())
	assert.Equal(t, l1Height.String(), L1Height.String())
	assert.Equal(t, receiptsByBlockNumberAndIndex.String(), ReceiptsByBlockNumberAndIndex.String())
	assert.Equal(t, transactionBlockNumbersAndIndicesByHash.String(), TransactionBlockNumbersAndIndicesByHash.String())
	assert.Equal(t, transactionsByBlockNumberAndIndex.String(), TransactionsByBlockNumberAndIndex.String())

	// Class
	assert.Equal(t, class.String(), Class.String())
	assert.Equal(t, classCasmHashMetadata.String(), ClassCasmHashMetadata.String())
	assert.Equal(t, classesTrie.String(), ClassesTrie.String())
	assert.Equal(t, classTrie.String(), ClassTrie.String())

	// Contract
	assert.Equal(t, contract.String(), Contract.String())
	assert.Equal(t, contractClassHash.String(), ContractClassHash.String())
	assert.Equal(t, contractNonce.String(), ContractNonce.String())
	assert.Equal(t, contractStorage.String(), ContractStorage.String())
	assert.Equal(t, contractDeploymentHeight.String(), ContractDeploymentHeight.String())
	assert.Equal(t, contractTrieContract.String(), ContractTrieContract.String())
	assert.Equal(t, contractTrieStorage.String(), ContractTrieStorage.String())
	assert.Equal(t, contractClassHashHistory.String(), ContractClassHashHistory.String())
	assert.Equal(t, contractNonceHistory.String(), ContractNonceHistory.String())
	assert.Equal(t, contractStorageHistory.String(), ContractStorageHistory.String())

	// Mempool
	assert.Equal(t, mempoolHead.String(), MempoolHead.String())
	assert.Equal(t, mempoolLength.String(), MempoolLength.String())
	assert.Equal(t, mempoolNode.String(), MempoolNode.String())
	assert.Equal(t, mempoolTail.String(), MempoolTail.String())

	// Migration
	assert.Equal(t, temporary.String(), Temporary.String())
	assert.Equal(t, schemaIntermediateState.String(), SchemaIntermediateState.String())
	assert.Equal(t, schemaMetadata.String(), SchemaMetadata.String())
	assert.Equal(t, deprecatedSchemaIntermediateState.String(), DeprecatedSchemaIntermediateState.String())
	assert.Equal(t, deprecatedSchemaVersion.String(), DeprecatedSchemaVersion.String())

	// State
	assert.Equal(t, persistedStateID.String(), PersistedStateID.String())
	assert.Equal(t, stateHashToTrieRoots.String(), StateHashToTrieRoots.String())
	assert.Equal(t, stateID.String(), StateID.String())
	assert.Equal(t, stateTrie.String(), StateTrie.String())
	assert.Equal(t, stateUpdatesByBlockNumber.String(), StateUpdatesByBlockNumber.String())

	// Other
	assert.Equal(t, peer.String(), Peer.String())
	assert.Equal(t, trieJournal.String(), TrieJournal.String())
	assert.Equal(t, aggregatedBloomFilters.String(), AggregatedBloomFilters.String())
	assert.Equal(t, runningEventFilter.String(), RunningEventFilter.String())
	assert.Equal(t, unused.String(), Unused.String())
}

func TestBucketIsABucket_Snapshot(t *testing.T) {
	// Block
	assert.Equal(t, blockCommitments.IsAinnerBucket(), BlockCommitments.IsABucket())
	assert.Equal(t, blockHeaderNumbersByHash.IsAinnerBucket(), BlockHeaderNumbersByHash.IsABucket())
	assert.Equal(t, blockHeadersByNumber.IsAinnerBucket(), BlockHeadersByNumber.IsABucket())
	assert.Equal(t, blockTransactions.IsAinnerBucket(), BlockTransactions.IsABucket())
	assert.Equal(t, chainHeight.IsAinnerBucket(), ChainHeight.IsABucket())
	assert.Equal(t, l1HandlerTxnHashByMsgHash.IsAinnerBucket(), L1HandlerTxnHashByMsgHash.IsABucket())
	assert.Equal(t, l1Height.IsAinnerBucket(), L1Height.IsABucket())
	assert.Equal(t, receiptsByBlockNumberAndIndex.IsAinnerBucket(), ReceiptsByBlockNumberAndIndex.IsABucket())
	assert.Equal(t, transactionBlockNumbersAndIndicesByHash.IsAinnerBucket(), TransactionBlockNumbersAndIndicesByHash.IsABucket())
	assert.Equal(t, transactionsByBlockNumberAndIndex.IsAinnerBucket(), TransactionsByBlockNumberAndIndex.IsABucket())

	// Class
	assert.Equal(t, class.IsAinnerBucket(), Class.IsABucket())
	assert.Equal(t, classCasmHashMetadata.IsAinnerBucket(), ClassCasmHashMetadata.IsABucket())
	assert.Equal(t, classesTrie.IsAinnerBucket(), ClassesTrie.IsABucket())
	assert.Equal(t, classTrie.IsAinnerBucket(), ClassTrie.IsABucket())

	// Contract
	assert.Equal(t, contract.IsAinnerBucket(), Contract.IsABucket())
	assert.Equal(t, contractClassHash.IsAinnerBucket(), ContractClassHash.IsABucket())
	assert.Equal(t, contractNonce.IsAinnerBucket(), ContractNonce.IsABucket())
	assert.Equal(t, contractStorage.IsAinnerBucket(), ContractStorage.IsABucket())
	assert.Equal(t, contractDeploymentHeight.IsAinnerBucket(), ContractDeploymentHeight.IsABucket())
	assert.Equal(t, contractTrieContract.IsAinnerBucket(), ContractTrieContract.IsABucket())
	assert.Equal(t, contractTrieStorage.IsAinnerBucket(), ContractTrieStorage.IsABucket())
	assert.Equal(t, contractClassHashHistory.IsAinnerBucket(), ContractClassHashHistory.IsABucket())
	assert.Equal(t, contractNonceHistory.IsAinnerBucket(), ContractNonceHistory.IsABucket())
	assert.Equal(t, contractStorageHistory.IsAinnerBucket(), ContractStorageHistory.IsABucket())

	// Mempool
	assert.Equal(t, mempoolHead.IsAinnerBucket(), MempoolHead.IsABucket())
	assert.Equal(t, mempoolLength.IsAinnerBucket(), MempoolLength.IsABucket())
	assert.Equal(t, mempoolNode.IsAinnerBucket(), MempoolNode.IsABucket())
	assert.Equal(t, mempoolTail.IsAinnerBucket(), MempoolTail.IsABucket())

	// Migration
	assert.Equal(t, temporary.IsAinnerBucket(), Temporary.IsABucket())
	assert.Equal(t, schemaIntermediateState.IsAinnerBucket(), SchemaIntermediateState.IsABucket())
	assert.Equal(t, schemaMetadata.IsAinnerBucket(), SchemaMetadata.IsABucket())
	assert.Equal(t, deprecatedSchemaIntermediateState.IsAinnerBucket(), DeprecatedSchemaIntermediateState.IsABucket())
	assert.Equal(t, deprecatedSchemaVersion.IsAinnerBucket(), DeprecatedSchemaVersion.IsABucket())

	// State
	assert.Equal(t, persistedStateID.IsAinnerBucket(), PersistedStateID.IsABucket())
	assert.Equal(t, stateHashToTrieRoots.IsAinnerBucket(), StateHashToTrieRoots.IsABucket())
	assert.Equal(t, stateID.IsAinnerBucket(), StateID.IsABucket())
	assert.Equal(t, stateTrie.IsAinnerBucket(), StateTrie.IsABucket())
	assert.Equal(t, stateUpdatesByBlockNumber.IsAinnerBucket(), StateUpdatesByBlockNumber.IsABucket())

	// Other
	assert.Equal(t, peer.IsAinnerBucket(), Peer.IsABucket())
	assert.Equal(t, trieJournal.IsAinnerBucket(), TrieJournal.IsABucket())
	assert.Equal(t, aggregatedBloomFilters.IsAinnerBucket(), AggregatedBloomFilters.IsABucket())
	assert.Equal(t, runningEventFilter.IsAinnerBucket(), RunningEventFilter.IsABucket())
	assert.Equal(t, unused.IsAinnerBucket(), Unused.IsABucket())
}

func TestBucketStringParse_Snapshot(t *testing.T) {
	// Block
	assertBucketStringParse(t, blockCommitments)
	assertBucketStringParse(t, blockHeaderNumbersByHash)
	assertBucketStringParse(t, blockHeadersByNumber)
	assertBucketStringParse(t, blockTransactions)
	assertBucketStringParse(t, chainHeight)
	assertBucketStringParse(t, l1HandlerTxnHashByMsgHash)
	assertBucketStringParse(t, l1Height)
	assertBucketStringParse(t, receiptsByBlockNumberAndIndex)
	assertBucketStringParse(t, transactionBlockNumbersAndIndicesByHash)
	assertBucketStringParse(t, transactionsByBlockNumberAndIndex)

	// Class
	assertBucketStringParse(t, class)
	assertBucketStringParse(t, classCasmHashMetadata)
	assertBucketStringParse(t, classesTrie)
	assertBucketStringParse(t, classTrie)

	// Contract
	assertBucketStringParse(t, contract)
	assertBucketStringParse(t, contractClassHash)
	assertBucketStringParse(t, contractNonce)
	assertBucketStringParse(t, contractStorage)
	assertBucketStringParse(t, contractDeploymentHeight)
	assertBucketStringParse(t, contractTrieContract)
	assertBucketStringParse(t, contractTrieStorage)
	assertBucketStringParse(t, contractClassHashHistory)
	assertBucketStringParse(t, contractNonceHistory)
	assertBucketStringParse(t, contractStorageHistory)

	// Mempool
	assertBucketStringParse(t, mempoolHead)
	assertBucketStringParse(t, mempoolLength)
	assertBucketStringParse(t, mempoolNode)
	assertBucketStringParse(t, mempoolTail)

	// Migration
	assertBucketStringParse(t, temporary)
	assertBucketStringParse(t, schemaIntermediateState)
	assertBucketStringParse(t, schemaMetadata)
	assertBucketStringParse(t, deprecatedSchemaIntermediateState)
	assertBucketStringParse(t, deprecatedSchemaVersion)

	// State
	assertBucketStringParse(t, persistedStateID)
	assertBucketStringParse(t, stateHashToTrieRoots)
	assertBucketStringParse(t, stateID)
	assertBucketStringParse(t, stateTrie)
	assertBucketStringParse(t, stateUpdatesByBlockNumber)

	// Other
	assertBucketStringParse(t, peer)
	assertBucketStringParse(t, trieJournal)
	assertBucketStringParse(t, aggregatedBloomFilters)
	assertBucketStringParse(t, runningEventFilter)
	assertBucketStringParse(t, unused)
}

func assertBucketStringParse(t *testing.T, b innerBucket) {
	t.Helper()
	name := b.String()
	innerResult, innerErr := innerBucketString(name)
	bucketResult, bucketErr := BucketString(name)
	assert.Equal(t, innerErr, bucketErr)
	assert.EqualValues(t, innerResult, bucketResult)
}

func TestBucketValuesFunc_Snapshot(t *testing.T) {
	assert.Equal(t, innerBucketValues(), BucketValues())
}

func TestBucketStringsFunc_Snapshot(t *testing.T) {
	assert.Equal(t, innerBucketStrings(), BucketStrings())
}
