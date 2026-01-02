package rpcv10

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
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

func (h *Handler) blockByID(blockID *rpcv9.BlockID) (*core.Block, *jsonrpc.Error) {
	var block *core.Block
	var err error

	switch {
	case blockID.IsPreConfirmed():
		var pending core.PendingData
		pending, err = h.PendingData()
		if err == nil {
			block = pending.GetBlock()
		}
	case blockID.IsLatest():
		block, err = h.bcReader.Head()
	case blockID.IsHash():
		block, err = h.bcReader.BlockByHash(blockID.Hash())
	case blockID.IsL1Accepted():
		var l1Head core.L1Head
		l1Head, err = h.bcReader.L1Head()
		if err != nil {
			break
		}
		block, err = h.bcReader.BlockByNumber(l1Head.BlockNumber)
	default:
		block, err = h.bcReader.BlockByNumber(blockID.Number())
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

func (h *Handler) blockTxnsByNumber(blockID *rpcv9.BlockID) ([]core.Transaction, *jsonrpc.Error) {
	switch {
	case blockID.IsPreConfirmed():
		pending, err := h.PendingData()
		if err != nil {
			if errors.Is(err, core.ErrPendingDataNotFound) {
				return nil, rpccore.ErrBlockNotFound
			}
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		txns := pending.GetTransactions()
		return txns, nil
	default:
		txns, err := h.bcReader.TransactionsByBlockNumber(blockID.Number())
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, core.ErrPendingDataNotFound) {
				return nil, rpccore.ErrBlockNotFound
			}
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return txns, nil
	}
}

func (h *Handler) blockHeaderByID(blockID *rpcv9.BlockID) (*core.Header, *jsonrpc.Error) {
	var header *core.Header
	var err error
	switch {
	case blockID.IsPreConfirmed():
		var pending core.PendingData
		pending, err = h.PendingData()
		if err == nil {
			header = pending.GetBlock().Header
		}
	case blockID.IsLatest():
		header, err = h.bcReader.HeadsHeader()
	case blockID.IsHash():
		header, err = h.bcReader.BlockHeaderByHash(blockID.Hash())
	case blockID.IsNumber():
		header, err = h.bcReader.BlockHeaderByNumber(blockID.Number())
	case blockID.IsL1Accepted():
		var l1Head core.L1Head
		l1Head, err = h.bcReader.L1Head()
		if err != nil {
			break
		}
		header, err = h.bcReader.BlockHeaderByNumber(l1Head.BlockNumber)
	default:
		panic("unknown block type id")
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

func (h *Handler) stateByBlockID(
	blockID *rpcv9.BlockID,
) (core.StateReader, blockchain.StateCloser, *jsonrpc.Error) {
	var reader core.StateReader
	var closer blockchain.StateCloser
	var err error
	switch {
	case blockID.IsPreConfirmed():
		reader, closer, err = h.PendingState()
	case blockID.IsLatest():
		reader, closer, err = h.bcReader.HeadState()
	case blockID.IsHash():
		reader, closer, err = h.bcReader.StateAtBlockHash(blockID.Hash())
	case blockID.IsNumber():
		reader, closer, err = h.bcReader.StateAtBlockNumber(blockID.Number())
	case blockID.IsL1Accepted():
		var l1Head core.L1Head
		l1Head, err = h.bcReader.L1Head()
		if err != nil {
			break
		}
		reader, closer, err = h.bcReader.StateAtBlockNumber(l1Head.BlockNumber)
	default:
		panic("unknown block id type")
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) || errors.Is(err, core.ErrPendingDataNotFound) {
			return nil, nil, rpccore.ErrBlockNotFound
		}
		return nil, nil, rpccore.ErrInternal.CloneWithData(err)
	}
	return reader, closer, nil
}

func getTransactionType(t core.Transaction) rpcv9.TransactionType {
	switch t.(type) {
	case *core.DeployTransaction:
		return rpcv9.TxnDeploy
	case *core.InvokeTransaction:
		return rpcv9.TxnInvoke
	case *core.DeclareTransaction:
		return rpcv9.TxnDeclare
	case *core.DeployAccountTransaction:
		return rpcv9.TxnDeployAccount
	case *core.L1HandlerTransaction:
		return rpcv9.TxnL1Handler
	default:
		panic("unknown transaction type")
	}
}

// getCommitmentsAndStateDiff retrieves commitments and stateDiff by block number.
func (h *Handler) getCommitmentsAndStateDiff(
	blockNumber uint64,
) (*core.BlockCommitments, *core.StateDiff, error) {
	// Get commitments
	commitments, err := h.bcReader.BlockCommitmentsByNumber(blockNumber)
	if err != nil {
		return nil, nil, err
	}

	// Get stateDiff from stateUpdate
	stateUpdate, err := h.bcReader.StateUpdateByNumber(blockNumber)
	if err != nil {
		return nil, nil, err
	}

	return commitments, stateUpdate.StateDiff, nil
}
