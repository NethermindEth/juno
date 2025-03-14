package blockchain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	// "github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

// type L1HeadSubscription struct {
// 	*feed.Subscription[*core.L1Head]
// }

// //go:generate mockgen -destination=../mocks/mock_blockchain.go -package=mocks github.com/NethermindEth/juno/blockchain Reader
type Reader2 interface {
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

	HeadState() (core.StateReader2, StateCloser, error)
	StateAtBlockHash(blockHash *felt.Felt) (core.StateReader2, StateCloser, error)
	StateAtBlockNumber(blockNumber uint64) (core.StateReader2, StateCloser, error)

	BlockCommitmentsByNumber(blockNumber uint64) (*core.BlockCommitments, error)

	EventFilter(from *felt.Felt, keys [][]felt.Felt) (EventFilterer, error)

	Network() *utils.Network
}

// var (
// 	ErrParentDoesNotMatchHead = errors.New("block's parent hash does not match head block hash")
// 	SupportedStarknetVersion  = semver.MustParse("0.13.3")
// )

// func CheckBlockVersion(protocolVersion string) error {
// 	blockVer, err := core.ParseBlockVersion(protocolVersion)
// 	if err != nil {
// 		return err
// 	}

// 	// We ignore changes in patch part of the version
// 	blockVerMM, supportedVerMM := copyWithoutPatch(blockVer), copyWithoutPatch(SupportedStarknetVersion)
// 	if blockVerMM.GreaterThan(supportedVerMM) {
// 		return errors.New("unsupported block version")
// 	}

// 	return nil
// }

// func copyWithoutPatch(v *semver.Version) *semver.Version {
// 	if v == nil {
// 		return nil
// 	}

// 	return semver.New(v.Major(), v.Minor(), 0, v.Prerelease(), v.Metadata())
// }

var _ Reader2 = (*Blockchain2)(nil)

// Blockchain2 is responsible for keeping track of all things related to the Starknet blockchain
type Blockchain2 struct {
	network        *utils.Network
	database2      db.KeyValueStore // TODO(weiihann): deal with this
	listener       EventListener
	l1HeadFeed     *feed.Feed[*core.L1Head]
	pendingBlockFn func() *core.Block
}

func New2(database db.KeyValueStore, network *utils.Network) *Blockchain2 {
	return &Blockchain2{
		database2:  database,
		network:    network,
		listener:   &SelectiveListener{},
		l1HeadFeed: feed.New[*core.L1Head](),
	}
}

func (b *Blockchain2) WithPendingBlockFn(pendingBlockFn func() *core.Block) *Blockchain2 {
	b.pendingBlockFn = pendingBlockFn
	return b
}

func (b *Blockchain2) WithListener(listener EventListener) *Blockchain2 {
	b.listener = listener
	return b
}

func (b *Blockchain2) Network() *utils.Network {
	return b.network
}

// StateCommitment returns the latest block state commitment.
// If blockchain is empty zero felt is returned.
func (b *Blockchain2) StateCommitment() (*felt.Felt, error) {
	b.listener.OnRead("StateCommitment")
	batch := b.database2.NewIndexedBatch() // this is a hack because we don't need to write to the db
	return core.NewState2(batch).Root()
}

// Height returns the latest block height. If blockchain is empty nil is returned.
func (b *Blockchain2) Height() (uint64, error) {
	b.listener.OnRead("Height")
	return GetChainHeight(b.database2)
}

func (b *Blockchain2) Head() (*core.Block, error) {
	b.listener.OnRead("Head")
	curHeight, err := GetChainHeight(b.database2)
	if err != nil {
		return nil, err
	}

	txn := b.database2.NewIndexedBatch()
	return GetBlockByNumber(txn, curHeight)
}

func (b *Blockchain2) HeadsHeader() (*core.Header, error) {
	b.listener.OnRead("HeadsHeader")
	height, err := GetChainHeight(b.database2)
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(b.database2, height)
}

func headsHeader2(txn db.KeyValueReader) (*core.Header, error) {
	height, err := GetChainHeight(txn)
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(txn, height)
}

func (b *Blockchain2) BlockByNumber(number uint64) (*core.Block, error) {
	b.listener.OnRead("BlockByNumber")
	txn := b.database2.NewIndexedBatch()
	return GetBlockByNumber(txn, number)
}

func (b *Blockchain2) BlockHeaderByNumber(number uint64) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByNumber")
	return GetBlockHeaderByNumber(b.database2, number)
}

func (b *Blockchain2) BlockByHash(hash *felt.Felt) (*core.Block, error) {
	b.listener.OnRead("BlockByHash")
	blockNum, err := GetBlockHeaderNumberByHash(b.database2, hash)
	if err != nil {
		return nil, err
	}

	txn := b.database2.NewIndexedBatch()
	return GetBlockByNumber(txn, blockNum)
}

func (b *Blockchain2) BlockHeaderByHash(hash *felt.Felt) (*core.Header, error) {
	b.listener.OnRead("BlockHeaderByHash")
	return GetBlockHeaderByHash(b.database2, hash)
}

func (b *Blockchain2) StateUpdateByNumber(number uint64) (*core.StateUpdate, error) {
	b.listener.OnRead("StateUpdateByNumber")
	return GetStateUpdateByBlockNum(b.database2, number)
}

func (b *Blockchain2) StateUpdateByHash(hash *felt.Felt) (*core.StateUpdate, error) {
	b.listener.OnRead("StateUpdateByHash")
	return GetStateUpdateByHash(b.database2, hash)
}

func (b *Blockchain2) L1HandlerTxnHash(msgHash *common.Hash) (*felt.Felt, error) {
	b.listener.OnRead("L1HandlerTxnHash")
	txnHash, err := GetL1HandlerTxnHashByMsgHash(b.database2, msgHash.Bytes())
	return &txnHash, err // TODO: return felt value
}

// TransactionByBlockNumberAndIndex gets the transaction for a given block number and index.
func (b *Blockchain2) TransactionByBlockNumberAndIndex(blockNumber, index uint64) (core.Transaction, error) {
	b.listener.OnRead("TransactionByBlockNumberAndIndex")
	return GetTxByBlockNumIndex(b.database2, blockNumber, index)
}

// TransactionByHash gets the transaction for a given hash.
func (b *Blockchain2) TransactionByHash(hash *felt.Felt) (core.Transaction, error) {
	b.listener.OnRead("TransactionByHash")
	return GetTxByHash(b.database2, hash)
}

// Receipt gets the transaction receipt for a given transaction hash.
func (b *Blockchain2) Receipt(hash *felt.Felt) (*core.TransactionReceipt, *felt.Felt, uint64, error) {
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

func (b *Blockchain2) SubscribeL1Head() L1HeadSubscription {
	return L1HeadSubscription{b.l1HeadFeed.Subscribe()}
}

func (b *Blockchain2) L1Head() (*core.L1Head, error) {
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

func (b *Blockchain2) SetL1Head(update *core.L1Head) error {
	data, err := encoder.Marshal(update)
	if err != nil {
		return err
	}
	return b.database2.Put(db.L1Height.Key(), data)
}

// Store takes a block and state update and performs sanity checks before putting in the database.
func (b *Blockchain2) Store(block *core.Block, blockCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class,
) error {
	return b.database2.Update2(func(txn db.IndexedBatch) error {
		if err := verifyBlock2(txn, block); err != nil {
			return err
		}

		if err := core.NewState2(txn).Update(block.Number, stateUpdate, newClasses); err != nil {
			return err
		}
		if err := WriteBlockHeader(txn, block.Header); err != nil {
			return err
		}

		for i, tx := range block.Transactions {
			if err := WriteTxAndReceipt(txn, block.Number, uint64(i), tx,
				block.Receipts[i]); err != nil {
				return err
			}
		}

		if err := WriteStateUpdateByBlockNum(txn, block.Number, stateUpdate); err != nil {
			return err
		}

		if err := WriteBlockCommitment(txn, block.Number, blockCommitments); err != nil {
			return err
		}

		if err := WriteL1HandlerMsgHashes(txn, block.Transactions); err != nil {
			return err
		}

		// Head of the blockchain is maintained as follows:
		// [db.ChainHeight]() -> (BlockNumber)
		heightBin := core.MarshalBlockNumber(block.Number)
		return txn.Put(db.ChainHeight.Key(), heightBin)
	})
}

// VerifyBlock assumes the block has already been sanity-checked.
func (b *Blockchain2) VerifyBlock(block *core.Block) error {
	return verifyBlock2(b.database2, block)
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

func (b *Blockchain2) BlockCommitmentsByNumber(blockNumber uint64) (*core.BlockCommitments, error) {
	b.listener.OnRead("BlockCommitmentsByNumber")
	return GetBlockCommitmentByBlockNum(b.database2, blockNumber)
}

// SanityCheckNewHeight checks integrity of a block and resulting state update
func (b *Blockchain2) SanityCheckNewHeight(block *core.Block, stateUpdate *core.StateUpdate,
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

type StateCloser = func() error

var noopStateCloser = func() error { return nil }

// HeadState returns a StateReader that provides a stable view to the latest state
// TODO(weiihann): handle the interface later....
func (b *Blockchain2) HeadState() (core.StateReader2, StateCloser, error) {
	b.listener.OnRead("HeadState")
	txn := b.database2.NewIndexedBatch()

	_, err := GetChainHeight(txn)
	if err != nil {
		return nil, nil, err
	}

	return core.NewState2(txn), noopStateCloser, nil
}

// StateAtBlockNumber returns a StateReader that provides a stable view to the state at the given block number
func (b *Blockchain2) StateAtBlockNumber(blockNumber uint64) (core.StateReader2, StateCloser, error) {
	b.listener.OnRead("StateAtBlockNumber")
	txn := b.database2.NewIndexedBatch()

	_, err := GetBlockHeaderByNumber(txn, blockNumber)
	if err != nil {
		return nil, nil, err
	}

	return core.NewStateSnapshot2(core.NewState2(txn), blockNumber), noopStateCloser, nil
}

// StateAtBlockHash returns a StateReader that provides a stable view to the state at the given block hash
func (b *Blockchain2) StateAtBlockHash(blockHash *felt.Felt) (core.StateReader2, StateCloser, error) {
	b.listener.OnRead("StateAtBlockHash")
	if blockHash.IsZero() {
		memDB := memory.New()
		txn := memDB.NewIndexedBatch()
		emptyState := core.NewState2(txn)
		return emptyState, noopStateCloser, nil
	}

	txn := b.database2.NewIndexedBatch()
	header, err := GetBlockHeaderByHash(txn, blockHash)
	if err != nil {
		return nil, nil, err
	}

	return core.NewStateSnapshot2(core.NewState2(txn), header.Number), noopStateCloser, nil
}

// EventFilter returns an EventFilter object that is tied to a snapshot of the blockchain
func (b *Blockchain2) EventFilter(from *felt.Felt, keys [][]felt.Felt) (EventFilterer, error) {
	b.listener.OnRead("EventFilter")
	latest, err := GetChainHeight(b.database2)
	if err != nil {
		return nil, err
	}

	return newEventFilter(b.database2, from, keys, 0, latest, b.pendingBlockFn), nil
}

// RevertHead reverts the head block
func (b *Blockchain2) RevertHead() error {
	return b.database2.Update2(b.revertHead)
}

func (b *Blockchain2) GetReverseStateDiff() (*core.StateDiff, error) {
	var reverseStateDiff *core.StateDiff

	txn := b.database2.NewIndexedBatch()
	blockNum, err := GetChainHeight(txn)
	if err != nil {
		return nil, err
	}

	stateUpdate, err := GetStateUpdateByBlockNum(txn, blockNum)
	if err != nil {
		return nil, err
	}

	state := core.NewState2(txn)
	reverseStateDiff, err = state.GetReverseStateDiff(blockNum, stateUpdate.StateDiff)
	if err != nil {
		return nil, err
	}

	return reverseStateDiff, nil
}

func (b *Blockchain2) revertHead(txn db.IndexedBatch) error {
	blockNumber, err := GetChainHeight(txn)
	if err != nil {
		return err
	}

	stateUpdate, err := GetStateUpdateByBlockNum(txn, blockNumber)
	if err != nil {
		return err
	}

	state := core.NewState2(txn)
	// revert state
	if err = state.Revert(blockNumber, stateUpdate); err != nil {
		return err
	}

	header, err := GetBlockHeaderByNumber(txn, blockNumber)
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

	if err = removeTxsAndReceipts2(txn, blockNumber, header.TransactionCount); err != nil {
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
	return txn.Put(db.ChainHeight.Key(), heightBin)
}
