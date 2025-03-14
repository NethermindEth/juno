package peers

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type iterator2 struct {
	bcReader blockchain.Reader2

	blockNumber uint64
	step        uint64
	limit       uint64

	forward    bool
	reachedEnd bool
}

func newIteratorByNumber2(bcReader blockchain.Reader2, blockNumber, limit, step uint64, forward bool) (*iterator2, error) {
	if step == 0 {
		return nil, errors.New("step is zero")
	}
	if limit == 0 {
		return nil, errors.New("limit is zero")
	}

	return &iterator2{
		bcReader:    bcReader,
		blockNumber: blockNumber,
		limit:       limit,
		step:        step,
		forward:     forward,
		reachedEnd:  false,
	}, nil
}

func newIteratorByHash2(bcReader blockchain.Reader2, blockHash *felt.Felt, limit, step uint64, forward bool) (*iterator2, error) {
	if blockHash == nil {
		return nil, errors.New("block hash is nil")
	}

	block, err := bcReader.BlockByHash(blockHash)
	if err != nil {
		return nil, err
	}

	return newIteratorByNumber2(bcReader, block.Number, limit, step, forward)
}

func (it *iterator2) Valid() bool {
	if it.limit == 0 || it.reachedEnd {
		return false
	}

	return true
}

func (it *iterator2) Next() bool {
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

func (it *iterator2) BlockNumber() uint64 {
	return it.blockNumber
}

func (it *iterator2) Block() (*core.Block, error) {
	block, err := it.bcReader.BlockByNumber(it.blockNumber)
	if errors.Is(err, db.ErrKeyNotFound) {
		it.reachedEnd = true
	}

	return block, err
}

func (it *iterator2) Header() (*core.Header, error) {
	header, err := it.bcReader.BlockHeaderByNumber(it.blockNumber)
	if errors.Is(err, db.ErrKeyNotFound) {
		it.reachedEnd = true
	}

	return header, err
}
