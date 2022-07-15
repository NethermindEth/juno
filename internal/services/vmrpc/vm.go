package vmrpc

import (
	"context"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"

	statedb "github.com/NethermindEth/juno/internal/db/state"
)

type storageRPCServer struct {
	stateManager *statedb.Manager
	UnimplementedStorageAdapterServer
}

func NewStorageRPCServer(stateManager *statedb.Manager) *storageRPCServer {
	return &storageRPCServer{
		stateManager: stateManager,
	}
}

func (s *storageRPCServer) GetPatriciaNode(ctx context.Context, request *GetValueRequest) (*VMTrieNode, error) {
	node, err := s.stateManager.GetTrieNode(new(felt.Felt).SetBytes(request.GetKey()))
	if err != nil {
		return nil, err
	}
	return &VMTrieNode{
		Len:    uint32(node.Path().Len()),
		Path:   node.Path().Bytes(),
		Bottom: node.Bottom().ByteSlice(),
	}, nil
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
	cd, err := s.stateManager.GetContract(new(felt.Felt).SetBytes(request.GetKey()))
	if err != nil {
		return nil, err
	}
	return &VMContractDefinition{
		Value: cd.FullDef,
	}, nil
}
