package node

import (
	"context"
	"runtime"

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

// resolveThrottledCompilerSettings turns the config's compilation limits into concrete values.
// An absent flag (not Explicit) is derived from the available hardware.
// Any explicit value (including 0) is used as-is.
func resolveThrottledCompilerSettings(
	cfg *Config, logger *log.ZapLogger,
) (uint64, uint64) {
	maxConcurrency := cfg.MaxConcurrentCompilations
	availableMemoryMB := compiler.AvailableMemoryMB()
	if !cfg.MaxConcurrentCompilationsExplicit {
		maxConcurrency = compiler.ConcurrencyLimit(
			uint64(runtime.GOMAXPROCS(0)),
			availableMemoryMB,
			uint64(cfg.NodeMemoryReserve),
			uint64(cfg.MaxCompilationMemory),
		)
	}

	queueSize := cfg.MaxCompilationQueue
	if !cfg.MaxCompilationQueueExplicit {
		queueSize = 2 * maxConcurrency
	}

	logger.Info("Sierra compilation limits",
		zap.Uint64("maxConcurrency", maxConcurrency),
		zap.Uint64("queueSize", queueSize),
		zap.Uint64("availableMemoryMB", availableMemoryMB),
		zap.Uint("nodeMemoryReserveMB", cfg.NodeMemoryReserve),
		zap.Uint("maxCompilationMemoryMB", cfg.MaxCompilationMemory),
	)

	return maxConcurrency, queueSize
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
) *ThrottledCompiler {
	maxConcurrency, queueSize := resolveThrottledCompilerSettings(cfg, logger)

	return NewThrottledCompiler(
		compiler.New(
			&compiler.Config{
				MaxMemory:  uint64(cfg.MaxCompilationMemory) * 1024 * 1024,
				MaxCPUTime: uint64(cfg.MaxCompilationCPUTime),
			},
			"",
			logger,
		),
		uint(maxConcurrency),
		queueSize,
	)
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
