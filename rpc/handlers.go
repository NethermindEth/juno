package rpc

import (
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
)

var (
	ErrBlockNotFound = &jsonrpc.Error{Code: 24, Message: "Block not found"}
	ErrNoBlock       = &jsonrpc.Error{Code: 32, Message: "There are no blocks"}
)

type Handler struct {
	bcReader blockchain.Reader

	chainId *felt.Felt
}

func New(bcReader blockchain.Reader, chainId *felt.Felt) *Handler {
	return &Handler{
		bcReader: bcReader,
		chainId:  chainId,
	}
}

func (h *Handler) ChainId() (*felt.Felt, *jsonrpc.Error) {
	return h.chainId, nil
}

func (h *Handler) BlockNumber() (uint64, *jsonrpc.Error) {
	num, err := h.bcReader.Height()
	if err != nil {
		return 0, ErrNoBlock
	}

	return num, nil
}

func (h *Handler) BlockNumberAndHash() (*BlockNumberAndHash, *jsonrpc.Error) {
	if block, err := h.bcReader.Head(); err != nil {
		return nil, ErrNoBlock
	} else {
		return &BlockNumberAndHash{Number: block.Number, Hash: block.Hash}, nil
	}
}

func (h *Handler) GetBlockWithTxHashes(id *BlockId) (*BlockWithTxHashes, *jsonrpc.Error) {
	var block *core.Block
	var err error
	if id.Latest {
		block, err = h.bcReader.Head()
	} else if id.Pending {
		// todo
	} else if id.Hash != nil {
		block, err = h.bcReader.GetBlockByHash(id.Hash)
	} else {
		block, err = h.bcReader.GetBlockByNumber(id.Number)
	}

	if err != nil || block == nil {
		return nil, ErrBlockNotFound
	}

	txnHashes := make([]*felt.Felt, len(block.Transactions))
	for index, txn := range block.Transactions {
		txnHashes[index] = txn.Hash()
	}

	return &BlockWithTxHashes{
		Status: BlockStatusAcceptedL2, // todo
		BlockHeader: BlockHeader{
			Hash:             block.Hash,
			ParentHash:       block.ParentHash,
			Number:           block.Number,
			NewRoot:          block.GlobalStateRoot,
			Timestamp:        block.Timestamp,
			SequencerAddress: block.SequencerAddress,
		},
		TxnHashes: txnHashes,
	}, nil
}
