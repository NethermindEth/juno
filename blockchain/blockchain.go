package blockchain

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

type L1HeadSubscription struct {
	*feed.Subscription[*core.L1Head]
}

//go:generate mockgen -destination=../mocks/mock_blockchain.go -package=mocks github.com/NethermindEth/juno/blockchain Reader
type Reader interface {
	StateProvider
	Height() (height uint64, err error)

	Head() (head *core.Block, err error)
	L1Head() (*core.L1Head, error)
	SubscribeL1Head() L1HeadSubscription
	BlockByNumber(number uint64) (block *core.Block, err error)
	BlockByHash(hash *felt.Felt) (block *core.Block, err error)

	HeadsHeader() (header *core.Header, err error)
	BlockHeaderByNumber(number uint64) (header *core.Header, err error)
	BlockHeaderByHash(hash *felt.Felt) (header *core.Header, err error)

	TransactionByHash(hash *felt.Felt) (transaction core.Transaction, err error)
	TransactionByBlockNumberAndIndex(blockNumber, index uint64) (transaction core.Transaction, err error)
	Receipt(hash *felt.Felt) (receipt *core.TransactionReceipt, blockHash *felt.Felt, blockNumber uint64, err error)
	StateUpdateByNumber(number uint64) (update *core.StateUpdate, err error)
	StateUpdateByHash(hash *felt.Felt) (update *core.StateUpdate, err error)
	L1HandlerTxnHash(msgHash *common.Hash) (l1HandlerTxnHash felt.Felt, err error)

	BlockCommitmentsByNumber(blockNumber uint64) (*core.BlockCommitments, error)

	EventFilter(from *felt.Felt, keys [][]felt.Felt, pendingBlockFn func() *core.Block) (EventFilterer, error)

	Network() *utils.Network
}

type StateProvider interface {
	HeadState() (state.StateReader, error)
	StateAtBlockHash(blockHash *felt.Felt) (state.StateReader, error)
	StateAtBlockNumber(blockNumber uint64) (state.StateReader, error)
}

var (
	ErrParentDoesNotMatchHead = errors.New("block's parent hash does not match head block hash")
	SupportedStarknetVersion  = semver.MustParse("0.14.0")
)

func CheckBlockVersion(protocolVersion string) error {
	blockVer, err := core.ParseBlockVersion(protocolVersion)
	if err != nil {
		return err
	}

	// We ignore changes in patch part of the version
	blockVerMM, supportedVerMM := copyWithoutPatch(blockVer), copyWithoutPatch(SupportedStarknetVersion)
	if blockVerMM.GreaterThan(supportedVerMM) {
		return errors.New("unsupported block version")
	}

	return nil
}

func copyWithoutPatch(v *semver.Version) *semver.Version {
	if v == nil {
		return nil
	}

	return semver.New(v.Major(), v.Minor(), 0, v.Prerelease(), v.Metadata())
}

var _ Reader = (*Blockchain)(nil)

// Blockchain is responsible for keeping track of all things related to the Starknet blockchain
type Blockchain struct {
	network       *utils.Network
	database      db.KeyValueStore
	trieDB        *triedb.Database
	StateDB       *state.StateDB // TODO(weiihann): not sure if it's a good idea to expose this
	listener      EventListener
	l1HeadFeed    *feed.Feed[*core.L1Head]
	cachedFilters *AggregatedBloomFilterCache
	runningFilter *core.RunningEventFilter
}

func New(database db.KeyValueStore, network *utils.Network) *Blockchain {
	trieDB, err := triedb.New(database, nil) // TODO: handle hashdb
	if err != nil {
		panic(err)
	}
	stateDB := state.NewStateDB(database, trieDB)

	cachedFilters := NewAggregatedBloomCache(AggregatedBloomFilterCacheSize)
	fallback := func(key EventFiltersCacheKey) (core.AggregatedBloomFilter, error) {
		return core.GetAggregatedBloomFilter(database, key.fromBlock, key.toBlock)
	}
	cachedFilters.WithFallback(fallback)

	runningFilter := core.NewRunningEventFilterLazy(database)

	return &Blockchain{
		database:      database,
		trieDB:        trieDB,
		StateDB:       stateDB,
		network:       network,
		listener:      &SelectiveListener{},
		l1HeadFeed:    feed.New[*core.L1Head](),
		cachedFilters: &cachedFilters,
		runningFilter: runningFilter,
	}
}

func (b *Blockchain) WithListener(listener EventListener) *Blockchain {
	b.listener = listener
	return b
}

func (b *Blockchain) Network() *utils.Network {
	return b.network
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

func headsHeader(txn db.KeyValueReader) (*core.Header, error) {
	height, err := core.GetChainHeight(txn)
	if err != nil {
		return nil, err
	}

	return core.GetBlockHeaderByNumber(txn, height)
}

func (b *Blockchain) BlockByNumber(number uint64) (*core.Block, error) {
	b.listener.OnRead("BlockByNumber")
	return core.GetBlockByNumber(b.database, number)
}

func (b *Blockchain) BlockHeaderByNumber(number uint64) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByNumber")
	return core.GetBlockHeaderByNumber(b.database, number)
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
	return core.GetTxByBlockNumIndex(b.database, blockNumber, index)
}

// TransactionByHash gets the transaction for a given hash.
func (b *Blockchain) TransactionByHash(hash *felt.Felt) (core.Transaction, error) {
	b.listener.OnRead("TransactionByHash")
	return core.GetTxByHash(b.database, hash)
}

// Receipt gets the transaction receipt for a given transaction hash.
func (b *Blockchain) Receipt(hash *felt.Felt) (*core.TransactionReceipt, *felt.Felt, uint64, error) {
	b.listener.OnRead("Receipt")
	bnIndex, err := core.GetTxBlockNumIndexByHash(b.database, hash)
	if err != nil {
		return nil, nil, 0, err
	}

	receipt, err := core.GetReceiptByHash(b.database, hash)
	if err != nil {
		return nil, nil, 0, err
	}

	header, err := core.GetBlockHeaderByNumber(b.database, bnIndex.Number)
	if err != nil {
		return nil, nil, 0, err
	}

	return receipt, header.Hash, header.Number, nil
}

func (b *Blockchain) SubscribeL1Head() L1HeadSubscription {
	return L1HeadSubscription{b.l1HeadFeed.Subscribe()}
}

func (b *Blockchain) L1Head() (*core.L1Head, error) {
	b.listener.OnRead("L1Head")
	return core.GetL1Head(b.database)
}

func (b *Blockchain) SetL1Head(update *core.L1Head) error {
	b.l1HeadFeed.Send(update)
	return core.WriteL1Head(b.database, update)
}

// Store takes a block and state update and performs sanity checks before putting in the database.
func (b *Blockchain) Store(block *core.Block, blockCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class,
) error {
	// TODO(weiihann): handle unexpected shutdown
	if err := verifyBlock(b.database, block); err != nil {
		return err
	}
	state, err := state.New(stateUpdate.OldRoot, b.StateDB)
	if err != nil {
		return err
	}
	if err := state.Update(block.Number, stateUpdate, newClasses, false); err != nil {
		return err
	}
	batch := b.database.NewBatch()
	if err := core.WriteBlockHeader(batch, block.Header); err != nil {
		return err
	}
	for i, tx := range block.Transactions {
		if err := core.WriteTxAndReceipt(batch, block.Number, uint64(i), tx,
			block.Receipts[i]); err != nil {
			return err
		}
	}
	if err := core.WriteStateUpdateByBlockNum(batch, block.Number, stateUpdate); err != nil {
		return err
	}

	if err := core.WriteBlockCommitment(batch, block.Number, blockCommitments); err != nil {
		return err
	}

	if err := core.WriteL1HandlerMsgHashes(batch, block.Transactions); err != nil {
		return err
	}

	if err := core.WriteChainHeight(batch, block.Number); err != nil {
		return err
	}

	return batch.Write()
}

// VerifyBlock assumes the block has already been sanity-checked.
func (b *Blockchain) VerifyBlock(block *core.Block) error {
	return verifyBlock(b.database, block)
}

func verifyBlock(txn db.KeyValueReader, block *core.Block) error {
	if err := CheckBlockVersion(block.ProtocolVersion); err != nil {
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
	newClasses map[felt.Felt]core.Class,
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
func (b *Blockchain) HeadState() (state.StateReader, error) {
	b.listener.OnRead("HeadState")

	height, err := core.GetChainHeight(b.database)
	if err != nil {
		return nil, err
	}

	header, err := core.GetBlockHeaderByNumber(b.database, height)
	if err != nil {
		return nil, err
	}

	return state.New(header.GlobalStateRoot, b.StateDB)
}

// StateAtBlockNumber returns a StateReader that provides a stable view to the state at the given block number
func (b *Blockchain) StateAtBlockNumber(blockNumber uint64) (state.StateReader, error) {
	b.listener.OnRead("StateAtBlockNumber")

	header, err := core.GetBlockHeaderByNumber(b.database, blockNumber)
	if err != nil {
		return nil, err
	}

	return state.New(header.GlobalStateRoot, b.StateDB)
}

// StateAtBlockHash returns a StateReader that provides a stable view to the state at the given block hash
func (b *Blockchain) StateAtBlockHash(blockHash *felt.Felt) (state.StateReader, error) {
	b.listener.OnRead("StateAtBlockHash")
	if blockHash.IsZero() {
		return state.New(&felt.Zero, b.StateDB)
	}

	header, err := core.GetBlockHeaderByHash(b.database, blockHash)
	if err != nil {
		return nil, err
	}

	return state.New(header.GlobalStateRoot, b.StateDB)
}

// EventFilter returns an EventFilter object that is tied to a snapshot of the blockchain
func (b *Blockchain) EventFilter(from *felt.Felt, keys [][]felt.Felt, pendingBlockFn func() *core.Block) (EventFilterer, error) {
	b.listener.OnRead("EventFilter")
	latest, err := core.GetChainHeight(b.database)
	if err != nil {
		return nil, err
	}

	return newEventFilter(b.database, from, keys, 0, latest, pendingBlockFn, b.cachedFilters, b.runningFilter), nil
}

// RevertHead reverts the head block
func (b *Blockchain) RevertHead() error {
	blockNumber, err := core.GetChainHeight(b.database)
	if err != nil {
		return err
	}
	stateUpdate, err := core.GetStateUpdateByBlockNum(b.database, blockNumber)
	if err != nil {
		return err
	}
	state, err := state.New(stateUpdate.NewRoot, b.StateDB)
	if err != nil {
		return err
	}
	// revert state
	if err = state.Revert(blockNumber, stateUpdate); err != nil {
		return err
	}
	header, err := core.GetBlockHeaderByNumber(b.database, blockNumber)
	if err != nil {
		return err
	}
	genesisBlock := blockNumber == 0

	batch := b.database.NewBatch()
	for _, key := range [][]byte{
		db.BlockHeaderByNumberKey(header.Number),
		db.BlockHeaderNumbersByHashKey(header.Hash),
		db.BlockCommitmentsKey(header.Number),
	} {
		if err = batch.Delete(key); err != nil {
			return err
		}
	}
	if err = core.DeleteTxsAndReceipts(b.database, batch, blockNumber, header.TransactionCount); err != nil {
		return err
	}
	if err = core.DeleteStateUpdateByBlockNum(batch, blockNumber); err != nil {
		return err
	}
	if genesisBlock {
		if err := core.DeleteChainHeight(batch); err != nil {
			return err
		}
	} else {
		if err := core.WriteChainHeight(batch, blockNumber-1); err != nil {
			return err
		}
	}
	return batch.Write()
}

func (b *Blockchain) GetReverseStateDiff() (core.StateDiff, error) {
	var ret core.StateDiff

	blockNum, err := core.GetChainHeight(b.database)
	if err != nil {
		return ret, err
	}
	stateUpdate, err := core.GetStateUpdateByBlockNum(b.database, blockNum)
	if err != nil {
		return ret, err
	}
	state, err := state.New(stateUpdate.NewRoot, b.StateDB)
	if err != nil {
		return ret, err
	}

	ret, err = state.GetReverseStateDiff(blockNum, stateUpdate.StateDiff)
	if err != nil {
		return ret, err
	}

	return ret, nil
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
	newClasses map[felt.Felt]core.Class,
	sign utils.BlockSignFunc,
) (SimulateResult, error) {
	// Simulate without commit
	if err := b.updateStateRoots(block, stateUpdate, newClasses); err != nil {
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

// StoreSimulated stores the simulated block. There is no need to recomute the state roots etc
func (b *Blockchain) StoreSimulated(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.Class,
	commitments *core.BlockCommitments,
	sign utils.BlockSignFunc,
) error {
	if err := b.signBlock(block, stateUpdate, sign); err != nil {
		return err
	}
	if err := b.storeBlockData(block, stateUpdate, commitments); err != nil {
		return err
	}
	return core.WriteChainHeight(b.database, block.Number)
}

// Finalise checks the block correctness and appends it to the chain
func (b *Blockchain) Finalise(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.Class,
	sign utils.BlockSignFunc,
) error {
	if err := b.updateStateRoots(block, stateUpdate, newClasses); err != nil {
		return err
	}
	commitments, err := b.calculateBlockHash(block, stateUpdate)
	if err != nil {
		return err
	}
	if err := b.signBlock(block, stateUpdate, sign); err != nil {
		return err
	}
	if err := b.storeBlockData(block, stateUpdate, commitments); err != nil {
		return err
	}
	if err := core.WriteChainHeight(b.database, block.Number); err != nil {
		return err
	}

	return b.runningFilter.Insert(block.EventsBloom, block.Number)
}

// updateStateRoots computes and updates state roots in the block and state update
func (b *Blockchain) updateStateRoots(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.Class,
) error {
	var height uint64
	var err error
	if height, err = core.GetChainHeight(b.database); err != nil {
		height = 0
	}

	header, _ := core.GetBlockHeaderByNumber(b.database, height)
	var st *state.State
	if header != nil {
		st, err = state.New(header.GlobalStateRoot, b.StateDB)
	} else {
		st, err = state.New(&felt.Zero, b.StateDB)
	}

	if err != nil {
		return err
	}

	// Get old state root
	oldStateRoot, err := st.Commitment()
	if err != nil {
		return err
	}
	stateUpdate.OldRoot = &oldStateRoot

	// Apply state update
	if err = st.Update(block.Number, stateUpdate, newClasses, true); err != nil {
		return err
	}

	// Get new state root
	newStateRoot, err := st.Commitment()
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
	block.Hash = blockHash
	stateUpdate.BlockHash = blockHash
	return commitments, nil
}

// calculateBlockHash computes and sets the block hash and commitments
func (b *Blockchain) calculateBlockHash(block *core.Block, stateUpdate *core.StateUpdate) (*core.BlockCommitments, error) {
	blockHash, commitments, err := core.BlockHash(
		block,
		stateUpdate.StateDiff,
		b.network,
		block.SequencerAddress)
	if err != nil {
		return nil, err
	}
	block.Hash = blockHash
	stateUpdate.BlockHash = blockHash
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
	sig, err := sign(block.Hash, stateUpdate.StateDiff.Commitment())
	if err != nil {
		return err
	}

	block.Signatures = [][]*felt.Felt{sig}

	return nil
}

// storeBlockData persists all block-related data to the database
func (b *Blockchain) storeBlockData(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	commitments *core.BlockCommitments,
) error {
	// Store block header
	if err := core.WriteBlockHeader(b.database, block.Header); err != nil {
		return err
	}

	// Store transactions and receipts
	for i, tx := range block.Transactions {
		if err := core.WriteTxAndReceipt(b.database, block.Number, uint64(i), tx, block.Receipts[i]); err != nil {
			return err
		}
	}

	// Store state update
	if err := core.WriteStateUpdateByBlockNum(b.database, block.Number, stateUpdate); err != nil {
		return err
	}

	// Store block commitments
	if err := core.WriteBlockCommitment(b.database, block.Number, commitments); err != nil {
		return err
	}

	// Store L1 handler message hashes
	if err := core.WriteL1HandlerMsgHashes(b.database, block.Transactions); err != nil {
		return err
	}

	return nil
}

func (b *Blockchain) StoreGenesis(
	diff *core.StateDiff,
	classes map[felt.Felt]core.Class,
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

func (b *Blockchain) Stop() error {
	if b.trieDB.Scheme() == triedb.PathScheme {
		head, err := b.HeadsHeader()
		if err != nil {
			return err
		}

		stateUpdate, err := b.StateUpdateByNumber(head.Number)
		if err != nil {
			return err
		}

		return b.trieDB.Journal(stateUpdate.NewRoot)
	}
	return nil
}
