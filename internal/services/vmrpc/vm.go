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

func (s *storageRPCServer) GetValue(ctx context.Context, request *GetValueRequest) (*GetValueResponse, error) {
	return &GetValueResponse{Value: request.GetKey()}, nil
}
