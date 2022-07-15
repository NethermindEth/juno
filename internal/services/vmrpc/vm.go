package vmrpc

import (
	"context"

	"github.com/NethermindEth/juno/pkg/state"

	"github.com/NethermindEth/juno/pkg/types"

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
	f := types.BytesToFelt(request.GetKey())
	node, err := s.stateManager.GetTrieNode(&f)
	if err != nil {
		return nil, err
	}
	return &VMTrieNode{
		Len:    uint32(node.Path().Len()),
		Path:   node.Path().Bytes(),
		Bottom: node.Bottom().Bytes(),
	}, nil
}

func (s *storageRPCServer) GetContractState(ctx context.Context, request *GetValueRequest) (*VMContractState, error) {
	f := types.BytesToFelt(request.GetKey())
	st, err := s.stateManager.GetContractState(&f)
	if err != nil {
		return nil, err
	}
	return &VMContractState{
		ContractHash: st.ContractHash.Bytes(),
		StorageRoot:  st.StorageRoot.Bytes(),
		Height:       uint32(state.StorageTrieHeight),
	}, nil
}

func (s *storageRPCServer) GetContractDefinition(ctx context.Context, request *GetValueRequest) (*VMContractDefinition, error) {
	f := types.BytesToFelt(request.GetKey())
	cd, err := s.stateManager.GetContract(&f)
	if err != nil {
		return nil, err
	}
	return &VMContractDefinition{
		Value: cd.FullDef,
	}, nil
}
