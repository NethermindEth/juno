package vmrpc

import (
	"context"
)

type StorageRPCServer struct {
	UnimplementedStorageAdapterServer
}

func (s *StorageRPCServer) GetValue(ctx context.Context, request *GetValueRequest) (*GetValueResponse, error) {
	return &GetValueResponse{Value: request.GetKey()}, nil
}
