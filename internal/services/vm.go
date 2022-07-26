package services

import (
	"context"
	_ "embed"
	"errors"
	"github.com/NethermindEth/juno/internal/log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/NethermindEth/juno/internal/config"
	statedb "github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/services/vmrpc"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type pySubProcessLogger struct {
	// TODO: this should use an interface, but everywhere else it's a
	// *zap.SugaredLogger.
	logger *zap.SugaredLogger
}

func (p *pySubProcessLogger) Write(p0 []byte) (int, error) {
	p.logger.Warnf("Python VM Subprocess: \n%s\n", p0)
	return len(p0), nil
}

type VirtualMachine struct {
	manager *statedb.Manager

	logger *zap.SugaredLogger

	vmDir string
	vmCmd *exec.Cmd

	rpcServer      *grpc.Server
	rpcNet         string
	rpcVMAddr      string
	rpcStorageAddr string
}

var (
	//go:embed vmrpc/vm.py
	pyMain []byte
	//go:embed vmrpc/vm_pb2.py
	pyPb []byte
	//go:embed vmrpc/vm_pb2_grpc.py
	pyPbGRpc []byte
)

var errPythonNotFound = errors.New("services: Python not found in $PATH")

func NewVM(stateManager *statedb.Manager) *VirtualMachine {
	vm := VirtualMachine{}
	vm.rpcNet = "tcp"
	vm.manager = stateManager
	ports, err := freePorts(2)
	if err != nil {
		return nil
	}
	vm.logger = log.Logger.Named("VM")
	vm.rpcVMAddr = "localhost:" + strconv.Itoa(ports[0])
	vm.rpcStorageAddr = "localhost:" + strconv.Itoa(ports[1])
	return &vm
}

// freePorts returns a slice of length n of free TCP ports on localhost.
// It assumes that those ports will not have been reassigned by the
// time this function exits or the return value is used.
func freePorts(n int) ([]int, error) {
	ports := make([]int, 0, n)
	for i := 0; i < n; i++ {
		addr, err := net.ResolveTCPAddr("tcp", ":0")
		if err != nil {
			return nil, err
		}
		listener, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return nil, err
		}
		defer listener.Close()
		ports = append(ports, listener.Addr().(*net.TCPAddr).Port)
	}
	return ports, nil
}

func (s *VirtualMachine) Run() error {
	s.rpcServer = grpc.NewServer()

	// Generate the Python environment in the data dir.
	s.vmDir = config.DataDir

	files := [...]struct {
		name     string
		contents []byte
	}{
		{"vm.py", pyMain},
		{"vm_pb2.py", pyPb},
		{"vm_pb2_grpc.py", pyPbGRpc},
	}

	for _, file := range files {
		path := filepath.Join(s.vmDir, file.name)
		if err := os.WriteFile(path, file.contents, 0o644); err != nil {
			//s.logger.Errorf("failed to write to %s: %v", path, err)
			return err
		}
	}
	//s.logger.Infof("vm dir: %s", s.vmDir)

	// Start the cairo-lang gRPC server (serving contract calls).
	s.vmCmd = exec.Command("python3", filepath.Join(s.vmDir, "vm.py"), s.rpcVMAddr, s.rpcStorageAddr)
	pyLogger := &pySubProcessLogger{logger: s.logger}
	s.vmCmd.Stdout = pyLogger
	s.vmCmd.Stderr = pyLogger
	if err := s.vmCmd.Start(); err != nil {
		s.logger.Errorf("failed to start python vm rpc: %v", err)
		return err
	}

	// Start the Go gRPC server (serving storage).
	lis, err := net.Listen(s.rpcNet, s.rpcStorageAddr)
	if err != nil {
		s.logger.Errorf("failed to listen: %v", err)
	}
	storageServer := vmrpc.NewStorageRPCServer(s.manager)
	vmrpc.RegisterStorageAdapterServer(s.rpcServer, storageServer)

	// Run the gRPC server.
	go func() {
		s.logger.Infof("gRPC server listening at %v", lis.Addr())
		if err := s.rpcServer.Serve(lis); err != nil {
			s.logger.Errorf("failed to serve: %v", err)
		}
	}()

	return nil
}

func (s *VirtualMachine) Close(ctx context.Context) {
	s.rpcServer.Stop()
	// TODO: we should probably wait for the process to exit.
	s.vmCmd.Process.Kill()
	os.RemoveAll(s.vmDir)
}

func (s *VirtualMachine) Call(
	ctx context.Context,
	state state.State,
	calldata []*felt.Felt,
	callerAddr,
	contractAddr,
	selector,
	sequencer *felt.Felt,
) ([]*felt.Felt, error) {

	s.logger.Info("Call")

	// XXX: Right now rpcVMAddr will probably only work if using TCP.
	conn, err := grpc.Dial(s.rpcVMAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		s.logger.Errorf("failed to dial: %v", err)
		return nil, err
	}
	defer conn.Close()
	c := vmrpc.NewVMClient(conn)

	contractState, err := state.GetContractState(contractAddr)
	if err != nil {
		return nil, err
	}

	calldataBytes := make([][]byte, len(calldata))
	for i, felt := range calldata {
		calldataBytes[i] = felt.ByteSlice()
	}

	// Contact the server and print out its response.
	r, err := c.Call(ctx, &vmrpc.VMCallRequest{
		Calldata:        calldataBytes,
		CallerAddress:   callerAddr.ByteSlice(),
		ContractAddress: contractAddr.ByteSlice(),
		ClassHash:       contractState.ContractHash.ByteSlice(),
		Root:            state.Root().ByteSlice(),
		Selector:        selector.ByteSlice(),
		Sequencer:       sequencer.ByteSlice(),
	})
	if err != nil {
		s.logger.Errorf("failed to call: %v", err)
		return nil, err
	}

	retdataFelts := make([]*felt.Felt, len(r.Retdata))
	for i, ret := range r.Retdata {
		retdataFelts[i] = new(felt.Felt).SetBytes(ret)
	}

	return retdataFelts, nil
}
