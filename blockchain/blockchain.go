package blockchain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

//go:generate mockgen -destination=../mocks/mock_blockchain.go -package=mocks github.com/NethermindEth/juno/blockchain Reader
type Reader interface {
	Height() (height uint64, err error)

	Head() (head *core.Block, err error)
	L1Head() (*core.L1Head, error)
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
	PendingState() (core.StateReader, StateCloser, error)

	BlockCommitmentsByNumber(blockNumber uint64) (*core.BlockCommitments, error)

	EventFilter(from *felt.Felt, keys [][]felt.Felt) (EventFilterer, error)

	Pending() (Pending, error)

	Network() *utils.Network
}

var (
	ErrParentDoesNotMatchHead = errors.New("block's parent hash does not match head block hash")
	SupportedStarknetVersion  = semver.MustParse("0.13.3")
)

func checkBlockVersion(protocolVersion string) error {
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
	network  *utils.Network
	database db.DB

	listener EventListener

	cachedPending atomic.Pointer[Pending]
}

func New(database db.DB, network *utils.Network) *Blockchain {
	RegisterCoreTypesToEncoder()
	return &Blockchain{
		database: database,
		network:  network,
		listener: &SelectiveListener{},
	}
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
	var commitment *felt.Felt
	return commitment, b.database.View(func(txn db.Transaction) error {
		var err error
		commitment, err = core.NewState(txn).Root()
		return err
	})
}

// Height returns the latest block height. If blockchain is empty nil is returned.
func (b *Blockchain) Height() (uint64, error) {
	b.listener.OnRead("Height")
	var height uint64
	return height, b.database.View(func(txn db.Transaction) error {
		var err error
		height, err = ChainHeight(txn)
		return err
	})
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
	var h *core.Block
	return h, b.database.View(func(txn db.Transaction) error {
		var err error
		h, err = head(txn)
		return err
	})
}

func (b *Blockchain) HeadsHeader() (*core.Header, error) {
	b.listener.OnRead("HeadsHeader")
	var header *core.Header

	return header, b.database.View(func(txn db.Transaction) error {
		var err error
		header, err = headsHeader(txn)
		return err
	})
}

func head(txn db.Transaction) (*core.Block, error) {
	height, err := ChainHeight(txn)
	if err != nil {
		return nil, err
	}
	return BlockByNumber(txn, height)
}

func headsHeader(txn db.Transaction) (*core.Header, error) {
	height, err := ChainHeight(txn)
	if err != nil {
		return nil, err
	}

	return blockHeaderByNumber(txn, height)
}

func (b *Blockchain) BlockByNumber(number uint64) (*core.Block, error) {
	b.listener.OnRead("BlockByNumber")
	var block *core.Block
	return block, b.database.View(func(txn db.Transaction) error {
		var err error
		block, err = BlockByNumber(txn, number)
		return err
	})
}

func (b *Blockchain) BlockHeaderByNumber(number uint64) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByNumber")
	var header *core.Header
	return header, b.database.View(func(txn db.Transaction) error {
		var err error
		header, err = blockHeaderByNumber(txn, number)
		return err
	})
}

func (b *Blockchain) BlockByHash(hash *felt.Felt) (*core.Block, error) {
	b.listener.OnRead("BlockByHash")
	var block *core.Block
	return block, b.database.View(func(txn db.Transaction) error {
		var err error
		block, err = blockByHash(txn, hash)
		return err
	})
}

func (b *Blockchain) BlockHeaderByHash(hash *felt.Felt) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByHash")
	var header *core.Header
	return header, b.database.View(func(txn db.Transaction) error {
		var err error
		header, err = blockHeaderByHash(txn, hash)
		return err
	})
}

func (b *Blockchain) StateUpdateByNumber(number uint64) (*core.StateUpdate, error) {
	b.listener.OnRead("StateUpdateByNumber")
	var update *core.StateUpdate
	return update, b.database.View(func(txn db.Transaction) error {
		var err error
		update, err = stateUpdateByNumber(txn, number)
		return err
	})
}

func (b *Blockchain) StateUpdateByHash(hash *felt.Felt) (*core.StateUpdate, error) {
	b.listener.OnRead("StateUpdateByHash")
	var update *core.StateUpdate
	return update, b.database.View(func(txn db.Transaction) error {
		var err error
		update, err = stateUpdateByHash(txn, hash)
		return err
	})
}

func (b *Blockchain) L1HandlerTxnHash(msgHash *common.Hash) (*felt.Felt, error) {
	b.listener.OnRead("L1HandlerTxnHash")
	var l1HandlerTxnHash *felt.Felt
	return l1HandlerTxnHash, b.database.View(func(txn db.Transaction) error {
		var err error
		l1HandlerTxnHash, err = l1HandlerTxnHashByMsgHash(txn, msgHash)
		return err
	})
}

// TransactionByBlockNumberAndIndex gets the transaction for a given block number and index.
func (b *Blockchain) TransactionByBlockNumberAndIndex(blockNumber, index uint64) (core.Transaction, error) {
	b.listener.OnRead("TransactionByBlockNumberAndIndex")
	var transaction core.Transaction
	return transaction, b.database.View(func(txn db.Transaction) error {
		var err error
		transaction, err = transactionByBlockNumberAndIndex(txn, &txAndReceiptDBKey{blockNumber, index})
		return err
	})
}

// TransactionByHash gets the transaction for a given hash.
func (b *Blockchain) TransactionByHash(hash *felt.Felt) (core.Transaction, error) {
	b.listener.OnRead("TransactionByHash")
	var transaction core.Transaction
	return transaction, b.database.View(func(txn db.Transaction) error {
		var err error
		transaction, err = transactionByHash(txn, hash)

		// not found in the canonical blocks, try pending
		if errors.Is(err, db.ErrKeyNotFound) {
			var pending Pending
			pending, err = b.pendingBlock(txn)
			if err != nil {
				return err
			}

			for _, t := range pending.Block.Transactions {
				if hash.Equal(t.Hash()) {
					transaction = t
					return nil
				}
			}
			return db.ErrKeyNotFound
		}

		return err
	})
}

// Receipt gets the transaction receipt for a given transaction hash.
func (b *Blockchain) Receipt(hash *felt.Felt) (*core.TransactionReceipt, *felt.Felt, uint64, error) {
	b.listener.OnRead("Receipt")
	var (
		receipt     *core.TransactionReceipt
		blockHash   *felt.Felt
		blockNumber uint64
	)
	return receipt, blockHash, blockNumber, b.database.View(func(txn db.Transaction) error {
		var err error
		receipt, blockHash, blockNumber, err = receiptByHash(txn, hash)

		// not found in the canonical blocks, try pending
		if errors.Is(err, db.ErrKeyNotFound) {
			var pending Pending
			pending, err = b.pendingBlock(txn)
			if err != nil {
				return err
			}

			for i, t := range pending.Block.Transactions {
				if hash.Equal(t.Hash()) {
					receipt = pending.Block.Receipts[i]
					blockHash = nil
					blockNumber = 0
					return nil
				}
			}
			return db.ErrKeyNotFound
		}

		return err
	})
}

func (b *Blockchain) L1Head() (*core.L1Head, error) {
	b.listener.OnRead("L1Head")
	var update *core.L1Head

	return update, b.database.View(func(txn db.Transaction) error {
		var err error
		update, err = l1Head(txn)
		return err
	})
}

func l1Head(txn db.Transaction) (*core.L1Head, error) {
	var update *core.L1Head
	if err := txn.Get(db.L1Height.Key(), func(updateBytes []byte) error {
		return encoder.Unmarshal(updateBytes, &update)
	}); err != nil {
		return nil, err
	}
	return update, nil
}

func (b *Blockchain) SetL1Head(update *core.L1Head) error {
	updateBytes, err := encoder.Marshal(update)
	if err != nil {
		return err
	}
	return b.database.Update(func(txn db.Transaction) error {
		return txn.Set(db.L1Height.Key(), updateBytes)
	})
}

// Store takes a block and state update and performs sanity checks before putting in the database.
func (b *Blockchain) Store(block *core.Block, blockCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class,
) error {
	return b.database.Update(func(txn db.Transaction) error {
		if err := verifyBlock(txn, block); err != nil {
			return err
		}

		state := core.NewState(txn)
		if err := state.Update(block.Number, stateUpdate.StateDiff, newClasses); err != nil {
			return err
		}
		stateRoot, err := state.Root()
		if err != nil {
			return err
		}

		if !stateRoot.Equal(stateUpdate.NewRoot) {
			return fmt.Errorf("new state root does not match expected root")
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

		if err := b.storeEmptyPending(txn, block.Header); err != nil {
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
	return b.database.View(func(txn db.Transaction) error {
		return verifyBlock(txn, block)
	})
}

func verifyBlock(txn db.Transaction, block *core.Block) error {
	if err := checkBlockVersion(block.ProtocolVersion); err != nil {
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

func StoreBlockCommitments(txn db.Transaction, blockNumber uint64, commitments *core.BlockCommitments) error {
	numBytes := core.MarshalBlockNumber(blockNumber)

	commitmentBytes, err := encoder.Marshal(commitments)
	if err != nil {
		return err
	}

	return txn.Set(db.BlockCommitments.Key(numBytes), commitmentBytes)
}

func (b *Blockchain) BlockCommitmentsByNumber(blockNumber uint64) (*core.BlockCommitments, error) {
	b.listener.OnRead("BlockCommitmentsByNumber")
	var commitments *core.BlockCommitments
	return commitments, b.database.View(func(txn db.Transaction) error {
		var err error
		commitments, err = blockCommitmentsByNumber(txn, blockNumber)
		return err
	})
}

func blockCommitmentsByNumber(txn db.Transaction, blockNumber uint64) (*core.BlockCommitments, error) {
	numBytes := core.MarshalBlockNumber(blockNumber)

	var commitments *core.BlockCommitments
	if err := txn.Get(db.BlockCommitments.Key(numBytes), func(val []byte) error {
		commitments = new(core.BlockCommitments)
		return encoder.Unmarshal(val, commitments)
	}); err != nil {
		return nil, err
	}
	return commitments, nil
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

	if err := txn.Set(db.BlockHeaderNumbersByHash.Key(header.Hash.Marshal()), numBytes); err != nil {
		return err
	}

	headerBytes, err := encoder.Marshal(header)
	if err != nil {
		return err
	}

	return txn.Set(db.BlockHeadersByNumber.Key(numBytes), headerBytes)
}

// blockHeaderByNumber retrieves a block header from database by its number
func blockHeaderByNumber(txn db.Transaction, number uint64) (*core.Header, error) {
	numBytes := core.MarshalBlockNumber(number)

	var header *core.Header
	if err := txn.Get(db.BlockHeadersByNumber.Key(numBytes), func(val []byte) error {
		header = new(core.Header)
		return encoder.Unmarshal(val, header)
	}); err != nil {
		return nil, err
	}
	return header, nil
}

func blockHeaderByHash(txn db.Transaction, hash *felt.Felt) (*core.Header, error) {
	var header *core.Header
	return header, txn.Get(db.BlockHeaderNumbersByHash.Key(hash.Marshal()), func(val []byte) error {
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
	iterator, err := txn.NewIterator()
	if err != nil {
		return nil, err
	}

	txs := []core.Transaction{}
	numBytes := core.MarshalBlockNumber(number)

	prefix := db.TransactionsByBlockNumberAndIndex.Key(numBytes)
	for iterator.Seek(prefix); iterator.Valid(); iterator.Next() {
		if !bytes.HasPrefix(iterator.Key(), prefix) {
			break
		}

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
	iterator, err := txn.NewIterator()
	if err != nil {
		return nil, err
	}

	receipts := []*core.TransactionReceipt{}
	numBytes := core.MarshalBlockNumber(number)

	prefix := db.ReceiptsByBlockNumberAndIndex.Key(numBytes)
	for iterator.Seek(prefix); iterator.Valid(); iterator.Next() {
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

// blockByHash retrieves a block from database by its hash
func blockByHash(txn db.Transaction, hash *felt.Felt) (*core.Block, error) {
	var block *core.Block
	return block, txn.Get(db.BlockHeaderNumbersByHash.Key(hash.Marshal()), func(val []byte) error {
		var err error
		block, err = BlockByNumber(txn, binary.BigEndian.Uint64(val))
		return err
	})
}

func StoreL1HandlerMsgHashes(dbTxn db.Transaction, blockTxns []core.Transaction) error {
	for _, txn := range blockTxns {
		if l1Handler, ok := (txn).(*core.L1HandlerTransaction); ok {
			err := dbTxn.Set(db.L1HandlerTxnHashByMsgHash.Key(l1Handler.MessageHash()), txn.Hash().Marshal())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func storeStateUpdate(txn db.Transaction, blockNumber uint64, update *core.StateUpdate) error {
	numBytes := core.MarshalBlockNumber(blockNumber)

	updateBytes, err := encoder.Marshal(update)
	if err != nil {
		return err
	}

	return txn.Set(db.StateUpdatesByBlockNumber.Key(numBytes), updateBytes)
}

func stateUpdateByNumber(txn db.Transaction, blockNumber uint64) (*core.StateUpdate, error) {
	numBytes := core.MarshalBlockNumber(blockNumber)

	var update *core.StateUpdate
	if err := txn.Get(db.StateUpdatesByBlockNumber.Key(numBytes), func(val []byte) error {
		update = new(core.StateUpdate)
		return encoder.Unmarshal(val, update)
	}); err != nil {
		return nil, err
	}
	return update, nil
}

func stateUpdateByHash(txn db.Transaction, hash *felt.Felt) (*core.StateUpdate, error) {
	var update *core.StateUpdate
	return update, txn.Get(db.BlockHeaderNumbersByHash.Key(hash.Marshal()), func(val []byte) error {
		var err error
		update, err = stateUpdateByNumber(txn, binary.BigEndian.Uint64(val))
		return err
	})
}

func l1HandlerTxnHashByMsgHash(txn db.Transaction, l1HandlerMsgHash *common.Hash) (*felt.Felt, error) {
	l1HandlerTxnHash := new(felt.Felt)
	return l1HandlerTxnHash, txn.Get(db.L1HandlerTxnHashByMsgHash.Key(l1HandlerMsgHash.Bytes()), func(val []byte) error {
		l1HandlerTxnHash.Unmarshal(val)
		return nil
	})
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

	if err := txn.Set(db.TransactionBlockNumbersAndIndicesByHash.Key((r.TransactionHash).Marshal()),
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

// transactionBlockNumberAndIndexByHash gets the block number and index for a given transaction hash
func transactionBlockNumberAndIndexByHash(txn db.Transaction, hash *felt.Felt) (*txAndReceiptDBKey, error) {
	var bnIndex *txAndReceiptDBKey
	if err := txn.Get(db.TransactionBlockNumbersAndIndicesByHash.Key(hash.Marshal()), func(val []byte) error {
		bnIndex = new(txAndReceiptDBKey)
		return bnIndex.UnmarshalBinary(val)
	}); err != nil {
		return nil, err
	}
	return bnIndex, nil
}

// transactionByBlockNumberAndIndex gets the transaction for a given block number and index.
func transactionByBlockNumberAndIndex(txn db.Transaction, bnIndex *txAndReceiptDBKey) (core.Transaction, error) {
	var transaction core.Transaction
	err := txn.Get(db.TransactionsByBlockNumberAndIndex.Key(bnIndex.MarshalBinary()), func(val []byte) error {
		return encoder.Unmarshal(val, &transaction)
	})
	return transaction, err
}

// transactionByHash gets the transaction for a given hash.
func transactionByHash(txn db.Transaction, hash *felt.Felt) (core.Transaction, error) {
	bnIndex, err := transactionBlockNumberAndIndexByHash(txn, hash)
	if err != nil {
		return nil, err
	}
	return transactionByBlockNumberAndIndex(txn, bnIndex)
}

// receiptByHash gets the transaction receipt for a given hash.
func receiptByHash(txn db.Transaction, hash *felt.Felt) (*core.TransactionReceipt, *felt.Felt, uint64, error) {
	bnIndex, err := transactionBlockNumberAndIndexByHash(txn, hash)
	if err != nil {
		return nil, nil, 0, err
	}

	receipt, err := receiptByBlockNumberAndIndex(txn, bnIndex)
	if err != nil {
		return nil, nil, 0, err
	}

	header, err := blockHeaderByNumber(txn, bnIndex.Number)
	if err != nil {
		return nil, nil, 0, err
	}

	return receipt, header.Hash, header.Number, nil
}

// receiptByBlockNumberAndIndex gets the transaction receipt for a given block number and index.
func receiptByBlockNumberAndIndex(txn db.Transaction, bnIndex *txAndReceiptDBKey) (*core.TransactionReceipt, error) {
	var r *core.TransactionReceipt
	err := txn.Get(db.ReceiptsByBlockNumberAndIndex.Key(bnIndex.MarshalBinary()), func(val []byte) error {
		return encoder.Unmarshal(val, &r)
	})
	return r, err
}

type StateCloser = func() error

// HeadState returns a StateReader that provides a stable view to the latest state
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
	txn, err := b.database.NewTransaction(false)
	if err != nil {
		return nil, err
	}

	latest, err := ChainHeight(txn)
	if err != nil {
		return nil, err
	}

	return newEventFilter(txn, from, keys, 0, latest), nil
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
	numBytes := core.MarshalBlockNumber(blockNumber)

	stateUpdate, err := stateUpdateByNumber(txn, blockNumber)
	if err != nil {
		return err
	}

	state := core.NewState(txn)
	// revert state
	if err = state.Revert(blockNumber, stateUpdate); err != nil {
		return err
	}

	if err = b.verifyStateUpdateRoot(state, stateUpdate.OldRoot); err != nil {
		return err
	}

	header, err := blockHeaderByNumber(txn, blockNumber)
	if err != nil {
		return err
	}

	genesisBlock := blockNumber == 0

	// remove block header
	for _, key := range [][]byte{
		db.BlockHeadersByNumber.Key(numBytes),
		db.BlockHeaderNumbersByHash.Key(header.Hash.Marshal()),
		db.BlockCommitments.Key(numBytes),
	} {
		if err = txn.Delete(key); err != nil {
			return err
		}
	}

	if err = removeTxsAndReceipts(txn, blockNumber, header.TransactionCount); err != nil {
		return err
	}

	// remove state update
	if err = txn.Delete(db.StateUpdatesByBlockNumber.Key(numBytes)); err != nil {
		return err
	}

	// Revert chain height and pending.
	if genesisBlock {
		if err = txn.Delete(db.Pending.Key()); err != nil {
			return err
		}
		return txn.Delete(db.ChainHeight.Key())
	}

	var newHeader *core.Header
	newHeader, err = blockHeaderByNumber(txn, blockNumber-1)
	if err != nil {
		return err
	}
	if err := b.storeEmptyPending(txn, newHeader); err != nil {
		return err
	}

	heightBin := core.MarshalBlockNumber(blockNumber - 1)
	return txn.Set(db.ChainHeight.Key(), heightBin)
}

func removeTxsAndReceipts(txn db.Transaction, blockNumber, numTxs uint64) error {
	blockIDAndIndex := txAndReceiptDBKey{
		Number: blockNumber,
	}
	// remove txs and receipts
	for i := uint64(0); i < numTxs; i++ {
		blockIDAndIndex.Index = i
		reorgedTxn, err := transactionByBlockNumberAndIndex(txn, &blockIDAndIndex)
		if err != nil {
			return err
		}

		keySuffix := blockIDAndIndex.MarshalBinary()
		if err = txn.Delete(db.TransactionsByBlockNumberAndIndex.Key(keySuffix)); err != nil {
			return err
		}
		if err = txn.Delete(db.ReceiptsByBlockNumberAndIndex.Key(keySuffix)); err != nil {
			return err
		}
		if err = txn.Delete(db.TransactionBlockNumbersAndIndicesByHash.Key(reorgedTxn.Hash().Marshal())); err != nil {
			return err
		}
		if l1handler, ok := reorgedTxn.(*core.L1HandlerTransaction); ok {
			if err = txn.Delete(db.L1HandlerTxnHashByMsgHash.Key(l1handler.MessageHash())); err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *Blockchain) CleanPendingState() error {
	header, err := b.HeadsHeader()
	if err != nil {
		return err
	}
	txn, err := b.database.NewTransaction(true)
	if err != nil {
		return err
	}
	err = b.storeEmptyPending(txn, header)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (b *Blockchain) storeEmptyPending(txn db.Transaction, latestHeader *core.Header) error {
	receipts := make([]*core.TransactionReceipt, 0)
	pendingBlock := &core.Block{
		Header: &core.Header{
			ParentHash:       latestHeader.Hash,
			SequencerAddress: latestHeader.SequencerAddress,
			Number:           latestHeader.Number + 1,
			Timestamp:        uint64(time.Now().Unix()),
			ProtocolVersion:  latestHeader.ProtocolVersion,
			EventsBloom:      core.EventsBloom(receipts),
			GasPrice:         latestHeader.GasPrice,
			GasPriceSTRK:     latestHeader.GasPriceSTRK,
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts,
	}

	stateDiff, err := MakeStateDiffForEmptyBlock(b, latestHeader.Number+1)
	if err != nil {
		return err
	}

	emptyPending := &Pending{
		Block: pendingBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   latestHeader.GlobalStateRoot,
			StateDiff: stateDiff,
		},
		NewClasses: make(map[felt.Felt]core.Class, 0),
	}
	return b.storePending(txn, emptyPending)
}

// StorePending stores a pending block given that it is for the next height
func (b *Blockchain) StorePending(pending *Pending) error {
	return b.database.Update(func(txn db.Transaction) error {
		expectedParentHash := new(felt.Felt)
		h, err := headsHeader(txn)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		} else if err == nil {
			expectedParentHash = h.Hash
		}

		if !expectedParentHash.Equal(pending.Block.ParentHash) {
			return ErrParentDoesNotMatchHead
		}

		if existingPending, err := b.pendingBlock(txn); err == nil {
			if existingPending.Block.TransactionCount >= pending.Block.TransactionCount {
				return nil // ignore the incoming pending if it has fewer transactions than the one we already have
			}
			pending.Block.Number = existingPending.Block.Number // Just in case the number is not set.
		} else if !errors.Is(err, db.ErrKeyNotFound) { // Allow StorePending before block zero.
			return err
		}

		return b.storePending(txn, pending)
	})
}

func (b *Blockchain) storePending(txn db.Transaction, pending *Pending) error {
	if err := storePending(txn, pending); err != nil {
		return err
	}
	b.cachedPending.Store(pending)
	return nil
}

func storePending(txn db.Transaction, pending *Pending) error {
	pendingBytes, err := encoder.Marshal(pending)
	if err != nil {
		return err
	}
	return txn.Set(db.Pending.Key(), pendingBytes)
}

func (b *Blockchain) pendingBlock(txn db.Transaction) (Pending, error) {
	if cachedPending := b.cachedPending.Load(); cachedPending != nil {
		expectedParentHash := &felt.Zero
		if head, err := headsHeader(txn); err == nil {
			expectedParentHash = head.Hash
		}
		if cachedPending.Block.ParentHash.Equal(expectedParentHash) {
			return *cachedPending, nil
		}
	}

	// Either cachedPending was nil or wasn't consistent with the HEAD we have
	// in the database, so read it directly from the database
	return pendingBlock(txn)
}

func pendingBlock(txn db.Transaction) (Pending, error) {
	var pending Pending
	err := txn.Get(db.Pending.Key(), func(bytes []byte) error {
		return encoder.Unmarshal(bytes, &pending)
	})
	return pending, err
}

// Pending returns the pending block from the database
func (b *Blockchain) Pending() (Pending, error) {
	b.listener.OnRead("Pending")
	var pending Pending
	return pending, b.database.View(func(txn db.Transaction) error {
		var err error
		pending, err = b.pendingBlock(txn)
		return err
	})
}

// PendingState returns the state resulting from execution of the pending block
func (b *Blockchain) PendingState() (core.StateReader, StateCloser, error) {
	b.listener.OnRead("PendingState")
	txn, err := b.database.NewTransaction(false)
	if err != nil {
		return nil, nil, err
	}

	pending, err := b.pendingBlock(txn)
	if err != nil {
		return nil, nil, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return NewPendingState(
		pending.StateUpdate.StateDiff,
		pending.NewClasses,
		core.NewState(txn),
	), txn.Discard, nil
}

func MakeStateDiffForEmptyBlock(bc Reader, blockNumber uint64) (*core.StateDiff, error) {
	stateDiff := core.EmptyStateDiff()

	const blockHashLag = 10
	if blockNumber < blockHashLag {
		return stateDiff, nil
	}

	header, err := bc.BlockHeaderByNumber(blockNumber - blockHashLag)
	if err != nil {
		return nil, err
	}

	blockHashStorageContract := new(felt.Felt).SetUint64(1)
	stateDiff.StorageDiffs[*blockHashStorageContract] = map[felt.Felt]*felt.Felt{
		*new(felt.Felt).SetUint64(header.Number): header.Hash,
	}
	return stateDiff, nil
}

func (b *Blockchain) verifyStateUpdateRoot(s *core.State, root *felt.Felt) error {
	currentRoot, err := s.Root()
	if err != nil {
		return err
	}

	if !root.Equal(currentRoot) {
		return fmt.Errorf("state's current root: %s does not match the expected root: %s", currentRoot, root)
	}
	return nil
}

type BlockSignFunc func(blockHash, stateDiffCommitment *felt.Felt) ([]*felt.Felt, error)

// Finalise will calculate the state commitment and block hash for the given pending block and append it to the
// blockchain. In cases where the sequencer needs to re-generate another chain (eg Sepolia), the optional reference
// block and state update should be provided.
func (b *Blockchain) Finalise(pending *Pending, sign BlockSignFunc, refStateUpdate *core.StateUpdate, refBlock *core.Block) error {
	return b.database.Update(func(txn db.Transaction) error {
		var err error
		state := core.NewState(txn)
		oldStateRoot, err := state.Root()
		if err != nil {
			return err
		}
		pending.StateUpdate.OldRoot = oldStateRoot

		if err = state.Update(pending.Block.Number, pending.StateUpdate.StateDiff, pending.NewClasses); err != nil {
			return err
		}
		newStateRoot, err := state.Root()
		if err != nil {
			return err
		}
		pending.Block.GlobalStateRoot = newStateRoot
		pending.StateUpdate.NewRoot = pending.Block.GlobalStateRoot

		var commitments *core.BlockCommitments
		pending.Block.Hash, commitments, err = core.BlockHash(
			pending.Block,
			pending.StateUpdate.StateDiff,
			b.network,
			pending.Block.SequencerAddress)
		if err != nil {
			return err
		}
		pending.StateUpdate.BlockHash = pending.Block.Hash

		if refStateUpdate != nil && refBlock != nil {
			err := b.verifyAgainstReference(pending, commitments, refStateUpdate, refBlock)
			if err != nil {
				return err
			}
		}

		if sign != nil {
			sig, err := sign(pending.Block.Hash, pending.StateUpdate.StateDiff.Commitment())
			if err != nil {
				return err
			}
			pending.Block.Signatures = [][]*felt.Felt{sig}
		}

		if !newStateRoot.Equal(pending.Block.GlobalStateRoot) {
			return fmt.Errorf("new pe root does not match expected root")
		}

		if err := StoreBlockHeader(txn, pending.Block.Header); err != nil {
			return err
		}

		for i, tx := range pending.Block.Transactions {
			if err := storeTransactionAndReceipt(txn, pending.Block.Number, uint64(i), tx,
				pending.Block.Receipts[i]); err != nil {
				return err
			}
		}

		if err := storeStateUpdate(txn, pending.Block.Number, pending.StateUpdate); err != nil {
			return err
		}

		if err := StoreBlockCommitments(txn, pending.Block.Number, commitments); err != nil {
			return err
		}

		if err := StoreL1HandlerMsgHashes(txn, pending.Block.Transactions); err != nil {
			return err
		}

		if err := b.storeEmptyPending(txn, pending.Block.Header); err != nil {
			return err
		}

		// Head of the blockchain is maintained as follows:
		// [db.ChainHeight]() -> (BlockNumber)
		heightBin := core.MarshalBlockNumber(pending.Block.Number)
		return txn.Set(db.ChainHeight.Key(), heightBin)
	})
}

func (b *Blockchain) verifyAgainstReference(pending *Pending, commitments *core.BlockCommitments,
	refStateUpdate *core.StateUpdate, refBlock *core.Block,
) error {
	if err := b.validateStateDiff(refStateUpdate, pending.StateUpdate); err != nil {
		return err
	}
	if err := b.validateCommitments(refBlock, refStateUpdate, commitments); err != nil {
		return err
	}
	if err := b.validateHeader(refBlock.Header, pending.Block.Header); err != nil {
		return err
	}
	return nil
}

func (b *Blockchain) validateStateDiff(shadowStateUpdate, pendingStateUpdate *core.StateUpdate) error {
	_, diffFound := pendingStateUpdate.StateDiff.Diff(shadowStateUpdate.StateDiff, "sequencer", "sepolia")
	if diffFound {
		// Todo: make format nicely
		// fmt.Println(diffString)
		return fmt.Errorf("state diff validation failed")
	}
	return nil
}

func (b *Blockchain) validateCommitments(shadowBlock *core.Block, shadowStateUpdate *core.StateUpdate,
	sequenceCommitments *core.BlockCommitments,
) error {
	_, shadowCommitments, err := core.BlockHash(shadowBlock, shadowStateUpdate.StateDiff, b.network, nil)
	if err != nil {
		return fmt.Errorf("failed to compute the shadow commitments %s", err)
	}
	blockVer, err := core.ParseBlockVersion(shadowBlock.ProtocolVersion)
	if err != nil {
		return err
	}
	if blockVer.LessThan(semver.MustParse("0.13.2")) {
		// receipt commitment isn't needed for pre 0.13.2 blocks. see post07Hash().
		shadowCommitments.ReceiptCommitment = nil
	}

	if !shadowCommitments.TransactionCommitment.Equal(sequenceCommitments.TransactionCommitment) {
		return fmt.Errorf("transaction commitment mismatch: shadow commitment %v, sequence commitment %v",
			shadowCommitments.TransactionCommitment, sequenceCommitments.TransactionCommitment)
	}
	if !shadowCommitments.EventCommitment.Equal(sequenceCommitments.EventCommitment) {
		return fmt.Errorf("event commitment mismatch: shadow commitment %v, sequence commitment %v",
			shadowCommitments.EventCommitment, sequenceCommitments.EventCommitment)
	}
	if shadowCommitments.ReceiptCommitment != nil && !shadowCommitments.ReceiptCommitment.Equal(sequenceCommitments.ReceiptCommitment) {
		return fmt.Errorf("receipt commitment mismatch: shadow commitment %v, sequence commitment %v",
			shadowCommitments.ReceiptCommitment, sequenceCommitments.ReceiptCommitment)
	}
	if shadowCommitments.StateDiffCommitment != nil && !shadowCommitments.StateDiffCommitment.Equal(sequenceCommitments.StateDiffCommitment) {
		return fmt.Errorf("state diff commitment mismatch: shadow commitment %v, sequence commitment %v",
			shadowCommitments.StateDiffCommitment, sequenceCommitments.StateDiffCommitment)
	}
	return nil
}

func (b *Blockchain) validateHeader(shadowHeader, sequenceHeader *core.Header) error {
	if !shadowHeader.ParentHash.Equal(sequenceHeader.ParentHash) {
		return fmt.Errorf("parent hash mismatch: shadowHeader parent hash %v, sequenceHeader parent hash %v",
			shadowHeader.ParentHash, sequenceHeader.ParentHash)
	}
	if shadowHeader.Number != sequenceHeader.Number {
		return fmt.Errorf("block number mismatch: shadowHeader number %v, sequenceHeader number %v",
			shadowHeader.Number, sequenceHeader.Number)
	}
	if !shadowHeader.SequencerAddress.Equal(sequenceHeader.SequencerAddress) {
		return fmt.Errorf("sequencer address mismatch: shadowHeader sequencer address %v, sequenceHeader sequencer address %v",
			shadowHeader.SequencerAddress, sequenceHeader.SequencerAddress)
	}
	if shadowHeader.TransactionCount != sequenceHeader.TransactionCount {
		return fmt.Errorf("transaction count mismatch: shadowHeader transaction count %v, sequenceHeader transaction count %v",
			shadowHeader.TransactionCount, sequenceHeader.TransactionCount)
	}
	if shadowHeader.EventCount != sequenceHeader.EventCount {
		return fmt.Errorf("event count mismatch: shadowHeader event count %v, sequenceHeader event count %v",
			shadowHeader.EventCount, sequenceHeader.EventCount)
	}
	if shadowHeader.Timestamp != sequenceHeader.Timestamp {
		return fmt.Errorf("timestamp mismatch: shadowHeader timestamp %v, sequenceHeader timestamp %v",
			shadowHeader.Timestamp, sequenceHeader.Timestamp)
	}
	if shadowHeader.ProtocolVersion != sequenceHeader.ProtocolVersion {
		return fmt.Errorf("protocol version mismatch: shadowHeader protocol version %v, sequenceHeader protocol version %v",
			shadowHeader.ProtocolVersion, sequenceHeader.ProtocolVersion)
	}
	if !shadowHeader.GasPrice.Equal(sequenceHeader.GasPrice) {
		return fmt.Errorf("gas price mismatch: shadowHeader gas price %v, sequenceHeader gas price %v",
			shadowHeader.GasPrice, sequenceHeader.GasPrice)
	}
	if !shadowHeader.GasPriceSTRK.Equal(sequenceHeader.GasPriceSTRK) {
		return fmt.Errorf("gas price STRK mismatch: shadowHeader gas price STRK %v, sequenceHeader gas price STRK %v",
			shadowHeader.GasPriceSTRK, sequenceHeader.GasPriceSTRK)
	}
	if shadowHeader.L1DAMode != sequenceHeader.L1DAMode {
		return fmt.Errorf("L1 data availability mode mismatch: shadowHeader L1DAMode %v, sequenceHeader L1DAMode %v",
			shadowHeader.L1DAMode, sequenceHeader.L1DAMode)
	}
	if !shadowHeader.L1DataGasPrice.PriceInFri.Equal(sequenceHeader.L1DataGasPrice.PriceInFri) {
		return fmt.Errorf("L1 data gas PriceInFri mismatch: shadowHeader L1DataGasPrice %v, sequenceHeader L1DataGasPrice %v",
			shadowHeader.L1DataGasPrice, sequenceHeader.L1DataGasPrice)
	}
	if !shadowHeader.L1DataGasPrice.PriceInWei.Equal(sequenceHeader.L1DataGasPrice.PriceInWei) {
		return fmt.Errorf("L1 data gas PriceInFri mismatch: shadowHeader L1DataGasPrice %v, sequenceHeader L1DataGasPrice %v",
			shadowHeader.L1DataGasPrice, sequenceHeader.L1DataGasPrice)
	}
	if !shadowHeader.GlobalStateRoot.Equal(sequenceHeader.GlobalStateRoot) {
		return fmt.Errorf("global state root mismatch: shadowHeader global state root %v, sequenceHeader global state root %v",
			shadowHeader.GlobalStateRoot, sequenceHeader.GlobalStateRoot)
	}
	if !shadowHeader.Hash.Equal(sequenceHeader.Hash) {
		return fmt.Errorf("hash mismatch: shadowHeader hash %v, sequenceHeader hash %v", shadowHeader.Hash, sequenceHeader.Hash)
	}
	return nil
}

func (b *Blockchain) StoreGenesis(diff *core.StateDiff, classes map[felt.Felt]core.Class) error {
	receipts := make([]*core.TransactionReceipt, 0)
	pendingGenesis := Pending{
		Block: &core.Block{
			Header: &core.Header{
				ParentHash:       &felt.Zero,
				Number:           0,
				SequencerAddress: &felt.Zero,
				EventsBloom:      core.EventsBloom(receipts),
				GasPrice:         &felt.Zero,
				GasPriceSTRK:     &felt.Zero,
			},
			Transactions: make([]core.Transaction, 0),
			Receipts:     receipts,
		},
		StateUpdate: &core.StateUpdate{
			OldRoot:   &felt.Zero,
			StateDiff: diff,
		},
		NewClasses: classes,
	}
	return b.Finalise(&pendingGenesis, func(_, _ *felt.Felt) ([]*felt.Felt, error) {
		return nil, nil
	}, nil, nil)
}
