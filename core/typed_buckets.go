package core

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/prefix"
	"github.com/NethermindEth/juno/db/typed/value"
)

// TODO: Bucket 0 and 3 are trie 1 / state 1 buckets.
// TODO: Bucket 1 is peer bucket, needs to write marshaler.

// Bucket 2: Contract class (Address) -> Contract class hash (ClassHash)
// TODO: Integrate this bucket
var ContractClassHashBucket = typed.NewBucket(
	db.ContractClassHash,
	key.Address,
	value.ClassHash,
)

// Bucket 4: Class hash (ClassHash) -> Class definition (DeclaredClassDefinition)
// TODO: Integrate this bucket
var ClassBucket = typed.NewBucket(
	db.Class,
	key.ClassHash,
	value.Binary[DeclaredClassDefinition](),
)

// Bucket 5: Contract address (Address) -> Contract nonce (Felt)
// TODO: Integrate this bucket
var ContractNonceBucket = typed.NewBucket(
	db.ContractNonce,
	key.Address,
	value.Felt,
)

// Bucket 6: Chain height (uint64)
// TODO: Integrate this bucket
var ChainHeightBucket = typed.NewBucket(
	db.ChainHeight,
	key.Empty,
	value.Uint64,
)

// Bucket 7: Block hash (Felt) -> Block number (uint64)
// TODO: Integrate this bucket
var BlockHeaderNumbersByHashBucket = typed.NewBucket(
	db.BlockHeaderNumbersByHash,
	key.Felt,
	value.Uint64,
)

// Bucket 8: Block number (uint64) -> Header
// TODO: Integrate this bucket
var BlockHeadersByNumberBucket = typed.NewBucket(
	db.BlockHeadersByNumber,
	key.Uint64,
	value.Cbor[Header](),
)

// Bucket 9: Transaction hash (TransactionHash) -> Block number and index (BlockNumIndexKey)
var TransactionBlockNumbersAndIndicesByHashBucket = typed.NewBucket(
	db.TransactionBlockNumbersAndIndicesByHash,
	key.TransactionHash,
	value.Binary[db.BlockNumIndexKey](),
)

// Bucket 10: Block number (uint64) -> Transaction index (uint64) -> Transaction
var TransactionsByBlockNumberAndIndexBucket = prefix.NewPrefixedBucket(
	typed.NewBucket(
		db.TransactionsByBlockNumberAndIndex,
		key.Marshal[db.BlockNumIndexKey](),
		value.Cbor[Transaction](),
	),
	prefix.Prefix(key.Uint64, prefix.Prefix(key.Uint64, prefix.End[Transaction]())),
)

// Bucket 11: Block number (uint64) -> Transaction index (uint64) -> TransactionReceipt
var ReceiptsByBlockNumberAndIndexBucket = prefix.NewPrefixedBucket(
	typed.NewBucket(
		db.ReceiptsByBlockNumberAndIndex,
		key.Marshal[db.BlockNumIndexKey](),
		value.Cbor[TransactionReceipt](),
	),
	prefix.Prefix(key.Uint64, prefix.Prefix(key.Uint64, prefix.End[TransactionReceipt]())),
)

// Bucket 12: Block number (uint64) -> StateUpdate
// TODO: Integrate this bucket
var StateUpdatesByBlockNumberBucket = typed.NewBucket(
	db.StateUpdatesByBlockNumber,
	key.Uint64,
	value.Cbor[StateUpdate](),
)

// TODO: Bucket 13 -> 16 are trie 1 / state 1 buckets.

// Bucket 17: Contract address (Address) -> Deployment height (uint64)
// TODO: Integrate this bucket
var ContractDeploymentHeightBucket = typed.NewBucket(
	db.ContractDeploymentHeight,
	key.Address,
	value.Uint64,
)

// Bucket 18: L1 height (uint64) -> L1 head (L1Head)
// TODO: Integrate this bucket
var L1HeightBucket = typed.NewBucket(
	db.L1Height,
	key.Empty,
	value.Cbor[L1Head](),
)

// TODO: Bucket 19, 20, 22, 23 are buckets only used for migrations.

// Bucket 21: Block number (uint64) -> Block commitments (BlockCommitments)
// TODO: Integrate this bucket
var BlockCommitmentsBucket = typed.NewBucket(
	db.BlockCommitments,
	key.Uint64,
	value.Cbor[BlockCommitments](),
)

// Bucket 24: L1 handler msg hash ([]byte) -> L1 handler txn hash (Felt)
// TODO: Integrate this bucket
var L1HandlerTxnHashByMsgHashBucket = typed.NewBucket(
	db.L1HandlerTxnHashByMsgHash,
	key.Bytes,
	value.Hash,
)

// TODO: Bucket 25 -> 28 are sequencer mempool buckets, containing private types.
// TODO: Bucket 29 -> 36 are trie 2 / state 2 buckets.

// Bucket 37: AggregatedBloomFilterRangeKey -> AggregatedBloomFilter
// TODO: Integrate this bucket
var AggregatedBloomFiltersBucket = typed.NewBucket(
	db.AggregatedBloomFilters,
	key.Marshal[db.AggregatedBloomFilterRangeKey](),
	value.Cbor[AggregatedBloomFilter](),
)

// Bucket 38: RunningEventFilter
// TODO: Integrate this bucket
var RunningEventFilterBucket = typed.NewBucket(
	db.RunningEventFilter,
	key.Empty,
	value.Cbor[RunningEventFilter](),
)

// Bucket 39: Sierra Class Hash -> Class CASM hash metadata
var ClassCasmHashMetadataBucket = typed.NewBucket(
	db.ClassCasmHashMetadata,
	key.SierraClassHash,
	value.Binary[ClassCasmHashMetadata](),
)

// Bucket 40: Block number (uint64) -> Block transactions (BlockTransactions)
var BlockTransactionsBucket = typed.NewBucket(
	db.BlockTransactions,
	key.Cbor[uint64](),
	BlockTransactionsSerializer{},
)
