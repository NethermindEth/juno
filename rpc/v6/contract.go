package rpcv6

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
)

/****************************************************
		Contract Handlers
*****************************************************/

// Nonce returns the nonce associated with the given address in the given block number
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L633
func (h *Handler) Nonce(id BlockID, address felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getNonce")

	nonce, err := stateReader.ContractNonce(&address)
	if err != nil {
		return nil, rpccore.ErrContractNotFound
	}

	return nonce, nil
}

// StorageAt gets the value of the storage at the given address and key for a given block.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L110
func (h *Handler) StorageAt(address, key felt.Felt, id BlockID) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getStorageAt")

	// Check if the contract exists because if it doesn't, the call to `ContractStorage` below
	// will just return a zero-valued felt with a nil error (not an `ErrContractNotFound` error)
	_, err := stateReader.ContractClassHash(&address)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrContractNotFound
		}
		h.log.Errorw("Failed to get contract class hash", "err", err)
		return nil, rpccore.ErrInternal
	}

	// Check if key exists in existing contract
	// If it doesn't, it will return a zero-valued felt with a nil error
	value, err := stateReader.ContractStorage(&address, &key)
	if err != nil {
		return nil, rpccore.ErrInternal
	}

	return value, nil
}
