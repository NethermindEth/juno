package services

import (
	"context"
	_ "embed"
	"net"
	"os"
	"os/exec"

	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/services/vmrpc"
	"github.com/NethermindEth/juno/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

//go:embed vmrpc/vm.py
var pyMain []byte

//go:embed vmrpc/vm_pb2.py
var pyPb []byte

//go:embed vmrpc/vm_pb2_grpc.py
var pyPbGRpc []byte

var VMService vmService

type vmService struct {
	service
	manager *state.Manager

	vmDir string
	vmCmd *exec.Cmd

	rpcServer      *grpc.Server
	rpcNet         string
	rpcVMAddr      string
	rpcStorageAddr string
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

	s.rpcServer = grpc.NewServer()

	// generate the py environment in the data dir
	var err error
	s.vmDir, err = os.MkdirTemp("", "vm") // TODO: we should use datadir
	if err != nil {
		s.logger.Errorf("failed to create vm dir: %v", err)
		return err
	}

	if err = os.WriteFile(s.vmDir+"/vm.py", pyMain, 0o644); err != nil {
		s.logger.Errorf("failed to write main.py: %v", err)
		return err
	}
	if err = os.WriteFile(s.vmDir+"/vm_pb2.py", pyPb, 0o644); err != nil {
		s.logger.Errorf("failed to write vm_pb2.py: %v", err)
		return err
	}
	if err = os.WriteFile(s.vmDir+"/vm_pb2_grpc.py", pyPbGRpc, 0o644); err != nil {
		s.logger.Errorf("failed to write vm_pb2_grpc.py: %v", err)
		return err
	}

	s.logger.Infof("vm dir: %s", s.vmDir)

	// start the py vm rpc server (serving vm)
	s.vmCmd = exec.Command("python", s.vmDir+"/vm.py", s.rpcVMAddr, s.rpcStorageAddr)
	pyLogger := &pySubProcessLogger{logger: s.logger}
	s.vmCmd.Stdout = pyLogger
	s.vmCmd.Stderr = pyLogger
	if err := s.vmCmd.Start(); err != nil {
		s.logger.Errorf("failed to start python vm rpc: %v", err)
		return err
	}

	// start the go vm rpc server (serving storage)
	lis, err := net.Listen(s.rpcNet, s.rpcStorageAddr)
	if err != nil {
		s.logger.Errorf("failed to listen: %v", err)
	}
	storageServer := vmrpc.NewStorageRPCServer()
	vmrpc.RegisterStorageAdapterServer(s.rpcServer, storageServer)

	// run the grpc server
	go func() {
		s.logger.Infof("grpc server listening at %v", lis.Addr())
		if err := s.rpcServer.Serve(lis); err != nil {
			s.logger.Errorf("failed to serve: %v", err)
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

	s.rpcNet = "tcp"
	s.rpcVMAddr = "localhost:8081"
	s.rpcStorageAddr = "localhost:8082"
}

func (s *vmService) Close(ctx context.Context) {
	s.service.Close(ctx)
	s.manager.Close()
	s.rpcServer.Stop()
	// TODO: we should probably wait for the process to exit
	s.vmCmd.Process.Kill()
	os.RemoveAll(s.vmDir)
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

	// TODO: right now rpcVMAddr will probably only work if using tcp
	conn, err := grpc.Dial(s.rpcVMAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		s.logger.Errorf("failed to dial: %v", err)
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
		s.logger.Errorf("failed to call: %v", err)
		return nil, err
	}

	return r.Retdata, nil
}

type pySubProcessLogger struct {
	// TODO: this should use an interface, but everywhere else it's a *zap.SugaredLogger
	logger *zap.SugaredLogger
}

func (p *pySubProcessLogger) Write(p0 []byte) (int, error) {
	p.logger.Warn("Python VM Subprocess: \n%s\n", p0)
	return len(p0), nil
}
