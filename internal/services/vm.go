package services

import (
	"context"
	"fmt"
	"net"
	"os/exec"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/services/vmrpc"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var VMService vmService

type vmService struct {
	service
	manager *state.Manager
}

// Setup sets the service configuration, service must be not running.
func (s *vmService) Setup(codeDatabase db.Databaser, storageDatabase *db.BlockSpecificDatabase) {
	if s.Running() {
		// notest
		s.logger.Panic("trying to Setup with service running")
	}
	s.manager = state.NewStateManager(codeDatabase, storageDatabase)
}

func (s *vmService) Run() error {
	if s.logger == nil {
		s.logger = log.Default.Named("VMService")
	}

	if err := s.service.Run(); err != nil {
		// notest
		return err
	}

	s.setDefaults()

	// start the go vm rpc server (serving storage)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8082))
	if err != nil {
		s.logger.Fatalf("failed to listen: %v", err)
	}
	server := grpc.NewServer()
	vmrpc.RegisterStorageAdapterServer(server, &vmrpc.StorageRPCServer{})
	go func() {
		s.logger.Infof("server listening at %v", lis.Addr())
		if err := server.Serve(lis); err != nil {
			s.logger.Fatalf("failed to serve: %v", err)
		}
	}()

	// start the py vm rpc server (serving vm)
	cmd := exec.Command("python", "./vmrpc/vm.py", "localhost:8081", lis.Addr().String())
	go func() {
		s.logger.Infof("server listening at %v", "localhost:8081")
		if err := cmd.Run(); err != nil {
			s.logger.Fatalf("failed to serve: %v\n%v", err)
		}
	}()

	return nil
}

func (s *vmService) setDefaults() {
	if s.manager == nil {
		// notest
		codeDatabase := db.NewKeyValueDb(config.DataDir+"/code", 0)
		storageDatabase := db.NewBlockSpecificDatabase(db.NewKeyValueDb(config.DataDir+"/storage", 0))
		s.manager = state.NewStateManager(codeDatabase, storageDatabase)
	}
}

func (s *vmService) Close(ctx context.Context) {
	s.service.Close(ctx)
	s.manager.Close()
}

func (s *vmService) Call(
	ctx context.Context,
	root types.Felt,
	contractAddr types.Felt,
	selector types.Felt,
	calldata types.Felt,
	callerAddr types.Felt,
	signature types.Felt,
) ([][]byte, error) {
	s.AddProcess()
	defer s.DoneProcess()

	conn, err := grpc.Dial("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		s.logger.Errorf("failed to dial: %v", err)
		// notest
		return nil, err
	}
	defer conn.Close()
	c := vmrpc.NewVMClient(conn)

	// Contact the server and print out its response.
	r, err := c.Call(ctx, &vmrpc.VMCallRequest{
		Root:            root.Bytes(),
		ContractAddress: contractAddr.Bytes(),
		Selector:        selector.Bytes(),
		Calldata:        calldata.Bytes(),
		CallerAddress:   callerAddr.Bytes(),
		Signature:       signature.Bytes(),
	})
	if err != nil {
		s.logger.Errorf("failed to call: %v", err.Error())
		return nil, err
	}

	return r.Retdata, nil
}
