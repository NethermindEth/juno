package rpcv10

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

/****************************************************
		Nonce Handler
*****************************************************/

// Nonce returns the nonce associated with the given address in the given block number
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/v0.10.3/api/starknet_api_openrpc.json#L940
func (h *Handler) Nonce(id *BlockID, address *felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getNonce")

	// System contracts (0x1, 0x2) hold storage but have no Cairo class.
	if state.IsSystemContract(address) {
		return nil, rpccore.ErrContractNotFound
	}

	nonce, err := stateReader.ContractNonce(address)
	if err != nil {
		return nil, rpccore.ErrContractNotFound
	}

	return &nonce, nil
}
