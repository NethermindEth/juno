package blockchain

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

type L1HeadSubscription struct {
	*feed.Subscription[*core.L1Head]
}

//go:generate mockgen -destination=../mocks/mock_blockchain.go -package=mocks github.com/NethermindEth/juno/blockchain Reader
type Reader interface {
	Height() (height uint64, err error)

	Head() (head *core.Block, err error)
	L1Head() (core.L1Head, error)
	SubscribeL1Head() L1HeadSubscription
	BlockByNumber(number uint64) (block *core.Block, err error)
	BlockByHash(hash *felt.Felt) (block *core.Block, err error)

	HeadsHeader() (header *core.Header, err error)
	BlockHeaderByNumber(number uint64) (header *core.Header, err error)
	BlockHeaderByHash(hash *felt.Felt) (header *core.Header, err error)

	BlockNumberByHash(hash *felt.Felt) (uint64, error)
	BlockNumberAndIndexByTxHash(
		hash *felt.TransactionHash,
	) (blockNumber uint64, index uint64, err error)

	TransactionByHash(hash *felt.Felt) (transaction core.Transaction, err error)
	TransactionByBlockNumberAndIndex(
		blockNumber, index uint64,
	) (transaction core.Transaction, err error)
	TransactionsByBlockNumber(blockNumber uint64) (transactions []core.Transaction, err error)

	Receipt(hash *felt.Felt) (receipt *core.TransactionReceipt, blockHash *felt.Felt, blockNumber uint64, err error)
	ReceiptByBlockNumberAndIndex(
		blockNumber, index uint64,
	) (receipt core.TransactionReceipt, blockHash *felt.Felt, err error)

	StateUpdateByNumber(number uint64) (update *core.StateUpdate, err error)
	StateUpdateByHash(hash *felt.Felt) (update *core.StateUpdate, err error)
	L1HandlerTxnHash(msgHash *common.Hash) (l1HandlerTxnHash felt.Felt, err error)

	HeadState() (core.StateReader, StateCloser, error)
	StateAtBlockHash(blockHash *felt.Felt) (core.StateReader, StateCloser, error)
	StateAtBlockNumber(blockNumber uint64) (core.StateReader, StateCloser, error)

	BlockCommitmentsByNumber(blockNumber uint64) (*core.BlockCommitments, error)

	EventFilter(
		addresses []felt.Address,
		keys [][]felt.Felt,
		pendingDataFn func() (core.PendingData, error),
	) (EventFilterer, error)

	Network() *utils.Network
}

var ErrParentDoesNotMatchHead = errors.New("block's parent hash does not match head block hash")

var _ Reader = (*Blockchain)(nil)

// Blockchain is responsible for keeping track of all things related to the Starknet blockchain
type Blockchain struct {
	network           *utils.Network
	database          db.KeyValueStore
	listener          EventListener
	l1HeadFeed        *feed.Feed[*core.L1Head]
	cachedFilters     *AggregatedBloomFilterCache
	runningFilter     *core.RunningEventFilter
	transactionLayout core.TransactionLayout
}

func New(database db.KeyValueStore, network *utils.Network) *Blockchain {
	cachedFilters := NewAggregatedBloomCache(AggregatedBloomFilterCacheSize)
	fallback := func(key EventFiltersCacheKey) (core.AggregatedBloomFilter, error) {
		return core.GetAggregatedBloomFilter(database, key.fromBlock, key.toBlock)
	}
	cachedFilters.WithFallback(fallback)

	runningFilter := core.NewRunningEventFilterLazy(database)

	return &Blockchain{
		database:          database,
		network:           network,
		listener:          &SelectiveListener{},
		l1HeadFeed:        feed.New[*core.L1Head](),
		cachedFilters:     &cachedFilters,
		runningFilter:     runningFilter,
		transactionLayout: core.TransactionLayoutPerTx, // default to per-tx for backward compatibility
	}
}

// WithTransactionLayout sets the transaction storage layout.
// If combined is true, uses combined (per-block) layout; otherwise uses per-tx layout.
func (b *Blockchain) WithTransactionLayout(combined bool) *Blockchain {
	if combined {
		b.transactionLayout = core.TransactionLayoutCombined
	} else {
		b.transactionLayout = core.TransactionLayoutPerTx
	}
	return b
}

// TransactionLayout returns the transaction storage layout used by this blockchain
func (b *Blockchain) TransactionLayout() core.TransactionLayout {
	return b.transactionLayout
}

func (b *Blockchain) WithListener(listener EventListener) *Blockchain {
	b.listener = listener
	return b
}

func (b *Blockchain) Network() *utils.Network {
	return b.network
}

// StateCommitment returns the latest block state commitment.
// If blockchain is empty zero felt is returned.
func (b *Blockchain) StateCommitment() (felt.Felt, error) {
	b.listener.OnRead("StateCommitment")
	batch := b.database.NewIndexedBatch() // this is a hack because we don't need to write to the db
	return core.NewDeprecatedState(batch).Commitment()
}

// Height returns the latest block height. If blockchain is empty nil is returned.
func (b *Blockchain) Height() (uint64, error) {
	b.listener.OnRead("Height")
	return core.GetChainHeight(b.database)
}

func (b *Blockchain) Head() (*core.Block, error) {
	b.listener.OnRead("Head")
	curHeight, err := core.GetChainHeight(b.database)
	if err != nil {
		return nil, err
	}

	txn := b.database.NewIndexedBatch()
	return b.transactionLayout.BlockByNumber(txn, curHeight)
}

func (b *Blockchain) HeadsHeader() (*core.Header, error) {
	b.listener.OnRead("HeadsHeader")
	height, err := core.GetChainHeight(b.database)
	if err != nil {
		return nil, err
	}

	return core.GetBlockHeaderByNumber(b.database, height)
}

func headsHeader(txn db.KeyValueReader) (*core.Header, error) {
	height, err := core.GetChainHeight(txn)
	if err != nil {
		return nil, err
	}

	return core.GetBlockHeaderByNumber(txn, height)
}

func (b *Blockchain) BlockByNumber(number uint64) (*core.Block, error) {
	b.listener.OnRead("BlockByNumber")
	txn := b.database.NewIndexedBatch()
	return b.transactionLayout.BlockByNumber(txn, number)
}

func (b *Blockchain) BlockHeaderByNumber(number uint64) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByNumber")
	return core.GetBlockHeaderByNumber(b.database, number)
}

func (b *Blockchain) BlockNumberByHash(hash *felt.Felt) (uint64, error) {
	b.listener.OnRead("BlockNumberByHash")
	return core.GetBlockHeaderNumberByHash(b.database, hash)
}

func (b *Blockchain) BlockByHash(hash *felt.Felt) (*core.Block, error) {
	b.listener.OnRead("BlockByHash")
	blockNum, err := core.GetBlockHeaderNumberByHash(b.database, hash)
	if err != nil {
		return nil, err
	}

	txn := b.database.NewIndexedBatch()
	return b.transactionLayout.BlockByNumber(txn, blockNum)
}

func (b *Blockchain) BlockHeaderByHash(hash *felt.Felt) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByHash")
	return core.GetBlockHeaderByHash(b.database, hash)
}

func (b *Blockchain) StateUpdateByNumber(number uint64) (*core.StateUpdate, error) {
	b.listener.OnRead("StateUpdateByNumber")
	return core.GetStateUpdateByBlockNum(b.database, number)
}

func (b *Blockchain) StateUpdateByHash(hash *felt.Felt) (*core.StateUpdate, error) {
	b.listener.OnRead("StateUpdateByHash")
	return core.GetStateUpdateByHash(b.database, hash)
}

func (b *Blockchain) L1HandlerTxnHash(msgHash *common.Hash) (felt.Felt, error) {
	b.listener.OnRead("L1HandlerTxnHash")
	return core.GetL1HandlerTxnHashByMsgHash(b.database, msgHash.Bytes())
}

// TransactionByBlockNumberAndIndex gets the transaction for a given block number and index.
func (b *Blockchain) TransactionByBlockNumberAndIndex(blockNumber, index uint64) (core.Transaction, error) {
	b.listener.OnRead("TransactionByBlockNumberAndIndex")
	return b.transactionLayout.TransactionByBlockAndIndex(b.database, blockNumber, index)
}

// TransactionByHash gets the transaction for a given hash.
func (b *Blockchain) TransactionByHash(hash *felt.Felt) (core.Transaction, error) {
	b.listener.OnRead("TransactionByHash")
	return b.transactionLayout.TransactionByHash(b.database, (*felt.TransactionHash)(hash))
}

// TransactionsByBlockNumber gets all transactions for a given block number
func (b *Blockchain) TransactionsByBlockNumber(number uint64) ([]core.Transaction, error) {
	b.listener.OnRead("TransactionsByBlockNumber")
	return b.transactionLayout.TransactionsByBlockNumber(b.database, number)
}

// BlockNumberAndIndexByTxHash gets transaction block number and index by Tx hash
func (b *Blockchain) BlockNumberAndIndexByTxHash(
	hash *felt.TransactionHash,
) (blockNumber, txIndex uint64, returnedErr error) {
	b.listener.OnRead("BlockNumberAndIndexByTxHash")
	data, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(b.database, hash)
	return data.Number, data.Index, err
}

// Receipt gets the transaction receipt for a given transaction hash.
// TODO: Return TransactionReceipt instead of *TransactionReceipt.
func (b *Blockchain) Receipt(hash *felt.Felt) (*core.TransactionReceipt, *felt.Felt, uint64, error) {
	b.listener.OnRead("Receipt")
	txHash := (*felt.TransactionHash)(hash)
	bnIndex, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(b.database, txHash)
	if err != nil {
		return nil, nil, 0, err
	}

	receipt, err := b.transactionLayout.ReceiptByBlockAndIndex(
		b.database,
		bnIndex.Number,
		bnIndex.Index,
	)
	if err != nil {
		return nil, nil, 0, err
	}

	header, err := core.GetBlockHeaderByNumber(b.database, bnIndex.Number)
	if err != nil {
		return nil, nil, 0, err
	}

	return receipt, header.Hash, header.Number, nil
}

func (b *Blockchain) ReceiptByBlockNumberAndIndex(
	blockNumber, index uint64,
) (core.TransactionReceipt, *felt.Felt, error) {
	b.listener.OnRead("ReceiptByBlockNumberAndIndex")

	receipt, err := b.transactionLayout.ReceiptByBlockAndIndex(b.database, blockNumber, index)
	if err != nil {
		return core.TransactionReceipt{}, nil, err
	}

	header, err := core.GetBlockHeaderByNumber(b.database, blockNumber)
	if err != nil {
		return core.TransactionReceipt{}, nil, err
	}

	return *receipt, header.Hash, nil
}

func (b *Blockchain) SubscribeL1Head() L1HeadSubscription {
	return L1HeadSubscription{b.l1HeadFeed.Subscribe()}
}

func (b *Blockchain) L1Head() (core.L1Head, error) {
	b.listener.OnRead("L1Head")
	l1Head, err := core.GetL1Head(b.database)
	return l1Head, err
}

func (b *Blockchain) SetL1Head(update *core.L1Head) error {
	b.l1HeadFeed.Send(update)
	return core.WriteL1Head(b.database, update)
}

// Store takes a block and state update and performs sanity checks before putting in the database.
func (b *Blockchain) Store(
	block *core.Block,
	blockCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	err := b.database.Update(func(txn db.IndexedBatch) error {
		if err := verifyBlock(txn, block); err != nil {
			return err
		}

		state := core.NewDeprecatedState(txn)
		err := state.Update(block.Number, stateUpdate, newClasses, false)
		if err != nil {
			return err
		}

		if err := core.WriteBlockHeader(txn, block.Header); err != nil {
			return err
		}

		err = b.transactionLayout.WriteTransactionsAndReceipts(
			txn,
			block.Number,
			block.Transactions,
			block.Receipts,
		)
		if err != nil {
			return err
		}

		if err := core.WriteStateUpdateByBlockNum(txn, block.Number, stateUpdate); err != nil {
			return err
		}

		if err := core.WriteBlockCommitment(txn, block.Number, blockCommitments); err != nil {
			return err
		}

		if err := core.WriteL1HandlerMsgHashes(txn, block.Transactions); err != nil {
			return err
		}

		err = storeCasmHashMetadata(
			txn,
			block.Number,
			block.ProtocolVersion,
			stateUpdate,
			newClasses,
		)
		if err != nil {
			return err
		}

		return core.WriteChainHeight(txn, block.Number)
	})
	if err != nil {
		return err
	}

	return b.runningFilter.Insert(
		block.EventsBloom,
		block.Number,
	)
}

// storeCasmHashMetadata stores CASM hash metadata for declared and migrated classes.
// See [core.ClassCasmHashMetadata]
func storeCasmHashMetadata(
	txn db.IndexedBatch,
	blockNumber uint64,
	protocolVersion string,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	ver, err := core.ParseBlockVersion(protocolVersion)
	if err != nil {
		return err
	}

	isV2Protocol := ver.GreaterThanEqual(core.Ver0_14_1)

	if isV2Protocol {
		return storeCasmHashMetadataV2(txn, blockNumber, stateUpdate)
	}

	return storeCasmHashMetadataV1(txn, blockNumber, stateUpdate, newClasses)
}

// storeCasmHashMetadataV2 stores metadata for classes declared with casm hash v2 or
// migrated from v1. casm hash v2 is after protocol version >= 0.14.1.
func storeCasmHashMetadataV2(
	txn db.IndexedBatch,
	blockNumber uint64,
	stateUpdate *core.StateUpdate,
) error {
	for sierraClassHash, casmHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		metadata := core.NewCasmHashMetadataDeclaredV2(
			blockNumber,
			(*felt.CasmClassHash)(casmHash),
		)
		err := core.WriteClassCasmHashMetadata(
			txn,
			(*felt.SierraClassHash)(&sierraClassHash),
			&metadata,
		)
		if err != nil {
			return err
		}
	}

	for sierraClassHash := range stateUpdate.StateDiff.MigratedClasses {
		metadata, err := core.GetClassCasmHashMetadata(txn, &sierraClassHash)
		if err != nil {
			return fmt.Errorf("cannot migrate class %s: metadata not found",
				sierraClassHash.String(),
			)
		}

		if err := metadata.Migrate(blockNumber); err != nil {
			return fmt.Errorf("failed to migrate class %s at block %d: %w",
				sierraClassHash.String(),
				blockNumber,
				err,
			)
		}

		err = core.WriteClassCasmHashMetadata(txn, &sierraClassHash, &metadata)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeDeclaredV1Classes stores metadata for classes declared with V1 hash (protocol < 0.14.1).
// It computes the V2 hash from the class definition.
func storeCasmHashMetadataV1(
	txn db.IndexedBatch,
	blockNumber uint64,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	for sierraClassHash, casmHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		casmHashV1 := (*felt.CasmClassHash)(casmHash)

		classDef, ok := newClasses[sierraClassHash]
		if !ok {
			return fmt.Errorf("class %s not available in newClasses at block %d",
				sierraClassHash.String(),
				blockNumber,
			)
		}

		sierraClass, ok := classDef.(*core.SierraClass)
		if !ok {
			return fmt.Errorf("class %s must be a SierraClass at block %d",
				sierraClassHash.String(),
				blockNumber,
			)
		}

		v2Hash := sierraClass.Compiled.Hash(core.HashVersionV2)
		casmHashV2 := felt.CasmClassHash(v2Hash)

		metadata := core.NewCasmHashMetadataDeclaredV1(blockNumber, casmHashV1, &casmHashV2)
		err := core.WriteClassCasmHashMetadata(
			txn,
			(*felt.SierraClassHash)(&sierraClassHash),
			&metadata,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// VerifyBlock assumes the block has already been sanity-checked.
func (b *Blockchain) VerifyBlock(block *core.Block) error {
	return verifyBlock(b.database, block)
}

func verifyBlock(txn db.KeyValueReader, block *core.Block) error {
	if err := core.CheckBlockVersion(block.ProtocolVersion); err != nil {
		return err
	}

	expectedBlockNumber := uint64(0)
	expectedParentHash := &felt.Zero

	h, err := headsHeader(txn)
	if err == nil {
		expectedBlockNumber = h.Number + 1
		expectedParentHash = h.Hash
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	if expectedBlockNumber != block.Number {
		return fmt.Errorf("expected block #%d, got block #%d", expectedBlockNumber, block.Number)
	}
	if !block.ParentHash.Equal(expectedParentHash) {
		return ErrParentDoesNotMatchHead
	}

	return nil
}

func (b *Blockchain) BlockCommitmentsByNumber(blockNumber uint64) (*core.BlockCommitments, error) {
	b.listener.OnRead("BlockCommitmentsByNumber")
	return core.GetBlockCommitmentByBlockNum(b.database, blockNumber)
}

// SanityCheckNewHeight checks integrity of a block and resulting state update
func (b *Blockchain) SanityCheckNewHeight(block *core.Block, stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) (*core.BlockCommitments, error) {
	if !block.Hash.Equal(stateUpdate.BlockHash) {
		return nil, errors.New("block hashes do not match")
	}
	if !block.GlobalStateRoot.Equal(stateUpdate.NewRoot) {
		return nil, errors.New("block's GlobalStateRoot does not match state update's NewRoot")
	}

	if err := core.VerifyClassHashes(newClasses); err != nil {
		return nil, err
	}

	return core.VerifyBlockHash(block, b.network, stateUpdate.StateDiff)
}

type StateCloser = func() error

var noopStateCloser = func() error { return nil } // TODO: remove this once we refactor the state

// HeadState returns a StateReader that provides a stable view to the latest state
func (b *Blockchain) HeadState() (core.StateReader, StateCloser, error) {
	b.listener.OnRead("HeadState")
	txn := b.database.NewIndexedBatch()

	_, err := core.GetChainHeight(txn)
	if err != nil {
		return nil, nil, err
	}

	return core.NewDeprecatedState(txn), noopStateCloser, nil
}

// StateAtBlockNumber returns a StateReader that provides
// a stable view to the state at the given block number
func (b *Blockchain) StateAtBlockNumber(
	blockNumber uint64,
) (core.StateReader, StateCloser, error) {
	b.listener.OnRead("StateAtBlockNumber")
	txn := b.database.NewIndexedBatch()

	_, err := core.GetBlockHeaderByNumber(txn, blockNumber)
	if err != nil {
		return nil, nil, err
	}

	return core.NewDeprecatedStateHistory(
		core.NewDeprecatedState(txn),
		blockNumber,
	), noopStateCloser, nil
}

// StateAtBlockHash returns a StateReader that provides
// a stable view to the state at the given block hash
func (b *Blockchain) StateAtBlockHash(
	// todo: this should be *felt.Hash or *felt.BlockHash
	blockHash *felt.Felt,
) (core.StateReader, StateCloser, error) {
	b.listener.OnRead("StateAtBlockHash")
	if blockHash.IsZero() {
		memDB := memory.New()
		txn := memDB.NewIndexedBatch()
		emptyState := core.NewDeprecatedState(txn)
		return emptyState, noopStateCloser, nil
	}

	txn := b.database.NewIndexedBatch()
	header, err := core.GetBlockHeaderByHash(txn, blockHash)
	if err != nil {
		return nil, nil, err
	}

	return core.NewDeprecatedStateHistory(
		core.NewDeprecatedState(txn),
		header.Number,
	), noopStateCloser, nil
}

// EventFilter returns an EventFilter object that is tied to a snapshot of the blockchain
func (b *Blockchain) EventFilter(
	addresses []felt.Address,
	keys [][]felt.Felt,
	pendingDataFn func() (core.PendingData, error),
) (EventFilterer, error) {
	b.listener.OnRead("EventFilter")
	latest, err := core.GetChainHeight(b.database)
	if err != nil {
		return nil, err
	}

	return newEventFilter(
		b.database,
		addresses,
		keys,
		0,
		latest,
		pendingDataFn,
		b.cachedFilters,
		b.runningFilter,
		b.transactionLayout,
	), nil
}

// RevertHead reverts the head block
func (b *Blockchain) RevertHead() error {
	return b.database.Update(b.revertHead)
}

func (b *Blockchain) GetReverseStateDiff() (core.StateDiff, error) {
	txn := b.database.NewIndexedBatch()
	blockNum, err := core.GetChainHeight(txn)
	if err != nil {
		return core.StateDiff{}, err
	}

	stateUpdate, err := core.GetStateUpdateByBlockNum(txn, blockNum)
	if err != nil {
		return core.StateDiff{}, err
	}

	state := core.NewDeprecatedState(txn)
	reverseStateDiff, err := state.GetReverseStateDiff(blockNum, stateUpdate.StateDiff)
	if err != nil {
		return core.StateDiff{}, err
	}

	return reverseStateDiff, nil
}

func (b *Blockchain) revertHead(txn db.IndexedBatch) error {
	blockNumber, err := core.GetChainHeight(txn)
	if err != nil {
		return err
	}

	stateUpdate, err := core.GetStateUpdateByBlockNum(txn, blockNumber)
	if err != nil {
		return err
	}

	state := core.NewDeprecatedState(txn)
	// revert state
	if err = state.Revert(blockNumber, stateUpdate); err != nil {
		return err
	}

	header, err := core.GetBlockHeaderByNumber(txn, blockNumber)
	if err != nil {
		return err
	}

	genesisBlock := blockNumber == 0

	// remove block header
	for _, key := range [][]byte{
		db.BlockHeaderByNumberKey(header.Number),
		db.BlockHeaderNumbersByHashKey(header.Hash),
		db.BlockCommitmentsKey(header.Number),
	} {
		if err = txn.Delete(key); err != nil {
			return err
		}
	}

	if err := b.transactionLayout.DeleteTxsAndReceipts(txn, blockNumber); err != nil {
		return err
	}

	// remove state update
	if err := core.DeleteStateUpdateByBlockNum(txn, blockNumber); err != nil {
		return err
	}

	// Revert chain height.
	if genesisBlock {
		return core.DeleteChainHeight(txn)
	}

	if err = core.WriteChainHeight(txn, blockNumber-1); err != nil {
		return err
	}

	// Remove the block events bloom from the cache
	return b.runningFilter.OnReorg()
}

type SimulateResult struct {
	BlockCommitments *core.BlockCommitments
	ConcatCount      felt.Felt
}

// Simulate returns what the new completed header and state update would be if the
// provided block was added to the chain.
func (b *Blockchain) Simulate(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
	sign utils.BlockSignFunc,
) (SimulateResult, error) {
	// Simulate without commit
	txn := b.database.NewIndexedBatch()
	defer txn.Close()

	if err := b.updateStateRoots(txn, block, stateUpdate, newClasses); err != nil {
		return SimulateResult{}, err
	}

	commitments, err := b.updateBlockHash(block, stateUpdate)
	if err != nil {
		return SimulateResult{}, err
	}

	concatCount := core.ConcatCounts(
		block.TransactionCount,
		block.EventCount,
		stateUpdate.StateDiff.Length(),
		block.L1DAMode,
	)

	if err := b.signBlock(block, stateUpdate, sign); err != nil {
		return SimulateResult{}, err
	}

	return SimulateResult{
		BlockCommitments: commitments,
		ConcatCount:      concatCount,
	}, nil
}

// Finalise checks the block correctness and appends it to the chain
func (b *Blockchain) Finalise(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
	sign utils.BlockSignFunc,
) error {
	err := b.database.Update(func(txn db.IndexedBatch) error {
		if err := b.updateStateRoots(txn, block, stateUpdate, newClasses); err != nil {
			return err
		}
		commitments, err := b.updateBlockHash(block, stateUpdate)
		if err != nil {
			return err
		}
		if err := b.signBlock(block, stateUpdate, sign); err != nil {
			return err
		}
		if err := b.storeBlockData(txn, block, stateUpdate, commitments); err != nil {
			return err
		}

		err = storeCasmHashMetadata(
			txn,
			block.Number,
			block.ProtocolVersion,
			stateUpdate,
			newClasses,
		)
		if err != nil {
			return err
		}

		return core.WriteChainHeight(txn, block.Number)
	})
	if err != nil {
		return err
	}

	return b.runningFilter.Insert(block.EventsBloom, block.Number)
}

// updateStateRoots computes and updates state roots in the block and state update
func (b *Blockchain) updateStateRoots(
	txn db.IndexedBatch,
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	state := core.NewDeprecatedState(txn)

	// Get old state root
	oldStateRoot, err := state.Commitment()
	if err != nil {
		return err
	}
	stateUpdate.OldRoot = &oldStateRoot

	// Apply state update
	if err = state.Update(block.Number, stateUpdate, newClasses, true); err != nil {
		return err
	}

	// Get new state root
	newStateRoot, err := state.Commitment()
	if err != nil {
		return err
	}

	block.GlobalStateRoot = &newStateRoot
	stateUpdate.NewRoot = block.GlobalStateRoot

	return nil
}

// updateBlockHash computes and sets the block hash and commitments
func (b *Blockchain) updateBlockHash(block *core.Block, stateUpdate *core.StateUpdate) (*core.BlockCommitments, error) {
	blockHash, commitments, err := core.BlockHash(
		block,
		stateUpdate.StateDiff,
		b.network,
		block.SequencerAddress,
	)
	if err != nil {
		return nil, err
	}
	block.Hash = &blockHash
	stateUpdate.BlockHash = &blockHash
	return commitments, nil
}

// signBlock applies the signature to the block if a signing function is provided
func (b *Blockchain) signBlock(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	sign utils.BlockSignFunc,
) error {
	if sign == nil {
		return nil
	}
	commitment := stateUpdate.StateDiff.Commitment()
	sig, err := sign(block.Hash, &commitment)
	if err != nil {
		return err
	}

	block.Signatures = [][]*felt.Felt{sig}

	return nil
}

// storeBlockData persists all block-related data to the database
func (b *Blockchain) storeBlockData(
	txn db.IndexedBatch,
	block *core.Block,
	stateUpdate *core.StateUpdate,
	commitments *core.BlockCommitments,
) error {
	// Store block header
	if err := core.WriteBlockHeader(txn, block.Header); err != nil {
		return err
	}

	err := b.transactionLayout.WriteTransactionsAndReceipts(
		txn,
		block.Number,
		block.Transactions,
		block.Receipts,
	)
	if err != nil {
		return err
	}

	// Store state update
	if err := core.WriteStateUpdateByBlockNum(txn, block.Number, stateUpdate); err != nil {
		return err
	}

	// Store block commitments
	if err := core.WriteBlockCommitment(txn, block.Number, commitments); err != nil {
		return err
	}

	// Store L1 handler message hashes
	if err := core.WriteL1HandlerMsgHashes(txn, block.Transactions); err != nil {
		return err
	}

	return nil
}

func (b *Blockchain) StoreGenesis(
	diff *core.StateDiff,
	classes map[felt.Felt]core.ClassDefinition,
) error {
	receipts := make([]*core.TransactionReceipt, 0)

	block := &core.Block{
		Header: &core.Header{
			ParentHash:       &felt.Zero,
			Number:           0,
			SequencerAddress: &felt.Zero,
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    &felt.Zero,
			L1GasPriceSTRK:   &felt.Zero,
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts,
	}
	stateUpdate := &core.StateUpdate{
		OldRoot:   &felt.Zero,
		StateDiff: diff,
	}
	newClasses := classes

	err := b.Finalise(block, stateUpdate, newClasses, nil)
	if err != nil {
		return err
	}

	return b.runningFilter.Insert(block.EventsBloom, block.Number)
}

func (b *Blockchain) WriteRunningEventFilter() error {
	return b.runningFilter.Write()
}
