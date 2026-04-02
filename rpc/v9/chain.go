package rpcv9

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
)

/****************************************************
		Chain Handlers
*****************************************************/

// ChainID returns the chain ID of the currently configured network.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/c2e93098b9c2ca0423b7f4d15b201f52f22d8c36/api/starknet_api_openrpc.json#L767
func (h *Handler) ChainID() (*felt.Felt, *jsonrpc.Error) {
	return h.bcReader.Network().L2ChainIDFelt(), nil
}
