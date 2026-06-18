package compiler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// Compiler compiles Sierra classes to CASM.
type Compiler interface {
	Compile(ctx context.Context, sierra *starknet.SierraClass) (
		*starknet.CasmClass, error,
	)
}

// Flag names used to pass per-compilation resource limits to the
// compile-sierra child process. They are shared by the parent (which
// builds the child's argv) and the child command (which defines the
// flags and applies the limits to itself).
const (
	FlagMaxMemory  = "max-memory"   // bytes (RLIMIT_AS)
	FlagMaxCPUTime = "max-cpu-time" // seconds (RLIMIT_CPU)
)

// Config bounds the resources used by compilation child processes.
type Config struct {
	// MaxConcurrent is the maximum number of compilation processes
	// running at once.
	MaxConcurrent uint
	// MaxMemory is the address-space limit (RLIMIT_AS) in bytes
	// applied to each compilation process. Exceeding it aborts that
	// compilation. Enforced on Linux only. 0 disables the limit.
	MaxMemory uint64
	// MaxCPUTime is the CPU-time limit (RLIMIT_CPU) in seconds
	// applied to each compilation process. Exceeding it kills that
	// compilation. Enforced on Linux only. 0 disables the limit.
	MaxCPUTime uint64
}

// compiler compiles Sierra to CASM in a safe way by spawning
// a separate process.
type compiler struct {
	binaryPath string
	maxMemory  uint64
	maxCPUTime uint64
	sem        chan struct{}
	logger     log.StructuredLogger
}

// New creates a Compiler that runs Sierra-to-CASM compilation
// in isolated child processes with concurrency and resource
// control. The caller's context controls the compilation deadline.
func New(cfg *Config, binaryPath string, logger log.StructuredLogger) Compiler {
	if binaryPath == "" {
		var err error
		binaryPath, err = os.Executable()
		if err != nil {
			binaryPath = os.Args[0]
		}
	}
	if (cfg.MaxMemory > 0 || cfg.MaxCPUTime > 0) && !rlimitsSupported {
		logger.Warn(
			"Compilation CPU and memory limits are only enforced on Linux and will not be applied",
			zap.String("os", runtime.GOOS),
		)
	}
	return &compiler{
		binaryPath: binaryPath,
		maxMemory:  cfg.MaxMemory,
		maxCPUTime: cfg.MaxCPUTime,
		sem:        make(chan struct{}, cfg.MaxConcurrent),
		logger:     logger,
	}
}

// Compile runs Sierra-to-CASM compilation in an isolated child
// process. The child process is killed if the context is cancelled.
func (c *compiler) Compile(
	ctx context.Context, sierra *starknet.SierraClass,
) (*starknet.CasmClass, error) {
	c.logger.Debug("Compilation request received")

	sierraJSON, err := json.Marshal(starknet.SierraClass{
		EntryPoints: sierra.EntryPoints,
		Program:     sierra.Program,
		Version:     sierra.Version,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal sierra class: %w", err)
	}

	// Acquire semaphore slot for concurrency limiting.
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, fmt.Errorf(
			"waiting for compilation slot: %w", ctx.Err(),
		)
	}

	// The child applies these limits to itself before compiling, so they
	// are in force for the whole compile with no race window.
	args := []string{"compile-sierra"}
	if c.maxMemory > 0 {
		args = append(args, "--"+FlagMaxMemory, strconv.FormatUint(c.maxMemory, 10))
	}
	if c.maxCPUTime > 0 {
		args = append(args, "--"+FlagMaxCPUTime, strconv.FormatUint(c.maxCPUTime, 10))
	}

	//nolint:gosec // binaryPath is the juno binary, not user input
	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	cmd.Stdin = bytes.NewReader(sierraJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			c.logger.Warn("Sierra to CASM compilation timed out",
				zap.Error(ctxErr),
			)
			return nil, fmt.Errorf(
				"failed to compile Sierra to CASM: %w",
				ctxErr,
			)
		}
		return nil, fmt.Errorf(
			"failed to compile Sierra to CASM: %w. stderr: %s", err, stderr.String(),
		)
	}

	var casmClass starknet.CasmClass
	if err := json.Unmarshal(stdout.Bytes(), &casmClass); err != nil {
		return nil, fmt.Errorf("couldn't unmarshall casm class: %w", err)
	}

	return &casmClass, nil
}

type inProcessCompiler struct{}

// NewUnsafe returns a Compiler that compiles in the same process Juno is running.
// It can be unsafe if the compilation process get stuck.
func NewUnsafe() Compiler {
	return &inProcessCompiler{}
}

// Compile runs Sierra-to-CASM compilation as a process thread
// If there were to be a bug in the compilation process, such as an infinite loop
// it opens a vector for a DoS attack.
func (r *inProcessCompiler) Compile(
	_ context.Context, sierra *starknet.SierraClass,
) (*starknet.CasmClass, error) {
	return CompileFFI(sierra)
}
