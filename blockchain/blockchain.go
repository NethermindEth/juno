package blockchain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
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

	HeadState() (core.StateReader, StateCloser, error)
	StateAtBlockHash(blockHash *felt.Felt) (core.StateReader, StateCloser, error)
	StateAtBlockNumber(blockNumber uint64) (core.StateReader, StateCloser, error)

	BlockCommitmentsByNumber(blockNumber uint64) (*core.BlockCommitments, error)

	EventFilter(from *felt.Felt, keys [][]felt.Felt) (EventFilterer, error)

	Network() *utils.Network
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
	database       db.DB
	database2      db.KeyValueStore // TODO(weiihann): deal with this
	listener       EventListener
	l1HeadFeed     *feed.Feed[*core.L1Head]
	pendingBlockFn func() *core.Block
}

func New(database db.DB, network *utils.Network) *Blockchain {
	return &Blockchain{
		database:   database,
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

// StateCommitment returns the latest block state commitment.
// If blockchain is empty zero felt is returned.
func (b *Blockchain) StateCommitment() (*felt.Felt, error) {
	b.listener.OnRead("StateCommitment")
	batch := b.database2.NewIndexedBatch() // this is a hack because we don't need to write to the db
	return core.NewState2(batch).Root()
}

// Height returns the latest block height. If blockchain is empty nil is returned.
func (b *Blockchain) Height() (uint64, error) {
	b.listener.OnRead("Height")
	return GetChainHeight(b.database2)
}

func ChainHeight(txn db.Transaction) (uint64, error) {
	var height uint64
	return height, txn.Get(db.ChainHeight.Key(), func(val []byte) error {
		height = binary.BigEndian.Uint64(val)
		return nil
	})
}

func (b *Blockchain) Head() (*core.Block, error) {
	b.listener.OnRead("Head")
	curHeight, err := GetChainHeight(b.database2)
	if err != nil {
		return nil, err
	}

	return GetBlockByNumber(b.database2, curHeight)
}

func (b *Blockchain) HeadsHeader() (*core.Header, error) {
	b.listener.OnRead("HeadsHeader")
	height, err := GetChainHeight(b.database2)
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(b.database2, height)
}

func headsHeader(txn db.Transaction) (*core.Header, error) {
	height, err := ChainHeight(txn)
	if err != nil {
		return nil, err
	}

	return blockHeaderByNumber(txn, height)
}

func headsHeader2(txn db.KeyValueReader) (*core.Header, error) {
	height, err := GetChainHeight(txn)
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(txn, height)
}

func (b *Blockchain) BlockByNumber(number uint64) (*core.Block, error) {
	b.listener.OnRead("BlockByNumber")
	return GetBlockByNumber(b.database2, number)
}

func (b *Blockchain) BlockHeaderByNumber(number uint64) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByNumber")
	return GetBlockHeaderByNumber(b.database2, number)
}

func (b *Blockchain) BlockByHash(hash *felt.Felt) (*core.Block, error) {
	b.listener.OnRead("BlockByHash")
	blockNum, err := GetBlockHeaderNumberByHash(b.database2, hash)
	if err != nil {
		return nil, err
	}

	return GetBlockByNumber(b.database2, blockNum)
}

func (b *Blockchain) BlockHeaderByHash(hash *felt.Felt) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByHash")
	return GetBlockHeaderByHash(b.database2, hash)
}

func (b *Blockchain) StateUpdateByNumber(number uint64) (*core.StateUpdate, error) {
	b.listener.OnRead("StateUpdateByNumber")
	return GetStateUpdateByBlockNum(b.database2, number)
}

func (b *Blockchain) StateUpdateByHash(hash *felt.Felt) (*core.StateUpdate, error) {
	b.listener.OnRead("StateUpdateByHash")
	return GetStateUpdateByHash(b.database2, hash)
}

func (b *Blockchain) L1HandlerTxnHash(msgHash *common.Hash) (*felt.Felt, error) {
	b.listener.OnRead("L1HandlerTxnHash")
	txnHash, err := GetL1HandlerTxnHashByMsgHash(b.database2, msgHash.Bytes())
	return &txnHash, err // TODO: return felt value
}

// TransactionByBlockNumberAndIndex gets the transaction for a given block number and index.
func (b *Blockchain) TransactionByBlockNumberAndIndex(blockNumber, index uint64) (core.Transaction, error) {
	b.listener.OnRead("TransactionByBlockNumberAndIndex")
	return GetTxByBlockNumIndex(b.database2, blockNumber, index)
}

// TransactionByHash gets the transaction for a given hash.
func (b *Blockchain) TransactionByHash(hash *felt.Felt) (core.Transaction, error) {
	b.listener.OnRead("TransactionByHash")
	return GetTxByHash(b.database2, hash)
}

// Receipt gets the transaction receipt for a given transaction hash.
func (b *Blockchain) Receipt(hash *felt.Felt) (*core.TransactionReceipt, *felt.Felt, uint64, error) {
	b.listener.OnRead("Receipt")
	bnIndex, err := GetTxBlockNumIndexByHash(b.database2, hash)
	if err != nil {
		return nil, nil, 0, err
	}

	receipt, err := GetReceiptByHash(b.database2, hash)
	if err != nil {
		return nil, nil, 0, err
	}

	header, err := GetBlockHeaderByNumber(b.database2, bnIndex.Number)
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
	var l1Head core.L1Head
	data, err := b.database2.Get2(db.L1Height.Key())
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &l1Head)
	if err != nil {
		return nil, err
	}
	return &l1Head, nil
}

func (b *Blockchain) SetL1Head(update *core.L1Head) error {
	data, err := encoder.Marshal(update)
	if err != nil {
		return err
	}
	return b.database2.Put(db.L1Height.Key(), data)
}

// Store takes a block and state update and performs sanity checks before putting in the database.
func (b *Blockchain) Store(block *core.Block, blockCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class,
) error {
	return b.database.Update(func(txn db.Transaction) error {
		if err := verifyBlock(txn, block); err != nil {
			return err
		}

		if err := core.NewState(txn).Update(block.Number, stateUpdate, newClasses); err != nil {
			return err
		}
		if err := StoreBlockHeader(txn, block.Header); err != nil {
			return err
		}

		for i, tx := range block.Transactions {
			if err := storeTransactionAndReceipt(txn, block.Number, uint64(i), tx,
				block.Receipts[i]); err != nil {
				return err
			}
		}

		if err := storeStateUpdate(txn, block.Number, stateUpdate); err != nil {
			return err
		}

		if err := StoreBlockCommitments(txn, block.Number, blockCommitments); err != nil {
			return err
		}

		if err := StoreL1HandlerMsgHashes(txn, block.Transactions); err != nil {
			return err
		}

		// Head of the blockchain is maintained as follows:
		// [db.ChainHeight]() -> (BlockNumber)
		heightBin := core.MarshalBlockNumber(block.Number)
		return txn.Set(db.ChainHeight.Key(), heightBin)
	})
}

// VerifyBlock assumes the block has already been sanity-checked.
func (b *Blockchain) VerifyBlock(block *core.Block) error {
	return verifyBlock2(b.database2, block)
}

func verifyBlock(txn db.Transaction, block *core.Block) error {
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

func verifyBlock2(txn db.KeyValueReader, block *core.Block) error {
	if err := CheckBlockVersion(block.ProtocolVersion); err != nil {
		return err
	}

	expectedBlockNumber := uint64(0)
	expectedParentHash := &felt.Zero

	h, err := headsHeader2(txn)
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

func StoreBlockCommitments(txn db.Transaction, blockNumber uint64, commitments *core.BlockCommitments) error {
	commitmentBytes, err := encoder.Marshal(commitments)
	if err != nil {
		return err
	}

	return txn.Set(db.BlockCommitmentsKey(blockNumber), commitmentBytes)
}

func (b *Blockchain) BlockCommitmentsByNumber(blockNumber uint64) (*core.BlockCommitments, error) {
	b.listener.OnRead("BlockCommitmentsByNumber")
	return GetBlockCommitmentByBlockNum(b.database2, blockNumber)
}

// StoreBlockHeader stores the given block in the database.
// The db storage for blocks is maintained by two buckets as follows:
//
// [db.BlockHeaderNumbersByHash](BlockHash) -> (BlockNumber)
// [db.BlockHeadersByNumber](BlockNumber) -> (BlockHeader)
//
// "[]" is the db prefix to represent a bucket
// "()" are additional keys appended to the prefix or multiple values marshalled together
// "->" represents a key value pair.
func StoreBlockHeader(txn db.Transaction, header *core.Header) error {
	numBytes := core.MarshalBlockNumber(header.Number)

	if err := txn.Set(db.BlockHeaderNumbersByHashKey(header.Hash), numBytes); err != nil {
		return err
	}

	headerBytes, err := encoder.Marshal(header)
	if err != nil {
		return err
	}

	return txn.Set(db.BlockHeaderByNumberKey(header.Number), headerBytes)
}

// blockHeaderByNumber retrieves a block header from database by its number
func blockHeaderByNumber(txn db.Transaction, number uint64) (*core.Header, error) {
	var header *core.Header
	if err := txn.Get(db.BlockHeaderByNumberKey(number), func(val []byte) error {
		header = new(core.Header)
		return encoder.Unmarshal(val, header)
	}); err != nil {
		return nil, err
	}
	return header, nil
}

func blockHeaderByHash(txn db.Transaction, hash *felt.Felt) (*core.Header, error) {
	var header *core.Header
	return header, txn.Get(db.BlockHeaderNumbersByHashKey(hash), func(val []byte) error {
		var err error
		header, err = blockHeaderByNumber(txn, binary.BigEndian.Uint64(val))
		return err
	})
}

// BlockByNumber retrieves a block from database by its number
func BlockByNumber(txn db.Transaction, number uint64) (*core.Block, error) {
	header, err := blockHeaderByNumber(txn, number)
	if err != nil {
		return nil, err
	}

	block := new(core.Block)
	block.Header = header
	block.Transactions, err = TransactionsByBlockNumber(txn, number)
	if err != nil {
		return nil, err
	}

	block.Receipts, err = receiptsByBlockNumber(txn, number)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func TransactionsByBlockNumber(txn db.Transaction, number uint64) ([]core.Transaction, error) {
	numBytes := core.MarshalBlockNumber(number)
	prefix := db.TransactionsByBlockNumberAndIndex.Key(numBytes)

	iterator, err := txn.NewIterator(prefix, true)
	if err != nil {
		return nil, err
	}

	var txs []core.Transaction
	for iterator.First(); iterator.Valid(); iterator.Next() {
		val, vErr := iterator.Value()
		if vErr != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, vErr)
		}

		var tx core.Transaction
		if err = encoder.Unmarshal(val, &tx); err != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, err)
		}

		txs = append(txs, tx)
	}

	if err = iterator.Close(); err != nil {
		return nil, err
	}

	return txs, nil
}

func receiptsByBlockNumber(txn db.Transaction, number uint64) ([]*core.TransactionReceipt, error) {
	numBytes := core.MarshalBlockNumber(number)
	prefix := db.ReceiptsByBlockNumberAndIndex.Key(numBytes)

	iterator, err := txn.NewIterator(prefix, true)
	if err != nil {
		return nil, err
	}

	var receipts []*core.TransactionReceipt

	for iterator.First(); iterator.Valid(); iterator.Next() {
		if !bytes.HasPrefix(iterator.Key(), prefix) {
			break
		}

		val, vErr := iterator.Value()
		if vErr != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, vErr)
		}

		receipt := new(core.TransactionReceipt)
		if err = encoder.Unmarshal(val, receipt); err != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, err)
		}

		receipts = append(receipts, receipt)
	}

	if err = iterator.Close(); err != nil {
		return nil, err
	}

	return receipts, nil
}

func StoreL1HandlerMsgHashes(dbTxn db.Transaction, blockTxns []core.Transaction) error {
	for _, txn := range blockTxns {
		if l1Handler, ok := (txn).(*core.L1HandlerTransaction); ok {
			err := dbTxn.Set(db.L1HandlerTxnHashByMsgHashKey(l1Handler.MessageHash()), txn.Hash().Marshal())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func storeStateUpdate(txn db.Transaction, blockNumber uint64, update *core.StateUpdate) error {
	updateBytes, err := encoder.Marshal(update)
	if err != nil {
		return err
	}

	return txn.Set(db.StateUpdateByBlockNumKey(blockNumber), updateBytes)
}

func stateUpdateByNumber(txn db.Transaction, blockNumber uint64) (*core.StateUpdate, error) {
	var update *core.StateUpdate
	if err := txn.Get(db.StateUpdateByBlockNumKey(blockNumber), func(val []byte) error {
		update = new(core.StateUpdate)
		return encoder.Unmarshal(val, update)
	}); err != nil {
		return nil, err
	}
	return update, nil
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

type txAndReceiptDBKey struct {
	Number uint64
	Index  uint64
}

func (t *txAndReceiptDBKey) MarshalBinary() []byte {
	return binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64([]byte{}, t.Number), t.Index)
}

func (t *txAndReceiptDBKey) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)
	if err := binary.Read(r, binary.BigEndian, &t.Number); err != nil {
		return err
	}
	return binary.Read(r, binary.BigEndian, &t.Index)
}

// storeTransactionAndReceipt stores the given transaction receipt in the database.
// The db storage for transaction and receipts is maintained by three buckets as follows:
//
// [db.TransactionBlockNumbersAndIndicesByHash](TransactionHash) -> (BlockNumber, Index)
// [db.TransactionsByBlockNumberAndIndex](BlockNumber, Index) -> Transaction
// [db.ReceiptsByBlockNumberAndIndex](BlockNumber, Index) -> Receipt
//
// Note: we are using the same transaction hash bucket which keeps track of block number and
// index for both transactions and receipts since transaction and its receipt share the same hash.
// "[]" is the db prefix to represent a bucket
// "()" are additional keys appended to the prefix or multiple values marshalled together
// "->" represents a key value pair.
func storeTransactionAndReceipt(txn db.Transaction, number, i uint64, t core.Transaction, r *core.TransactionReceipt) error {
	bnIndexBytes := (&txAndReceiptDBKey{number, i}).MarshalBinary()

	if err := txn.Set(db.TxBlockNumIndexByHashKey(r.TransactionHash),
		bnIndexBytes); err != nil {
		return err
	}

	txnBytes, err := encoder.Marshal(t)
	if err != nil {
		return err
	}
	if err = txn.Set(db.TransactionsByBlockNumberAndIndex.Key(bnIndexBytes), txnBytes); err != nil {
		return err
	}

	rBytes, err := encoder.Marshal(r)
	if err != nil {
		return err
	}
	return txn.Set(db.ReceiptsByBlockNumberAndIndex.Key(bnIndexBytes), rBytes)
}

// transactionByBlockNumberAndIndex gets the transaction for a given block number and index.
func transactionByBlockNumberAndIndex(txn db.Transaction, bnIndex *txAndReceiptDBKey) (core.Transaction, error) {
	var transaction core.Transaction
	err := txn.Get(db.TransactionsByBlockNumberAndIndex.Key(bnIndex.MarshalBinary()), func(val []byte) error {
		return encoder.Unmarshal(val, &transaction)
	})
	return transaction, err
}

type StateCloser = func() error

// HeadState returns a StateReader that provides a stable view to the latest state
// TODO(weiihann): handle the interface later....
func (b *Blockchain) HeadState() (core.StateReader, StateCloser, error) {
	b.listener.OnRead("HeadState")
	txn, err := b.database.NewTransaction(false)
	if err != nil {
		return nil, nil, err
	}

	_, err = ChainHeight(txn)
	if err != nil {
		return nil, nil, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return core.NewState(txn), txn.Discard, nil
}

// StateAtBlockNumber returns a StateReader that provides a stable view to the state at the given block number
func (b *Blockchain) StateAtBlockNumber(blockNumber uint64) (core.StateReader, StateCloser, error) {
	b.listener.OnRead("StateAtBlockNumber")
	txn, err := b.database.NewTransaction(false)
	if err != nil {
		return nil, nil, err
	}

	_, err = blockHeaderByNumber(txn, blockNumber)
	if err != nil {
		return nil, nil, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return core.NewStateSnapshot(core.NewState(txn), blockNumber), txn.Discard, nil
}

// StateAtBlockHash returns a StateReader that provides a stable view to the state at the given block hash
func (b *Blockchain) StateAtBlockHash(blockHash *felt.Felt) (core.StateReader, StateCloser, error) {
	b.listener.OnRead("StateAtBlockHash")
	if blockHash.IsZero() {
		txn := db.NewMemTransaction()
		emptyState := core.NewState(txn)
		return emptyState, txn.Discard, nil
	}

	txn, err := b.database.NewTransaction(false)
	if err != nil {
		return nil, nil, err
	}

	header, err := blockHeaderByHash(txn, blockHash)
	if err != nil {
		return nil, nil, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return core.NewStateSnapshot(core.NewState(txn), header.Number), txn.Discard, nil
}

// EventFilter returns an EventFilter object that is tied to a snapshot of the blockchain
func (b *Blockchain) EventFilter(from *felt.Felt, keys [][]felt.Felt) (EventFilterer, error) {
	b.listener.OnRead("EventFilter")
	latest, err := GetChainHeight(b.database2)
	if err != nil {
		return nil, err
	}

	return newEventFilter(b.database2, from, keys, 0, latest, b.pendingBlockFn), nil
}

// RevertHead reverts the head block
func (b *Blockchain) RevertHead() error {
	return b.database.Update(b.revertHead)
}

func (b *Blockchain) GetReverseStateDiff() (*core.StateDiff, error) {
	var reverseStateDiff *core.StateDiff
	return reverseStateDiff, b.database.View(func(txn db.Transaction) error {
		blockNumber, err := ChainHeight(txn)
		if err != nil {
			return err
		}
		stateUpdate, err := stateUpdateByNumber(txn, blockNumber)
		if err != nil {
			return err
		}
		state := core.NewState(txn)
		reverseStateDiff, err = state.GetReverseStateDiff(blockNumber, stateUpdate.StateDiff)
		return err
	})
}

func (b *Blockchain) revertHead(txn db.Transaction) error {
	blockNumber, err := ChainHeight(txn)
	if err != nil {
		return err
	}

	stateUpdate, err := stateUpdateByNumber(txn, blockNumber)
	if err != nil {
		return err
	}

	state := core.NewState(txn)
	// revert state
	if err = state.Revert(blockNumber, stateUpdate); err != nil {
		return err
	}

	header, err := blockHeaderByNumber(txn, blockNumber)
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

	if err = removeTxsAndReceipts(txn, blockNumber, header.TransactionCount); err != nil {
		return err
	}

	// remove state update
	if err = txn.Delete(db.StateUpdateByBlockNumKey(blockNumber)); err != nil {
		return err
	}

	// Revert chain height.
	if genesisBlock {
		return txn.Delete(db.ChainHeight.Key())
	}

	heightBin := core.MarshalBlockNumber(blockNumber - 1)
	return txn.Set(db.ChainHeight.Key(), heightBin)
}

func removeTxsAndReceipts(txn db.Transaction, blockNumber, numTxs uint64) error {
	blockIDAndIndex := txAndReceiptDBKey{
		Number: blockNumber,
	}
	// remove txs and receipts
	for i := range numTxs {
		blockIDAndIndex.Index = i
		reorgedTxn, err := transactionByBlockNumberAndIndex(txn, &blockIDAndIndex)
		if err != nil {
			return err
		}

		keySuffix := blockIDAndIndex.MarshalBinary()
		if err = txn.Delete(db.TxByBlockNumIndexKeyBytes(keySuffix)); err != nil {
			return err
		}
		if err = txn.Delete(db.ReceiptByBlockNumIndexKeyBytes(keySuffix)); err != nil {
			return err
		}
		if err = txn.Delete(db.TxBlockNumIndexByHashKey(reorgedTxn.Hash())); err != nil {
			return err
		}
		if l1handler, ok := reorgedTxn.(*core.L1HandlerTransaction); ok {
			if err = txn.Delete(db.L1HandlerTxnHashByMsgHashKey(l1handler.MessageHash())); err != nil {
				return err
			}
		}
	}

	return nil
}
