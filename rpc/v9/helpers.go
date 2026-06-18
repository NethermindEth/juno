package rpcv9

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
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"go.uber.org/zap"
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

// l1AcceptedBlockNumber returns the L1-accepted block number bounded by the
// current chain height. The L1 head can be ahead of the L2 chain during sync,
// in which case the caller should fall back to the latest local block.
func (h *Handler) l1AcceptedBlockNumber() (uint64, error) {
	l1Head, err := h.bcReader.L1Head()
	if err != nil {
		return 0, err
	}
	height, err := h.bcReader.Height()
	if err != nil {
		return 0, err
	}
	return min(l1Head.BlockNumber, height), nil
}

func (h *Handler) blockByID(blockID *BlockID) (*core.Block, *jsonrpc.Error) {
	var block *core.Block
	var err error

	switch blockID.Type() {
	case preConfirmed:
		var chain preconfirmed.ChainReader
		chain, err = h.syncReader.PreConfirmedChain()
		if err == nil {
			block = chain.Head().Block
		}
	case latest:
		block, err = h.bcReader.Head()
	case hash:
		block, err = h.bcReader.BlockByHash(blockID.Hash())
	case l1Accepted:
		var blockNumber uint64
		blockNumber, err = h.l1AcceptedBlockNumber()
		if err != nil {
			break
		}
		block, err = h.bcReader.BlockByNumber(blockNumber)
	default:
		block, err = h.bcReader.BlockByNumber(blockID.Number())
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrBlockNotFound
		}
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}
	if block == nil {
		return nil, rpccore.ErrInternal.CloneWithData("nil block with no error")
	}
	return block, nil
}

func (h *Handler) blockTxnsByNumber(blockID *BlockID) ([]core.Transaction, *jsonrpc.Error) {
	switch blockID.Type() {
	case preConfirmed:
		chain, err := h.syncReader.PreConfirmedChain()
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				return nil, rpccore.ErrBlockNotFound
			}
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return chain.Head().Block.Transactions, nil
	default:
		txns, err := h.bcReader.TransactionsByBlockNumber(blockID.Number())
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				return nil, rpccore.ErrBlockNotFound
			}
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return txns, nil
	}
}

func (h *Handler) blockHeaderByID(blockID *BlockID) (*core.Header, *jsonrpc.Error) {
	var header *core.Header
	var err error
	switch blockID.Type() {
	case preConfirmed:
		var chain preconfirmed.ChainReader
		chain, err = h.syncReader.PreConfirmedChain()
		if err == nil {
			header = chain.Head().Block.Header
		}
	case latest:
		header, err = h.bcReader.HeadsHeader()
	case hash:
		header, err = h.bcReader.BlockHeaderByHash(blockID.Hash())
	case number:
		header, err = h.bcReader.BlockHeaderByNumber(blockID.Number())
	case l1Accepted:
		var blockNumber uint64
		blockNumber, err = h.l1AcceptedBlockNumber()
		if err != nil {
			break
		}
		header, err = h.bcReader.BlockHeaderByNumber(blockNumber)
	default:
		panic("unknown block type id")
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
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
		return &ExecutionResources{}
	}

	res := &ExecutionResources{}
	if tgc := resources.TotalGasConsumed; tgc != nil {
		res.L1Gas = tgc.L1Gas
		res.L2Gas = tgc.L2Gas
		res.L1DataGas = tgc.L1DataGas
	}
	return res
}

func (h *Handler) getRevealedBlockHash(blockNumber uint64) (*felt.Felt, error) {
	if blockNumber < core.BlockHashLag {
		return nil, nil
	}

	header, err := h.bcReader.BlockHeaderByNumber(blockNumber - core.BlockHashLag)
	if err != nil {
		return nil, err
	}
	return header.Hash, nil
}

func (h *Handler) callAndLogErr(f func() error, msg string) {
	if err := f(); err != nil {
		h.logger.Error(msg, zap.Error(err))
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

func (h *Handler) stateByBlockID(
	blockID *BlockID,
) (core.StateReader, blockchain.StateCloser, *jsonrpc.Error) {
	var reader core.StateReader
	var closer blockchain.StateCloser
	var err error
	switch blockID.Type() {
	case preConfirmed:
		var chain preconfirmed.ChainReader
		chain, err = h.syncReader.PreConfirmedChain()
		if err == nil {
			reader, closer, err = chain.PreConfirmedStateAt(chain.Head().Block.Number, h.bcReader)
		}
	case latest:
		reader, closer, err = h.bcReader.HeadState()
	case hash:
		reader, closer, err = h.bcReader.StateAtBlockHash(blockID.Hash())
	case number:
		reader, closer, err = h.bcReader.StateAtBlockNumber(blockID.Number())
	case l1Accepted:
		var blockNumber uint64
		blockNumber, err = h.l1AcceptedBlockNumber()
		if err != nil {
			break
		}
		reader, closer, err = h.bcReader.StateAtBlockNumber(blockNumber)
	default:
		panic("unknown block id type")
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil, rpccore.ErrBlockNotFound
		}
		return nil, nil, rpccore.ErrInternal.CloneWithData(err)
	}
	return reader, closer, nil
}
