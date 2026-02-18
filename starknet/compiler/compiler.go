package compiler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

// Compiler compiles Sierra classes to CASM.
type Compiler interface {
	Compile(ctx context.Context, sierra *starknet.SierraClass) (
		*starknet.CasmClass, error,
	)
}

// compiler compiles Sierra to CASM in a safe way by spawning
// a separate process.
type compiler struct {
	binaryPath string
	sem        chan struct{}
	log        utils.StructuredLogger
}

// New creates a Compiler that runs Sierra-to-CASM compilation
// in isolated child processes with concurrency control.
// The caller's context controls the compilation deadline.
func New(
	maxConcurrent uint,
	binaryPath string,
	log utils.StructuredLogger,
) Compiler {
	if binaryPath == "" {
		var err error
		binaryPath, err = os.Executable()
		if err != nil {
			binaryPath = os.Args[0]
		}
	}
	return &compiler{
		binaryPath: binaryPath,
		sem:        make(chan struct{}, maxConcurrent),
		log:        log,
	}
}

// Compile runs Sierra-to-CASM compilation in an isolated child
// process. The child process is killed if the context is cancelled.
func (c *compiler) Compile(
	ctx context.Context, sierra *starknet.SierraClass,
) (*starknet.CasmClass, error) {
	c.log.Debug("Compilation request received")

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

	//nolint:gosec // binaryPath is the juno binary, not user input
	cmd := exec.CommandContext(ctx, c.binaryPath, "compile-sierra")
	cmd.Stdin = bytes.NewReader(sierraJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			c.log.Warn("Sierra to CASM compilation timed out",
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
