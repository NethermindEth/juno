package blockchain

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
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
	L1HandlerTxnHash(msgHash *common.Hash) (l1HandlerTxnHash *felt.Felt, err error)

	BlockCommitmentsByNumber(blockNumber uint64) (*core.BlockCommitments, error)

	EventFilter(from *felt.Felt, keys [][]felt.Felt) (EventFilterer, error)

	Network() *utils.Network
}

type StateProvider interface {
	HeadState() (state.StateReader, error)
	StateAtBlockHash(blockHash *felt.Felt) (state.StateReader, error)
	StateAtBlockNumber(blockNumber uint64) (state.StateReader, error)
}

var (
	ErrParentDoesNotMatchHead = errors.New("block's parent hash does not match head block hash")
	SupportedStarknetVersion  = semver.MustParse("0.13.3")
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
	network        *utils.Network
	database       db.KeyValueStore
	trieDB         *triedb.Database
	StateDB        *state.StateDB // TODO(weiihann): not sure if it's a good idea to expose this
	listener       EventListener
	l1HeadFeed     *feed.Feed[*core.L1Head]
	pendingBlockFn func() *core.Block
}

func New(database db.KeyValueStore, network *utils.Network) *Blockchain {
	trieDB := triedb.New(database, &triedb.Config{PathConfig: &pathdb.Config{}}) // TODO: handle hashdb
	stateDB := state.NewStateDB(database, trieDB)

	return &Blockchain{
		database:   database,
		trieDB:     trieDB,
		StateDB:    stateDB,
		network:    network,
		listener:   &SelectiveListener{},
		l1HeadFeed: feed.New[*core.L1Head](),
	}
}

func (b *Blockchain) WithPendingBlockFn(pendingBlockFn func() *core.Block) *Blockchain {
	b.pendingBlockFn = pendingBlockFn
	return b
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
	// TODO: we should also cache the height in the blockchain struct
	b.listener.OnRead("Height")
	return GetChainHeight(b.database)
}

func (b *Blockchain) Head() (*core.Block, error) {
	b.listener.OnRead("Head")
	curHeight, err := GetChainHeight(b.database)
	if err != nil {
		return nil, err
	}

	txn := b.database.NewIndexedBatch()
	return GetBlockByNumber(txn, curHeight)
}

func (b *Blockchain) HeadsHeader() (*core.Header, error) {
	b.listener.OnRead("HeadsHeader")
	height, err := GetChainHeight(b.database)
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(b.database, height)
}

func headsHeader(txn db.KeyValueReader) (*core.Header, error) {
	height, err := GetChainHeight(txn)
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(txn, height)
}

func (b *Blockchain) BlockByNumber(number uint64) (*core.Block, error) {
	b.listener.OnRead("BlockByNumber")
	txn := b.database.NewIndexedBatch()
	return GetBlockByNumber(txn, number)
}

func (b *Blockchain) BlockHeaderByNumber(number uint64) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByNumber")
	return GetBlockHeaderByNumber(b.database, number)
}

func (b *Blockchain) BlockByHash(hash *felt.Felt) (*core.Block, error) {
	b.listener.OnRead("BlockByHash")
	blockNum, err := GetBlockHeaderNumberByHash(b.database, hash)
	if err != nil {
		return nil, err
	}

	txn := b.database.NewIndexedBatch()
	return GetBlockByNumber(txn, blockNum)
}

func (b *Blockchain) BlockHeaderByHash(hash *felt.Felt) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByHash")
	return GetBlockHeaderByHash(b.database, hash)
}

func (b *Blockchain) StateUpdateByNumber(number uint64) (*core.StateUpdate, error) {
	b.listener.OnRead("StateUpdateByNumber")
	return GetStateUpdateByBlockNum(b.database, number)
}

func (b *Blockchain) StateUpdateByHash(hash *felt.Felt) (*core.StateUpdate, error) {
	b.listener.OnRead("StateUpdateByHash")
	return GetStateUpdateByHash(b.database, hash)
}

func (b *Blockchain) L1HandlerTxnHash(msgHash *common.Hash) (*felt.Felt, error) {
	b.listener.OnRead("L1HandlerTxnHash")
	txnHash, err := GetL1HandlerTxnHashByMsgHash(b.database, msgHash.Bytes())
	return &txnHash, err // TODO: return felt value
}

// TransactionByBlockNumberAndIndex gets the transaction for a given block number and index.
func (b *Blockchain) TransactionByBlockNumberAndIndex(blockNumber, index uint64) (core.Transaction, error) {
	b.listener.OnRead("TransactionByBlockNumberAndIndex")
	return GetTxByBlockNumIndex(b.database, blockNumber, index)
}

// TransactionByHash gets the transaction for a given hash.
func (b *Blockchain) TransactionByHash(hash *felt.Felt) (core.Transaction, error) {
	b.listener.OnRead("TransactionByHash")
	return GetTxByHash(b.database, hash)
}

// Receipt gets the transaction receipt for a given transaction hash.
func (b *Blockchain) Receipt(hash *felt.Felt) (*core.TransactionReceipt, *felt.Felt, uint64, error) {
	b.listener.OnRead("Receipt")
	bnIndex, err := GetTxBlockNumIndexByHash(b.database, hash)
	if err != nil {
		return nil, nil, 0, err
	}

	receipt, err := GetReceiptByHash(b.database, hash)
	if err != nil {
		return nil, nil, 0, err
	}

	header, err := GetBlockHeaderByNumber(b.database, bnIndex.Number)
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
	var l1Head *core.L1Head
	err := b.database.Get(db.L1Height.Key(), func(data []byte) error {
		return encoder.Unmarshal(data, &l1Head)
	})
	return l1Head, err
}

func (b *Blockchain) SetL1Head(update *core.L1Head) error {
	data, err := encoder.Marshal(update)
	if err != nil {
		return err
	}

	b.l1HeadFeed.Send(update)
	return b.database.Put(db.L1Height.Key(), data)
}

// Store takes a block and state update and performs sanity checks before putting in the database.
func (b *Blockchain) Store(block *core.Block, blockCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class,
) error {
	// TODO(weiihann): handle unexpected shutdown
	if err := verifyBlock(b.database, block); err != nil {
		return err
	}

	state, err := state.New(*stateUpdate.OldRoot, b.StateDB)
	if err != nil {
		return err
	}

	if err := state.Update(block.Number, stateUpdate, newClasses); err != nil {
		return err
	}

	batch := b.database.NewBatch()
	if err := WriteBlockHeader(batch, block.Header); err != nil {
		return err
	}

	for i, tx := range block.Transactions {
		if err := WriteTxAndReceipt(batch, block.Number, uint64(i), tx,
			block.Receipts[i]); err != nil {
			return err
		}
	}

	if err := WriteStateUpdateByBlockNum(batch, block.Number, stateUpdate); err != nil {
		return err
	}

	if err := WriteBlockCommitment(batch, block.Number, blockCommitments); err != nil {
		return err
	}

	if err := WriteL1HandlerMsgHashes(batch, block.Transactions); err != nil {
		return err
	}

	if err := WriteChainHeight(batch, block.Number); err != nil {
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
	return GetBlockCommitmentByBlockNum(b.database, blockNumber)
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
	txn := b.database.NewIndexedBatch()

	height, err := GetChainHeight(txn)
	if err != nil {
		return nil, err
	}

	header, err := GetBlockHeaderByNumber(txn, height)
	if err != nil {
		return nil, err
	}

	return state.New(*header.GlobalStateRoot, b.StateDB)
}

// StateAtBlockNumber returns a StateReader that provides a stable view to the state at the given block number
func (b *Blockchain) StateAtBlockNumber(blockNumber uint64) (state.StateReader, error) {
	b.listener.OnRead("StateAtBlockNumber")
	txn := b.database.NewIndexedBatch()

	header, err := GetBlockHeaderByNumber(txn, blockNumber)
	if err != nil {
		return nil, err
	}

	return state.NewStateHistory(blockNumber, *header.GlobalStateRoot, b.StateDB)
}

// StateAtBlockHash returns a StateReader that provides a stable view to the state at the given block hash
func (b *Blockchain) StateAtBlockHash(blockHash *felt.Felt) (state.StateReader, error) {
	b.listener.OnRead("StateAtBlockHash")
	if blockHash.IsZero() {
		return state.New(felt.Zero, b.StateDB)
	}

	txn := b.database.NewIndexedBatch()
	header, err := GetBlockHeaderByHash(txn, blockHash)
	if err != nil {
		return nil, err
	}

	return state.NewStateHistory(header.Number, *header.GlobalStateRoot, b.StateDB)
}

// EventFilter returns an EventFilter object that is tied to a snapshot of the blockchain
func (b *Blockchain) EventFilter(from *felt.Felt, keys [][]felt.Felt) (EventFilterer, error) {
	b.listener.OnRead("EventFilter")
	latest, err := GetChainHeight(b.database)
	if err != nil {
		return nil, err
	}

	return newEventFilter(b.database, from, keys, 0, latest, b.pendingBlockFn), nil
}

// RevertHead reverts the head block
func (b *Blockchain) RevertHead() error {
	blockNumber, err := GetChainHeight(b.database)
	if err != nil {
		return err
	}

	stateUpdate, err := GetStateUpdateByBlockNum(b.database, blockNumber)
	if err != nil {
		return err
	}

	state, err := state.New(*stateUpdate.NewRoot, b.StateDB)
	if err != nil {
		return err
	}

	// revert state
	if err = state.Revert(blockNumber, stateUpdate); err != nil {
		return err
	}

	header, err := GetBlockHeaderByNumber(b.database, blockNumber)
	if err != nil {
		return err
	}

	genesisBlock := blockNumber == 0

	batch := b.database.NewBatch()
	// remove block header
	for _, key := range [][]byte{
		db.BlockHeaderByNumberKey(header.Number),
		db.BlockHeaderNumbersByHashKey(header.Hash),
		db.BlockCommitmentsKey(header.Number),
	} {
		if err = batch.Delete(key); err != nil {
			return err
		}
	}

	if err = DeleteTxsAndReceipts(b.database, batch, blockNumber, header.TransactionCount); err != nil {
		return err
	}

	// remove state update
	if err = DeleteStateUpdateByBlockNum(batch, blockNumber); err != nil {
		return err
	}

	// Revert chain height.
	if genesisBlock {
		if err := DeleteChainHeight(batch); err != nil {
			return err
		}
	} else {
		if err := WriteChainHeight(batch, blockNumber-1); err != nil {
			return err
		}
	}

	return batch.Write()
}

func (b *Blockchain) GetReverseStateDiff() (core.StateDiff, error) {
	var ret core.StateDiff

	blockNum, err := GetChainHeight(b.database)
	if err != nil {
		return ret, err
	}

	stateUpdate, err := GetStateUpdateByBlockNum(b.database, blockNum)
	if err != nil {
		return ret, err
	}

	state, err := state.New(*stateUpdate.NewRoot, b.StateDB)
	if err != nil {
		return ret, err
	}

	ret, err = state.GetReverseStateDiff(blockNum, stateUpdate.StateDiff)
	if err != nil {
		return ret, err
	}

	return ret, nil
}
