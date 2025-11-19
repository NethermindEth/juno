package rpcv7

// Helpers contains the supporting functions used in more than one handler from a different groups, e.g. block, trace, etc.
// I break this rule when function name strongly suggest the group, e.g. `AdaptTransaction` which is also used by block handlers.

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

func (h *Handler) l1Head() (core.L1Head, *jsonrpc.Error) {
	l1Head, err := h.bcReader.L1Head()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return core.L1Head{}, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	// empty L1Head is returned if l1 head doesn't exist
	return l1Head, nil
}

func isL1Verified(n uint64, l1 core.L1Head) bool {
	if l1 != (core.L1Head{}) && l1.BlockNumber >= n {
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
		var pending core.PendingData
		pending, err = h.PendingData()
		if err == nil {
			block = pending.GetBlock()
		}
	default:
		block, err = h.bcReader.BlockByNumber(id.Number)
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, core.ErrPendingDataNotFound) {
			return nil, rpccore.ErrBlockNotFound
		}
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}
	if block == nil {
		return nil, rpccore.ErrInternal.CloneWithData("nil block with no error")
	}
	return block, nil
}

func (h *Handler) blockHeaderByID(id BlockIdentifier) (*core.Header, *jsonrpc.Error) {
	var header *core.Header
	var err error

	switch {
	case id.IsLatest():
		header, err = h.bcReader.HeadsHeader()
	case id.GetHash() != nil:
		header, err = h.bcReader.BlockHeaderByHash(id.GetHash())
	case id.IsPending():
		var pending core.PendingData
		pending, err = h.PendingData()
		if err == nil {
			header = pending.GetHeader()
		}
	default:
		header, err = h.bcReader.BlockHeaderByNumber(id.GetNumber())
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, core.ErrPendingDataNotFound) {
			return nil, rpccore.ErrBlockNotFound
		}
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}
	if header == nil {
		return nil, rpccore.ErrInternal.CloneWithData("nil header with no error")
	}
	return header, nil
}

func adaptExecutionResources(resources *core.ExecutionResources) *ExecutionResources {
	if resources == nil {
		dataAvailability := &DataAvailability{}
		return &ExecutionResources{
			DataAvailability: dataAvailability,
		}
	}

	res := &ExecutionResources{
		ComputationResources: ComputationResources{
			Steps:        resources.Steps,
			MemoryHoles:  resources.MemoryHoles,
			Pedersen:     resources.BuiltinInstanceCounter.Pedersen,
			RangeCheck:   resources.BuiltinInstanceCounter.RangeCheck,
			Bitwise:      resources.BuiltinInstanceCounter.Bitwise,
			Ecdsa:        resources.BuiltinInstanceCounter.Ecsda,
			EcOp:         resources.BuiltinInstanceCounter.EcOp,
			Keccak:       resources.BuiltinInstanceCounter.Keccak,
			Poseidon:     resources.BuiltinInstanceCounter.Poseidon,
			SegmentArena: resources.BuiltinInstanceCounter.SegmentArena,
		},
	}

	if resources.DataAvailability == nil {
		res.DataAvailability = &DataAvailability{}
	} else {
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

func (h *Handler) stateByBlockID(id *BlockID) (core.CommonStateReader, blockchain.StateCloser, *jsonrpc.Error) {
	var reader core.CommonStateReader
	var closer blockchain.StateCloser
	var err error
	switch {
	case id.Latest:
		reader, closer, err = h.bcReader.HeadState()
	case id.Hash != nil:
		reader, closer, err = h.bcReader.StateAtBlockHash(id.Hash)
	case id.Pending:
		reader, closer, err = h.PendingState()
	default:
		reader, closer, err = h.bcReader.StateAtBlockNumber(id.Number)
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, core.ErrPendingDataNotFound) {
			return nil, nil, rpccore.ErrBlockNotFound
		}
		return nil, nil, rpccore.ErrInternal.CloneWithData(err)
	}
	return reader, closer, nil
}
