package rpc

import (
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
)

var ErrNoBlock = &jsonrpc.Error{Code: 32, Message: "There are no blocks"}

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
