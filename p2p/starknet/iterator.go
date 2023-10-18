package starknet

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type iterator struct {
	bcReader blockchain.Reader

	blockHash   *felt.Felt
	blockNumber uint64
	step        uint64
	limit       uint64
	forward     bool

	reachedEnd bool
}

func newIteratorByNumber(bcReader blockchain.Reader, blockNumber, limit, step uint64, forward bool) (*iterator, error) {
	if step == 0 {
		return nil, fmt.Errorf("step is zero")
	}
	if limit == 0 {
		return nil, fmt.Errorf("limit is zero")
	}

	return &iterator{
		bcReader:    bcReader,
		blockNumber: blockNumber,
		blockHash:   nil,
		limit:       limit,
		step:        step,
		forward:     forward,
		reachedEnd:  false,
	}, nil
}

func newIteratorByHash(bcReader blockchain.Reader, blockHash *felt.Felt, limit, step uint64, forward bool) (*iterator, error) {
	if step == 0 {
		return nil, fmt.Errorf("step is zero")
	}
	if limit == 0 {
		return nil, fmt.Errorf("limit is zero")
	}
	if blockHash == nil {
		return nil, fmt.Errorf("block hash is nil")
	}

	return &iterator{
		bcReader:    bcReader,
		blockHash:   blockHash,
		blockNumber: 0,
		limit:       limit,
		step:        step,
		forward:     forward,
		reachedEnd:  false,
	}, nil
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

func (it *iterator) Block() (*core.Block, error) {
	var (
		block *core.Block
		err   error
	)

	if it.blockHash != nil {
		block, err = it.bcReader.BlockByHash(it.blockHash)
		if err == nil {
			// after first successful fetch by hash
			// we switch to iteration by block number
			it.blockHash = nil
			it.blockNumber = block.Number
		}
	} else {
		block, err = it.bcReader.BlockByNumber(it.blockNumber)
	}

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
