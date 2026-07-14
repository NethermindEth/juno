package node

import (
	"context"
	"fmt"
	"runtime"
	"strconv"

	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/utils/throttler"
	"go.uber.org/zap"
)

var _ compiler.Compiler = (*ThrottledCompiler)(nil)

type ThrottledCompiler struct {
	*throttler.Throttler[compiler.Compiler]
}

type ThrottledCompilerSettings struct {
	maxConcurrency uint64
	queueSize      uint64
}

// parseCompilerLimit reads a compilation-sizing flag.
// An empty value derives the value at startup (derived is true).
func parseCompilerLimit(value string) (limit uint64, derived bool, err error) {
	if value == "" {
		return 0, true, nil
	}
	valueUint64, err := strconv.ParseUint(value, 10, strconv.IntSize)
	if err != nil {
		return 0, false, fmt.Errorf("%q is not an unsigned integer: %w", value, err)
	}
	return valueUint64, false, nil
}

func resolveThrottledCompilerSettings(
	cfg *Config, logger *log.ZapLogger,
) (*ThrottledCompilerSettings, error) {
	maxConcurrency, maxConcurrencyDerived, err := parseCompilerLimit(cfg.MaxConcurrentCompilations)
	if err != nil {
		return nil, fmt.Errorf("parsing max-concurrent-compilations: %w", err)
	}

	if maxConcurrencyDerived {
		maxConcurrency = compiler.ConcurrencyLimit(
			uint64(runtime.GOMAXPROCS(0)),
			compiler.AvailableMemoryMB(),
			uint64(cfg.NodeMemoryReserve),
			uint64(cfg.MaxCompilationMemory),
		)
	}

	queueSize, queueSizeDerived, err := parseCompilerLimit(cfg.MaxCompilationQueue)
	if err != nil {
		return nil, fmt.Errorf("parsing max-compilation-queue: %w", err)
	}

	if queueSizeDerived {
		queueSize = 2 * maxConcurrency
	}

	logger.Info("Sierra compilation limits",
		zap.Uint64("concurrency", maxConcurrency),
		zap.Bool("concurrencyDerived", maxConcurrencyDerived),
		zap.Uint64("queueSize", queueSize),
		zap.Bool("queueSizeDerived", queueSizeDerived),
	)

	return &ThrottledCompilerSettings{
		maxConcurrency, queueSize,
	}, nil
}

func NewThrottledCompiler(
	res compiler.Compiler, concurrencyBudget uint, maxQueueLen uint64,
) *ThrottledCompiler {
	return &ThrottledCompiler{
		Throttler: throttler.NewThrottler(
			concurrencyBudget, &res, throttler.WithMaxQueueLen(maxQueueLen),
		),
	}
}

func newThrottledCompilerFromConfig(
	cfg *Config, logger *log.ZapLogger,
) (*ThrottledCompiler, error) {
	throttledCompilerSettings, err := resolveThrottledCompilerSettings(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("resolving compiler settings: %w", err)
	}

	return NewThrottledCompiler(
		compiler.New(
			&compiler.Config{
				MaxMemory:  uint64(cfg.MaxCompilationMemory) * 1024 * 1024,
				MaxCPUTime: uint64(cfg.MaxCompilationCPUTime),
			},
			"",
			logger,
		),
		uint(throttledCompilerSettings.maxConcurrency),
		throttledCompilerSettings.queueSize,
	), nil
}

func (tc *ThrottledCompiler) Compile(
	ctx context.Context, sierra *starknet.SierraClass,
) (*starknet.CasmClass, error) {
	var result *starknet.CasmClass
	err := tc.Do(ctx, func(c *compiler.Compiler) error {
		var cErr error
		result, cErr = (*c).Compile(ctx, sierra)
		return cErr
	})
	return result, err
}
