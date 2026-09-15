package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type clock struct {
	current  atomic.Uint64
	from     uint64
	to       uint64
	interval time.Duration
	speed    float64
	blocks   []blockInfo
	logger   *log.ZapLogger
}

func newClock(blocks []blockInfo, config *config, logger *log.ZapLogger) *clock {
	clock := &clock{
		from:     config.from,
		to:       config.to,
		interval: config.interval,
		speed:    config.speed,
		blocks:   blocks,
		logger:   logger,
	}
	clock.current.Store(config.tip)
	return clock
}

func (clock *clock) tip() uint64 {
	return clock.current.Load()
}

func (clock *clock) run(ctx context.Context) {
	if clock.speed > 0 {
		clock.logger.Info("replaying captured block times", zap.Float64("speed", clock.speed))
	} else {
		clock.logger.Info("advancing tip on a fixed interval", zap.Duration("interval", clock.interval))
	}

	for clock.tip() < clock.to {
		select {
		case <-ctx.Done():
			return
		case <-time.After(clock.nextDelay()):
			clock.logger.Info("tip advanced", zap.Uint64("tip", clock.current.Add(1)))
		}
	}

	clock.logger.Info("tip reached --to; everything is served", zap.Uint64("tip", clock.to))
}

func (clock *clock) nextDelay() time.Duration {
	if clock.speed == 0 {
		return clock.interval
	}
	tip := clock.tip()
	current := clock.blocks[tip-clock.from].timestamp
	next := clock.blocks[tip+1-clock.from].timestamp

	if next <= current {
		return 0
	}
	return time.Duration(float64(next-current) * float64(time.Second) / clock.speed)
}
