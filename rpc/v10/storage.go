package rpcv10

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"go.uber.org/zap"
)

// StorageAtResponseFlags represents the flags for the `starknet_getStorageAt` operation.
type StorageAtResponseFlags struct {
	IncludeLastUpdateBlock bool
}

// UnmarshalJSON implements the [json.Unmarshaler] interface for StorageAtResponseFlags.
func (f *StorageAtResponseFlags) UnmarshalJSON(data []byte) error {
	var flags []string
	if err := json.Unmarshal(data, &flags); err != nil {
		return err
	}
	*f = StorageAtResponseFlags{}

	for _, flag := range flags {
		switch flag {
		case "INCLUDE_LAST_UPDATE_BLOCK":
			f.IncludeLastUpdateBlock = true
		default:
			return fmt.Errorf("unknown flag: %s", flag)
		}
	}

	return nil
}

// StorageAtResponse represents the result of a `starknet_getStorageAt` operation.
type StorageAtResponse struct {
	// used in the marshal and unmarshal logic
	includeLastUpdateBlock bool

	Value           felt.Felt `json:"value"`
	LastUpdateBlock uint64    `json:"last_update_block"`
}

// MarshalJSON implements the [json.Marshaler] interface for StorageAtResponse.
func (st *StorageAtResponse) MarshalJSON() ([]byte, error) {
	if st.includeLastUpdateBlock {
		type storageResultAlias StorageAtResponse
		return json.Marshal((*storageResultAlias)(st))
	}

	return st.Value.MarshalJSON()
}

// UnmarshalJSON implements the [json.Unmarshaler] interface for StorageAtResponse.
func (st *StorageAtResponse) UnmarshalJSON(data []byte) error {
	type storageResultAlias StorageAtResponse
	var alias storageResultAlias

	if err := json.Unmarshal(data, &alias); err == nil {
		alias.includeLastUpdateBlock = true
		*st = StorageAtResponse(alias)
		return nil
	}

	st.includeLastUpdateBlock = false
	return st.Value.UnmarshalJSON(data)
}

// StorageAt gets the value of the storage at the given address and key.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/d6dc6ad31a1bb61c287d862ca4bdef4eb66a59a2/api/starknet_api_openrpc.json#L197
//
//nolint:lll // URL exceeds line limit but should remain intact for reference
func (h *Handler) StorageAt(
	address *felt.Address,
	key *felt.Felt,
	id *BlockID,
	flags StorageAtResponseFlags,
) (*StorageAtResponse, *jsonrpc.Error) {
	var result StorageAtResponse
	result.includeLastUpdateBlock = flags.IncludeLastUpdateBlock
	addressFelt := (*felt.Felt)(address)

	stateReader, stateCloser, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getStorageAt")

	// This checks if the contract exists because if a key doesn't exist in contract storage,
	// the returned value is always zero and error is nil.
	_, err := stateReader.ContractClassHash(addressFelt)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrContractNotFound
		}
		h.log.Error("Failed to get contract class hash", zap.Error(err))
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	result.Value, err = stateReader.ContractStorage(addressFelt, key)
	if err != nil {
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	if !flags.IncludeLastUpdateBlock {
		return &result, nil
	}

	lastUpdateBlock, err := stateReader.ContractStorageLastUpdatedBlock(address, key)
	if err != nil {
		h.log.Error(
			"Failed to find last updated block for storage key",
			zap.Error(err),
			zap.String("storage key", key.String()),
		)
		return nil, rpccore.ErrInternal.CloneWithData(err)
	}

	result.LastUpdateBlock = lastUpdateBlock

	return &result, nil
}
