package blockchain

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/blockchain/statebackend"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
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
		preConfirmedFn func() (*pending.PreConfirmed, error),
	) (EventFilterer, error)

	Network() *networks.Network
}

// Re-export statebackend types for API compatibility.
type (
	StateCloser    = statebackend.StateCloser
	SimulateResult = statebackend.SimulateResult
)

var ErrParentDoesNotMatchHead = statebackend.ErrParentDoesNotMatchHead

var _ Reader = (*Blockchain)(nil)

// Blockchain is responsible for keeping track of all things related to the Starknet blockchain
type Blockchain struct {
	network       *networks.Network
	database      db.KeyValueStore
	listener      EventListener
	l1HeadFeed    *feed.Feed[*core.L1Head]
	cachedFilters *AggregatedBloomFilterCache
	runningFilter *core.RunningEventFilter
	stateBackend  statebackend.StateBackend
}

// options holds configuration for constructing a Blockchain.
type options struct {
	listener     EventListener
	stateVersion bool
}

// Option is a functional option for configuring Blockchain options.
type Option func(*options)

// WithListener sets the event listener for the blockchain.
func WithListener(listener EventListener) Option {
	return func(o *options) {
		o.listener = listener
	}
}

// WithNewState enables the new trie2-based state backend when enabled is true.
func WithNewState(enabled bool) Option {
	return func(o *options) { o.stateVersion = enabled }
}

func New(database db.KeyValueStore, network *networks.Network, opts ...Option) *Blockchain {
	o := options{
		listener:     &SelectiveListener{},
		stateVersion: false,
	}
	for _, opt := range opts {
		opt(&o)
	}

	cachedFilters := NewAggregatedBloomCache(AggregatedBloomFilterCacheSize)
	fallback := func(key EventFiltersCacheKey) (core.AggregatedBloomFilter, error) {
		return core.GetAggregatedBloomFilter(database, key.fromBlock, key.toBlock)
	}
	cachedFilters.WithFallback(fallback)

	runningFilter := core.NewRunningEventFilterLazy(database)

	return &Blockchain{
		database:      database,
		network:       network,
		listener:      o.listener,
		l1HeadFeed:    feed.New[*core.L1Head](),
		cachedFilters: &cachedFilters,
		runningFilter: runningFilter,
		stateBackend: statebackend.New(
			database,
			runningFilter,
			network,
			o.stateVersion,
		),
	}
}

func (b *Blockchain) Network() *networks.Network {
	return b.network
}

// StateCommitment returns the latest block state commitment.
// If blockchain is empty zero felt is returned.
func (b *Blockchain) StateCommitment() (felt.Felt, error) {
	b.listener.OnRead("StateCommitment")
	return b.stateBackend.StateCommitment()
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

	return core.GetBlockByNumber(b.database, curHeight)
}

func (b *Blockchain) HeadsHeader() (*core.Header, error) {
	b.listener.OnRead("HeadsHeader")
	height, err := core.GetChainHeight(b.database)
	if err != nil {
		return nil, err
	}

	return core.GetBlockHeaderByNumber(b.database, height)
}

func (b *Blockchain) BlockByNumber(number uint64) (*core.Block, error) {
	b.listener.OnRead("BlockByNumber")
	return core.GetBlockByNumber(b.database, number)
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

	return core.GetBlockByNumber(b.database, blockNum)
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
	return core.GetTransactionByBlockAndIndex(b.database, blockNumber, index)
}

// TransactionByHash gets the transaction for a given hash.
func (b *Blockchain) TransactionByHash(hash *felt.Felt) (core.Transaction, error) {
	b.listener.OnRead("TransactionByHash")
	return core.GetTransactionByHash(b.database, (*felt.TransactionHash)(hash))
}

// TransactionsByBlockNumber gets all transactions for a given block number
func (b *Blockchain) TransactionsByBlockNumber(number uint64) ([]core.Transaction, error) {
	b.listener.OnRead("TransactionsByBlockNumber")
	return core.GetTransactionsByBlockNumber(b.database, number)
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

	receipt, err := core.GetReceiptByBlockAndIndex(
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

	receipt, err := core.GetReceiptByBlockAndIndex(b.database, blockNumber, index)
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
	return core.GetL1Head(b.database)
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
	return b.stateBackend.Store(block, blockCommitments, stateUpdate, newClasses)
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

// HeadState returns a StateReader that provides a stable view to the latest state
func (b *Blockchain) HeadState() (core.StateReader, StateCloser, error) {
	b.listener.OnRead("HeadState")
	return b.stateBackend.HeadState()
}

// StateAtBlockNumber returns a StateReader that provides
// a stable view to the state at the given block number
func (b *Blockchain) StateAtBlockNumber(
	blockNumber uint64,
) (core.StateReader, StateCloser, error) {
	b.listener.OnRead("StateAtBlockNumber")
	return b.stateBackend.StateAtBlockNumber(blockNumber)
}

// StateAtBlockHash returns a StateReader that provides
// a stable view to the state at the given block hash
func (b *Blockchain) StateAtBlockHash(
	// todo: this should be *felt.Hash or *felt.BlockHash
	blockHash *felt.Felt,
) (core.StateReader, StateCloser, error) {
	b.listener.OnRead("StateAtBlockHash")
	return b.stateBackend.StateAtBlockHash(blockHash)
}

// EventFilter returns an EventFilter object that is tied to a snapshot of the blockchain
func (b *Blockchain) EventFilter(
	addresses []felt.Address,
	keys [][]felt.Felt,
	preConfirmedFn func() (*pending.PreConfirmed, error),
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
		preConfirmedFn,
		b.cachedFilters,
		b.runningFilter,
	), nil
}

// RevertHead reverts the head block
func (b *Blockchain) RevertHead() error {
	return b.stateBackend.RevertHead()
}

func (b *Blockchain) GetReverseStateDiff() (core.StateDiff, error) {
	return b.stateBackend.GetReverseStateDiff()
}

// Simulate returns what the new completed header and state update would be if the
// provided block was added to the chain.
func (b *Blockchain) Simulate(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
	sign core.BlockSignFunc,
) (SimulateResult, error) {
	return b.stateBackend.Simulate(block, stateUpdate, newClasses, sign)
}

// Finalise checks the block correctness and appends it to the chain
func (b *Blockchain) Finalise(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
	sign core.BlockSignFunc,
) error {
	return b.stateBackend.Finalise(block, stateUpdate, newClasses, sign)
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

	return b.Finalise(block, stateUpdate, newClasses, nil)
}

func (b *Blockchain) WriteRunningEventFilter() error {
	return b.runningFilter.Write()
}
