package server

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type iterator struct {
	bcReader blockchain.Reader

	blockNumber uint64
	step        uint64
	limit       uint64

	forward    bool
	reachedEnd bool
}

func newIteratorByNumber(bcReader blockchain.Reader, blockNumber, limit, step uint64, forward bool) (*iterator, error) {
	if step == 0 {
		return nil, errors.New("step is zero")
	}
	if limit == 0 {
		return nil, errors.New("limit is zero")
	}

	return &iterator{
		bcReader:    bcReader,
		blockNumber: blockNumber,
		limit:       limit,
		step:        step,
		forward:     forward,
		reachedEnd:  false,
	}, nil
}

func newIteratorByHash(bcReader blockchain.Reader, blockHash *felt.Felt, limit, step uint64, forward bool) (*iterator, error) {
	if blockHash == nil {
		return nil, errors.New("block hash is nil")
	}

	block, err := bcReader.BlockByHash(blockHash)
	if err != nil {
		return nil, err
	}

	return newIteratorByNumber(bcReader, block.Number, limit, step, forward)
}

func (it *iterator) Valid() bool {
	if it.limit == 0 || it.reachedEnd {
		return false
	}

	return true
}

func (it *iterator) Next() bool {
	if !it.Valid() {
		return false
	}

	if it.forward {
		it.blockNumber += it.step
	} else {
		it.blockNumber -= it.step
	}
	// assumption that it.Valid checks for zero limit i.e. no overflow is possible here
	it.limit--

	return it.Valid()
}

func (it *iterator) BlockNumber() uint64 {
	return it.blockNumber
}

func (it *iterator) Block() (*core.Block, error) {
	block, err := it.bcReader.BlockByNumber(it.blockNumber)
	if errors.Is(err, db.ErrKeyNotFound) {
		it.reachedEnd = true
	}

	return block, err
}

func (it *iterator) Header() (*core.Header, error) {
	header, err := it.bcReader.BlockHeaderByNumber(it.blockNumber)
	if errors.Is(err, db.ErrKeyNotFound) {
		it.reachedEnd = true
	}

	return header, err
}
