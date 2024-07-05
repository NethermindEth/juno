package rpc

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/jsonrpc"
)

func (h *Handler) NodesFromRoot(key *felt.Felt) ([]trie.StorageNode, *jsonrpc.Error) {
	stateReader, _, err := h.bcReader.HeadState()
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	trieInstance, _, err := stateReader.ClassesTrie()
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	adaptedKey := trieInstance.FeltToKey(key)
	storageNodes, err := trieInstance.NodesFromRoot(&adaptedKey)
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	return storageNodes, nil
}
