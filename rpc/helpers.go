package rpc

// Helpers contains the supporting functions used in more than one handler from a different groups, e.g. block, trace, etc.
// I break this rule when function name strongly suggest the group, e.g. `AdaptTransaction` which is also used by block handlers.

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
)

func (h *Handler) l1Head() (*core.L1Head, *jsonrpc.Error) {
	l1Head, err := h.bcReader.L1Head()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	// nil is returned if l1 head doesn't exist
	return l1Head, nil
}

func isL1Verified(n uint64, l1 *core.L1Head) bool {
	if l1 != nil && l1.BlockNumber >= n {
		return true
	}
	return false
}

func (h *Handler) blockByID(id *BlockID) (*core.Block, *jsonrpc.Error) {
	var block *core.Block
	var err error
	switch {
	case id.Latest:
		block, err = h.bcReader.Head()
	case id.Hash != nil:
		block, err = h.bcReader.BlockByHash(id.Hash)
	case id.Pending:
		var pending blockchain.Pending
		pending, err = h.bcReader.Pending()
		block = pending.Block
	default:
		block, err = h.bcReader.BlockByNumber(id.Number)
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, ErrBlockNotFound
		}
		return nil, ErrInternal.CloneWithData(err)
	}
	if block == nil {
		return nil, ErrInternal.CloneWithData("nil block with no error")
	}
	return block, nil
}

func (h *Handler) blockHeaderByID(id *BlockID) (*core.Header, *jsonrpc.Error) {
	var header *core.Header
	var err error
	switch {
	case id.Latest:
		header, err = h.bcReader.HeadsHeader()
	case id.Hash != nil:
		header, err = h.bcReader.BlockHeaderByHash(id.Hash)
	case id.Pending:
		var pending blockchain.Pending
		pending, err = h.bcReader.Pending()
		if pending.Block != nil {
			header = pending.Block.Header
		}
	default:
		header, err = h.bcReader.BlockHeaderByNumber(id.Number)
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, ErrBlockNotFound
		}
		return nil, ErrInternal.CloneWithData(err)
	}
	if header == nil {
		return nil, ErrInternal.CloneWithData("nil header with no error")
	}
	return header, nil
}

func adaptExecutionResources(resources *core.ExecutionResources, v0_6Response bool) *ExecutionResources {
	if resources == nil {
		return &ExecutionResources{}
	}

	res := &ExecutionResources{
		ComputationResources: ComputationResources{
			Steps:        resources.Steps,
			MemoryHoles:  resources.MemoryHoles,
			Pedersen:     resources.BuiltinInstanceCounter.Pedersen,
			RangeCheck:   resources.BuiltinInstanceCounter.RangeCheck,
			Bitwise:      resources.BuiltinInstanceCounter.Bitwise,
			Ecsda:        resources.BuiltinInstanceCounter.Ecsda,
			EcOp:         resources.BuiltinInstanceCounter.EcOp,
			Keccak:       resources.BuiltinInstanceCounter.Keccak,
			Poseidon:     resources.BuiltinInstanceCounter.Poseidon,
			SegmentArena: resources.BuiltinInstanceCounter.SegmentArena,
		},
	}
	if !v0_6Response && resources.DataAvailability != nil {
		res.DataAvailability = &DataAvailability{
			L1Gas:     resources.DataAvailability.L1Gas,
			L1DataGas: resources.DataAvailability.L1DataGas,
		}
	}

	return res
}

func (h *Handler) getRevealedBlockHash(blockNumber uint64) (*felt.Felt, error) {
	const blockHashLag = 10
	if blockNumber < blockHashLag {
		return nil, nil
	}

	header, err := h.bcReader.BlockHeaderByNumber(blockNumber - blockHashLag)
	if err != nil {
		return nil, err
	}
	return header.Hash, nil
}

func (h *Handler) callAndLogErr(f func() error, msg string) {
	if err := f(); err != nil {
		h.log.Errorw(msg, "err", err)
	}
}

func feeUnit(txn core.Transaction) FeeUnit {
	feeUnit := WEI
	version := txn.TxVersion()
	if !version.Is(0) && !version.Is(1) && !version.Is(2) {
		feeUnit = FRI
	}

	return feeUnit
}

func (h *Handler) stateByBlockID(id *BlockID) (core.StateReader, blockchain.StateCloser, *jsonrpc.Error) {
	var reader core.StateReader
	var closer blockchain.StateCloser
	var err error
	switch {
	case id.Latest:
		reader, closer, err = h.bcReader.HeadState()
	case id.Hash != nil:
		reader, closer, err = h.bcReader.StateAtBlockHash(id.Hash)
	case id.Pending:
		reader, closer, err = h.bcReader.PendingState()
	default:
		reader, closer, err = h.bcReader.StateAtBlockNumber(id.Number)
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil, ErrBlockNotFound
		}
		return nil, nil, ErrInternal.CloneWithData(err)
	}
	return reader, closer, nil
}
