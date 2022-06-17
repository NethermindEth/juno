package vmrpc

import (
	"context"
)

type storageRPCServer struct {
	UnimplementedStorageAdapterServer
}

func NewStorageRPCServer() *storageRPCServer {
	return &storageRPCServer{}
}

// GetValue calls the get_value method of the Storage adapter, 
// StorageRPCClient, on the Cairo RPC server.
func (s *storageRPCServer) GetValue(ctx context.Context, request *GetValueRequest) (*GetValueResponse, error) {
	// TODO: Handle database retrieval logic.
	return &GetValueResponse{Value: request.GetKey()}, nil
}
