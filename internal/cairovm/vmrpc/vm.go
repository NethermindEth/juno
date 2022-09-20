package vmrpc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/internal/utils"

	"github.com/NethermindEth/juno/internal/db/class"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/trie"

	statedb "github.com/NethermindEth/juno/internal/db/state"
)

type storageRPCServer struct {
	stateManager *statedb.Manager
	classManager class.ClassManager
	UnimplementedStorageAdapterServer
}

func NewStorageRPCServer(stateManager *statedb.Manager, classManager class.ClassManager) *storageRPCServer {
	return &storageRPCServer{
		stateManager: stateManager,
		classManager: classManager,
	}
}

func (s *storageRPCServer) GetPatriciaNode(ctx context.Context, request *GetValueRequest) (*VMTrieNode, error) {
	node, err := s.stateManager.GetTrieNode(new(felt.Felt).SetBytes(request.GetKey()))
	if err != nil {
		return nil, err
	}
	nodeP := &VMTrieNode{
		Len:    uint32(node.Path().Len()),
		Path:   node.Path().Bytes(),
		Bottom: node.Bottom().ByteSlice(),
	}
	switch n := node.(type) {
	case *trie.EdgeNode:
		nodeP.Type = NodeType_EDGE_NODE
	case *trie.BinaryNode:
		nodeP.Type = NodeType_BINARY_NODE
		nodeP.Left = n.LeftH.ByteSlice()
		nodeP.Right = n.RightH.ByteSlice()
	default:
		return nil, errors.New("unsupported trie node type")
	}
	return nodeP, nil
}

func (s *storageRPCServer) GetContractState(ctx context.Context, request *GetValueRequest) (*VMContractState, error) {
	st, err := s.stateManager.GetContractState(new(felt.Felt).SetBytes(request.GetKey()))
	if err != nil {
		return nil, err
	}
	return &VMContractState{
		ContractHash: st.ContractHash.ByteSlice(),
		StorageRoot:  st.StorageRoot.ByteSlice(),
		Height:       uint32(state.StorageTrieHeight),
	}, nil
}

func (s *storageRPCServer) GetContractDefinition(ctx context.Context, request *GetValueRequest) (*VMContractDefinition, error) {
	c, err := s.classManager.GetClass(new(felt.Felt).SetBytes(request.GetKey()))
	if err != nil {
		return nil, err
	}
	decompressedProgram, err := utils.DecompressGzipB64(c.Program)
	if err != nil {
		return nil, err
	}
	decompressedAbi, err := utils.DecompressGzipB64(c.Abi)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(map[string]interface{}{
		"program":              json.RawMessage(decompressedProgram),
		"abi":                  json.RawMessage(decompressedAbi),
		"entry_points_by_type": c.EntryPointsByType,
	})
	if err != nil {
		return nil, err
	}
	return &VMContractDefinition{
		Value: data,
	}, nil
}
