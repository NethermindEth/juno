package blockchain

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/fxamacker/cbor/v2"
)

// BlockStorage is just a dumb storage that allows saving and retrieving blocks from database
type BlockStorage struct {
	txn db.Transaction
}

func NewBlockStorage(txn db.Transaction) *BlockStorage {
	return &BlockStorage{txn: txn}
}

// Put stores the given block in the database. No check on whether the hash matches or not is done
func (s *BlockStorage) Put(block *core.Block) error {
	var numBytes [8]byte
	binary.BigEndian.PutUint64(numBytes[:], block.Number)
	if err := s.txn.Set(db.BlockNumbersByHash.Key(block.Hash.Marshal()), numBytes[:]); err != nil {
		return err
	}

	if blockBytes, err := cbor.Marshal(block); err != nil {
		return err
	} else if err = s.txn.Set(db.BlocksByNumber.Key(numBytes[:]), blockBytes); err != nil {
		return err
	}

	return nil
}

// GetByNumber retrieves a block from database by its number
func (s *BlockStorage) GetByNumber(number uint64) (*core.Block, error) {
	var numBytes [8]byte
	binary.BigEndian.PutUint64(numBytes[:], number)
	return s.getByNumber(numBytes[:])
}

func (s *BlockStorage) getByNumber(numBytes []byte) (*core.Block, error) {
	if blockBytes, err := s.txn.Get(db.BlocksByNumber.Key(numBytes)); err != nil {
		return nil, err
	} else {
		block := new(core.Block)
		return block, cbor.Unmarshal(blockBytes, block)
	}
}

// GetByHash retrieves a block from database by its hash
func (s *BlockStorage) GetByHash(hash *felt.Felt) (*core.Block, error) {
	if numBytes, err := s.txn.Get(db.BlockNumbersByHash.Key(hash.Marshal())); err != nil {
		return nil, err
	} else {
		return s.getByNumber(numBytes)
	}
}
