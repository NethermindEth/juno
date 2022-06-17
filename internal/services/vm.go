package services

import (
	"context"
	_ "embed"
	"errors"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"

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

type pySubProcessLogger struct {
	// TODO: this should use an interface, but everywhere else it's a
	// *zap.SugaredLogger.
	logger *zap.SugaredLogger
}

func (p *pySubProcessLogger) Write(p0 []byte) (int, error) {
	p.logger.Warn("Python VM Subprocess: \n%s\n", p0)
	return len(p0), nil
}

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

var (
	//go:embed vmrpc/vm.py
	pyMain []byte
	//go:embed vmrpc/vm_pb2.py
	pyPb []byte
	//go:embed vmrpc/vm_pb2_grpc.py
	pyPbGRpc []byte
)

var errPythonNotFound = errors.New("services: Python not found in $PATH")

var VMService vmService

func (s *vmService) setDefaults() {
	if s.manager == nil {
		// notest
		codeDatabase := db.NewKeyValueDb(filepath.Join(s.vmDir, "code"), 0)
		storageDatabase := db.NewBlockSpecificDatabase(db.NewKeyValueDb(filepath.Join(s.vmDir, "storage"), 0))
		s.manager = state.NewStateManager(codeDatabase, storageDatabase)
	}

	// TODO: We probably also need to account for the idea that the
	// following ports may already be occupied and so we may need to allow
	// the user to do so from a config or generate these automatically
	// i.e. passing in an empty string or port value of ":0". This value
	// can then be retrieved using lis.Addr().(*net.TCPAddr).Port where
	// lis is the net.Listener above.
	s.rpcNet = "tcp"
	s.rpcVMAddr = "localhost:8081"
	s.rpcStorageAddr = "localhost:8082"
}

// Setup sets the service configuration, service must be not running.
func (s *vmService) Setup(codeDatabase db.Databaser, storageDatabase *db.BlockSpecificDatabase) {
	if s.Running() {
		// notest
		s.logger.Panic("trying to Setup with service running")
	}
	s.manager = state.NewStateManager(codeDatabase, storageDatabase)
}

// python returns the location of Python on the system by searching the
// $PATH environment variable and returns an error otherwise.
func python() (string, error) {
	// TODO: python may also be an incompatible Python version i.e. 2.7 so
	// account for that as well say by calling python --version once it
	// has been founding and parsing the version string using regexp.
	path, err := exec.LookPath("python")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			path, err = exec.LookPath("python3")
			if err != nil {
				return "", errPythonNotFound
			}
			return path, nil
		}
		return "", errPythonNotFound
	}
	return path, nil
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
	s.vmDir = config.DataDir

	files := [...]struct {
		name     string
		contents []byte
	}{
		{"vm.py", pyMain},
		{"vm_pb2.py", pyPb},
		{"vm_pb2_grpc.py", pyPbGRpc},
	}

	for _, f := range files {
		path := filepath.Join(s.vmDir, f.name)
		if err := os.WriteFile(path, f.contents, 0o644); err != nil {
			s.logger.Errorf("failed to write to %s: %v", path, err)
			return err
		}
	}
	s.logger.Infof("vm dir: %s", s.vmDir)

	py, err := python()
	if err != nil {
		return err
	}
	// start the py vm rpc server (serving vm)
	s.vmCmd = exec.Command(py, filepath.Join(s.vmDir, "vm.py"), s.rpcVMAddr, s.rpcStorageAddr)
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

func (s *vmService) Close(ctx context.Context) {
	s.service.Close(ctx)
	s.manager.Close()
	s.rpcServer.Stop()
	// TODO: we should probably wait for the process to exit.
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

	// TODO: How to pass in the compiled contract into the function below?
	_, err = getFullContract(contractAddr.Big(), new(big.Int))
	if err != nil {
		s.logger.Errorf("failed to retrieve compiled contract: %v", err)
		return nil, err
	}

	// Contact the server and print out its response.
	r, err := c.Call(ctx, &vmrpc.VMCallRequest{
		// XXX: Calldata has to be an array of bytes (or hex strings if you
		// will) that represent function arguments.
		Calldata:        calldata.Bytes(),
		CallerAddress:   callerAddr.Bytes(),
		ContractAddress: contractAddr.Bytes(),
		Root:            root.Bytes(),
		Selector:        selector.Bytes(),
		// XXX: Since calls are read-only, the signature does not appear to
		// be required (see vm.py as well).
		Signature: signature.Bytes(),
	})
	if err != nil {
		s.logger.Errorf("failed to call: %v", err)
		return nil, err
	}

	return r.Retdata, nil
}
