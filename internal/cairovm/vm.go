package cairovm

import (
	"context"
	_ "embed"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"

	. "github.com/NethermindEth/juno/internal/log"

	vmrpc2 "github.com/NethermindEth/juno/internal/cairovm/vmrpc"

	statedb "github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type subprocessLogger struct {
	logger *zap.SugaredLogger
}

func (sl *subprocessLogger) Write(trace []byte) (int, error) {
	// notest
	sl.logger.Warnf("cairovm: python subprocess: \n%s\n", trace)
	return len(trace), nil
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

func New(stateManager *statedb.Manager, logger *zap.SugaredLogger) *VirtualMachine {
	ports, err := freePorts(2)
	if err != nil {
		// notest
		return nil
	}

	return &VirtualMachine{
		rpcNet:         "tcp",
		manager:        stateManager,
		rpcVMAddr:      "localhost:" + strconv.Itoa(ports[0]),
		rpcStorageAddr: "localhost:" + strconv.Itoa(ports[1]),
		// TODO: Use injected logger.
		logger: Logger,
	}
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

func (s *VirtualMachine) Run(dataDir string) error {
	s.rpcServer = grpc.NewServer()

	// Generate the Python environment in the data dir.
	s.vmDir = dataDir

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
			return err
		}
	}

	// Start the cairo-lang gRPC server (serving contract calls).
	s.vmCmd = exec.Command("python3", filepath.Join(s.vmDir, "vm.py"), s.rpcVMAddr, s.rpcStorageAddr)

	pyLogger := &subprocessLogger{logger: s.logger}
	s.vmCmd.Stdout = pyLogger
	s.vmCmd.Stderr = pyLogger

	if err := s.vmCmd.Start(); err != nil {
		s.logger.Error("cairovm: start virtual machine server: " + err.Error())
		return err
	}

	// Start the storage server.
	lis, err := net.Listen(s.rpcNet, s.rpcStorageAddr)
	if err != nil {
		s.logger.Error("cairovm: storage server: " + err.Error())
		// TODO: Execution was allowed to continue here. I am not sure that
		// was the right behaviour.
		return err
	}

	storageServer := vmrpc2.NewStorageRPCServer(s.manager)
	vmrpc2.RegisterStorageAdapterServer(s.rpcServer, storageServer)

	// Run the gRPC server.
	go func() {
		s.logger.Infow("Starting storage server at", "address", lis.Addr().String())
		if err := s.rpcServer.Serve(lis); err != nil {
			// notest
			s.logger.Error("cairovm: start storage server: " + err.Error())
		}
	}()

	return nil
}

func (s *VirtualMachine) Close() {
	s.rpcServer.Stop()
	s.vmCmd.Process.Kill()

	// Clean up.
	os.RemoveAll(path.Join(s.vmDir, "__pycache__"))
	os.Remove(path.Join(s.vmDir, "vm.py"))
	os.Remove(path.Join(s.vmDir, "vm_pb2.py"))
	os.Remove(path.Join(s.vmDir, "vm_pb2_grpc.py"))
}

func (s *VirtualMachine) Call(
	ctx context.Context,
	// TODO: Can the sequencer address be omitted?
	state state.State,
	sequencer,
	// Function call.
	contractAddr,
	entryPointSelector *felt.Felt,
	calldata []*felt.Felt,
) ([]*felt.Felt, error) {
	// TODO: Log arguments?
	s.logger.Info("Executing contract call")

	contractState, err := state.GetContractState(contractAddr)
	if err != nil {
		return nil, err
	}

	calldataBytes := make([][]byte, 0, len(calldata))
	for _, d := range calldata {
		calldataBytes = append(calldataBytes, d.ByteSlice())
	}

	conn, err := grpc.Dial(s.rpcVMAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		s.logger.Error("cairovm: grpc dial: " + err.Error())
		return nil, err
	}
	defer conn.Close()

	client := vmrpc2.NewVMClient(conn)
	response, err := client.Call(ctx, &vmrpc2.VMCallRequest{
		Calldata:        calldataBytes,
		ContractAddress: contractAddr.ByteSlice(),
		ClassHash:       contractState.ContractHash.ByteSlice(),
		Root:            state.Root().ByteSlice(),
		Selector:        entryPointSelector.ByteSlice(),
		Sequencer:       sequencer.ByteSlice(),
	})
	if err != nil {
		s.logger.Errorf("cairovm: call: " + err.Error())
		return nil, err
	}

	returned := make([]*felt.Felt, 0, len(response.Retdata))
	for _, data := range response.Retdata {
		returned = append(returned, new(felt.Felt).SetBytes(data))
	}

	return returned, nil
}
