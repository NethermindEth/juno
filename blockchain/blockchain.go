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
	"github.com/NethermindEth/juno/utils"
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

	HeadState() (core.StateReader, StateCloser, error)
	StateAtBlockHash(blockHash *felt.Felt) (core.StateReader, StateCloser, error)
	StateAtBlockNumber(blockNumber uint64) (core.StateReader, StateCloser, error)
	PendingState() (core.StateReader, StateCloser, error)

	EventFilter(from *felt.Felt, keys [][]felt.Felt) (*EventFilter, error)

	Pending() (Pending, error)
}

var (
	ErrParentDoesNotMatchHead = errors.New("block's parent hash does not match head block hash")
	supportedStarknetVersion  = semver.MustParse("0.11.2")
)

func checkBlockVersion(protocolVersion string) error {
	blockVer, err := core.ParseBlockVersion(protocolVersion)
	if err != nil {
		return err
	}

	if blockVer.GreaterThan(supportedStarknetVersion) {
		return errors.New("unsupported block version")
	}

	return nil
}

var _ Reader = (*Blockchain)(nil)

// Blockchain is responsible for keeping track of all things related to the Starknet blockchain
type Blockchain struct {
	network  utils.Network
	database db.DB

	log utils.SimpleLogger
}

func New(database db.DB, network utils.Network, log utils.SimpleLogger) *Blockchain {
	RegisterCoreTypesToEncoder()
	return &Blockchain{
		database: database,
		network:  network,
		log:      log,
	}
}

func (b *Blockchain) Network() utils.Network {
	return b.network
}

// StateCommitment returns the latest block state commitment.
// If blockchain is empty zero felt is returned.
func (b *Blockchain) StateCommitment() (*felt.Felt, error) {
	var commitment *felt.Felt
	return commitment, b.database.View(func(txn db.Transaction) error {
		var err error
		commitment, err = core.NewState(txn).Root()
		return err
	})
}

// Height returns the latest block height. If blockchain is empty nil is returned.
func (b *Blockchain) Height() (uint64, error) {
	var height uint64
	return height, b.database.View(func(txn db.Transaction) error {
		var err error
		height, err = chainHeight(txn)
		return err
	})
}

func chainHeight(txn db.Transaction) (uint64, error) {
	var height uint64
	return height, txn.Get(db.ChainHeight.Key(), func(val []byte) error {
		height = binary.BigEndian.Uint64(val)
		return nil
	})
}

func (b *Blockchain) Head() (*core.Block, error) {
	var h *core.Block
	return h, b.database.View(func(txn db.Transaction) error {
		var err error
		h, err = head(txn)
		return err
	})
}

func (b *Blockchain) HeadsHeader() (*core.Header, error) {
	var header *core.Header
	return header, b.database.View(func(txn db.Transaction) error {
		height, err := chainHeight(txn)
		if err != nil {
			return err
		}

		header, err = blockHeaderByNumber(txn, height)
		return err
	})
}

func head(txn db.Transaction) (*core.Block, error) {
	height, err := chainHeight(txn)
	if err != nil {
		return nil, err
	}
	return BlockByNumber(txn, height)
}

func (b *Blockchain) BlockByNumber(number uint64) (*core.Block, error) {
	var block *core.Block
	return block, b.database.View(func(txn db.Transaction) error {
		var err error
		block, err = BlockByNumber(txn, number)
		return err
	})
}

func (b *Blockchain) BlockHeaderByNumber(number uint64) (*core.Header, error) {
	var header *core.Header
	return header, b.database.View(func(txn db.Transaction) error {
		var err error
		header, err = blockHeaderByNumber(txn, number)
		return err
	})
}

func (b *Blockchain) BlockByHash(hash *felt.Felt) (*core.Block, error) {
	var block *core.Block
	return block, b.database.View(func(txn db.Transaction) error {
		var err error
		block, err = blockByHash(txn, hash)
		return err
	})
}

func (b *Blockchain) BlockHeaderByHash(hash *felt.Felt) (*core.Header, error) {
	var header *core.Header
	return header, b.database.View(func(txn db.Transaction) error {
		var err error
		header, err = blockHeaderByHash(txn, hash)
		return err
	})
}

func (b *Blockchain) StateUpdateByNumber(number uint64) (*core.StateUpdate, error) {
	var update *core.StateUpdate
	return update, b.database.View(func(txn db.Transaction) error {
		var err error
		update, err = stateUpdateByNumber(txn, number)
		return err
	})
}

func (b *Blockchain) StateUpdateByHash(hash *felt.Felt) (*core.StateUpdate, error) {
	var update *core.StateUpdate
	return update, b.database.View(func(txn db.Transaction) error {
		var err error
		update, err = stateUpdateByHash(txn, hash)
		return err
	})
}

// TransactionByBlockNumberAndIndex gets the transaction for a given block number and index.
func (b *Blockchain) TransactionByBlockNumberAndIndex(blockNumber, index uint64) (core.Transaction, error) {
	var transaction core.Transaction
	return transaction, b.database.View(func(txn db.Transaction) error {
		var err error
		transaction, err = transactionByBlockNumberAndIndex(txn, &txAndReceiptDBKey{blockNumber, index})
		return err
	})
}

// TransactionByHash gets the transaction for a given hash.
func (b *Blockchain) TransactionByHash(hash *felt.Felt) (core.Transaction, error) {
	var transaction core.Transaction
	return transaction, b.database.View(func(txn db.Transaction) error {
		var err error
		transaction, err = transactionByHash(txn, hash)

		// not found in the canonical blocks, try pending
		if errors.Is(err, db.ErrKeyNotFound) {
			var pending Pending
			pending, err = pendingBlock(txn)
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
			pending, err = pendingBlock(txn)
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
func (b *Blockchain) Store(block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class) error {
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

		if err := txn.Delete(db.Pending.Key()); err != nil {
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

	h, err := head(txn)
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

	if err = txn.Set(db.BlockHeadersByNumber.Key(numBytes), headerBytes); err != nil {
		return err
	}

	return nil
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
	block.Transactions, err = transactionsByBlockNumber(txn, number)
	if err != nil {
		return nil, err
	}

	block.Receipts, err = receiptsByBlockNumber(txn, number)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func transactionsByBlockNumber(txn db.Transaction, number uint64) ([]core.Transaction, error) {
	iterator, err := txn.NewIterator()
	if err != nil {
		return nil, err
	}

	var txs []core.Transaction
	numBytes := core.MarshalBlockNumber(number)

	prefix := db.TransactionsByBlockNumberAndIndex.Key(numBytes)
	for iterator.Seek(prefix); iterator.Valid(); iterator.Next() {
		if !bytes.Equal(iterator.Key()[:len(prefix)], prefix) {
			break
		}

		val, vErr := iterator.Value()
		if vErr != nil {
			return nil, db.CloseAndWrapOnError(iterator.Close, vErr)
		}

		var tx core.Transaction
		if err = encoder.Unmarshal(val, &tx); err != nil {
			return nil, db.CloseAndWrapOnError(iterator.Close, err)
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

	var receipts []*core.TransactionReceipt
	numBytes := core.MarshalBlockNumber(number)

	prefix := db.ReceiptsByBlockNumberAndIndex.Key(numBytes)
	for iterator.Seek(prefix); iterator.Valid(); iterator.Next() {
		if !bytes.Equal(iterator.Key()[:len(prefix)], prefix) {
			break
		}

		val, vErr := iterator.Value()
		if vErr != nil {
			return nil, db.CloseAndWrapOnError(iterator.Close, vErr)
		}

		receipt := new(core.TransactionReceipt)
		if err = encoder.Unmarshal(val, receipt); err != nil {
			return nil, db.CloseAndWrapOnError(iterator.Close, err)
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

// SanityCheckNewHeight checks integrity of a block and resulting state update
func (b *Blockchain) SanityCheckNewHeight(block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class) error {
	if !block.Hash.Equal(stateUpdate.BlockHash) {
		return errors.New("block hashes do not match")
	}
	if !block.GlobalStateRoot.Equal(stateUpdate.NewRoot) {
		return errors.New("block's GlobalStateRoot does not match state update's NewRoot")
	}

	if err := core.VerifyClassHashes(newClasses); err != nil {
		return err
	}

	return core.VerifyBlockHash(block, b.network)
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
	txn := b.database.NewTransaction(false)
	_, err := chainHeight(txn)
	if err != nil {
		return nil, nil, db.CloseAndWrapOnError(txn.Discard, err)
	}

	return core.NewState(txn), txn.Discard, nil
}

// StateAtBlockNumber returns a StateReader that provides a stable view to the state at the given block number
func (b *Blockchain) StateAtBlockNumber(blockNumber uint64) (core.StateReader, StateCloser, error) {
	txn := b.database.NewTransaction(false)
	_, err := blockHeaderByNumber(txn, blockNumber)
	if err != nil {
		return nil, nil, db.CloseAndWrapOnError(txn.Discard, err)
	}

	return core.NewStateSnapshot(core.NewState(txn), blockNumber), txn.Discard, nil
}

// StateAtBlockHash returns a StateReader that provides a stable view to the state at the given block hash
func (b *Blockchain) StateAtBlockHash(blockHash *felt.Felt) (core.StateReader, StateCloser, error) {
	txn := b.database.NewTransaction(false)
	header, err := blockHeaderByHash(txn, blockHash)
	if err != nil {
		return nil, nil, db.CloseAndWrapOnError(txn.Discard, err)
	}

	return core.NewStateSnapshot(core.NewState(txn), header.Number), txn.Discard, nil
}

// EventFilter returns an EventFilter object that is tied to a snapshot of the blockchain
func (b *Blockchain) EventFilter(from *felt.Felt, keys [][]felt.Felt) (*EventFilter, error) {
	txn := b.database.NewTransaction(false)
	latest, err := chainHeight(txn)
	if err != nil {
		return nil, err
	}

	return newEventFilter(txn, from, keys, 0, latest), nil
}

// RevertHead reverts the head block
func (b *Blockchain) RevertHead() error {
	return b.database.Update(revertHead)
}

func revertHead(txn db.Transaction) error {
	blockNumber, err := chainHeight(txn)
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

	header, err := blockHeaderByNumber(txn, blockNumber)
	if err != nil {
		return err
	}

	// remove block header
	if err = txn.Delete(db.BlockHeadersByNumber.Key(numBytes)); err != nil {
		return err
	}
	if err = txn.Delete(db.BlockHeaderNumbersByHash.Key(header.Hash.Marshal())); err != nil {
		return err
	}

	blockIDAndIndex := txAndReceiptDBKey{
		Number: blockNumber,
	}
	// remove txs and receipts
	for i := uint64(0); i < header.TransactionCount; i++ {
		blockIDAndIndex.Index = i
		var reorgedTxn core.Transaction
		reorgedTxn, err = transactionByBlockNumberAndIndex(txn, &blockIDAndIndex)
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
	}

	// remove state update
	if err = txn.Delete(db.StateUpdatesByBlockNumber.Key(numBytes)); err != nil {
		return err
	}

	// remove pending
	if err := txn.Delete(db.Pending.Key()); err != nil {
		return err
	}

	// update chain height
	if blockNumber == 0 {
		return txn.Delete(db.ChainHeight.Key())
	}

	heightBin := core.MarshalBlockNumber(blockNumber - 1)
	return txn.Set(db.ChainHeight.Key(), heightBin)
}

// StorePending stores a pending block given that it is for the next height
func (b *Blockchain) StorePending(pending *Pending) error {
	return b.database.Update(func(txn db.Transaction) error {
		expectedParent := new(felt.Felt)
		expectedOldRoot := new(felt.Felt)
		h, err := head(txn)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		} else if err == nil {
			expectedParent = h.Hash
			expectedOldRoot = h.GlobalStateRoot
		}

		if !expectedParent.Equal(pending.Block.ParentHash) || !expectedOldRoot.Equal(pending.StateUpdate.OldRoot) {
			return errors.New("pending block parent is not our local HEAD")
		}

		existingPending, err := pendingBlock(txn)
		if err == nil && existingPending.Block.TransactionCount >= pending.Block.TransactionCount {
			return nil // ignore the incoming pending if it has fewer transactions than the one we already have
		}

		pendingBytes, err := encoder.Marshal(pending)
		if err != nil {
			return err
		}
		return txn.Set(db.Pending.Key(), pendingBytes)
	})
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
	var pending Pending
	return pending, b.database.View(func(txn db.Transaction) error {
		var err error
		pending, err = pendingBlock(txn)
		return err
	})
}

// PendingState returns the state resulting from execution of the pending block
func (b *Blockchain) PendingState() (core.StateReader, StateCloser, error) {
	txn := b.database.NewTransaction(false)
	pending, err := pendingBlock(txn)
	if err != nil {
		return nil, nil, db.CloseAndWrapOnError(txn.Discard, err)
	}

	return NewPendingState(
		pending,
		core.NewState(txn),
	), txn.Discard, nil
}
