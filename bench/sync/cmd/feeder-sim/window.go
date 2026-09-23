package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type window struct {
	store    *store
	config   *config
	logger   *log.ZapLogger
	noChange []byte
	current  atomic.Pointer[snapshot]
}

func newWindow(
	store *store,
	config *config,
	logger *log.ZapLogger,
	start advance,
) (*window, error) {
	noChange, err := gzipBytes([]byte(`{"changed":false}`))
	if err != nil {
		return nil, err
	}

	window := &window{store: store, config: config, logger: logger, noChange: noChange}
	initial := newSnapshot(start)
	lo, hi := initial.bounds(config)
	for number := lo; number < hi; number++ {
		block, err := window.prepare(number)
		if err != nil {
			return nil, err
		}

		initial.blocks[number] = block
	}

	initial.blocks[hi], initial.latest, err = window.prepareLatest(hi)
	if err != nil {
		return nil, err
	}

	window.current.Store(initial)
	return window, nil
}

func (window *window) run(ctx context.Context, advances <-chan advance) {
	block, latest, ok := window.speculate()
	for {
		select {
		case <-ctx.Done():
			return
		case advance := <-advances:
			window.publish(advance, block, latest, ok)
			block, latest, ok = window.speculate()
		}
	}
}

func (window *window) speculate() (block prepared, latest [][]byte, ok bool) {
	snapshot := window.current.Load()
	_, hi := snapshot.bounds(window.config)
	entering := hi + 1
	if entering > window.config.to {
		return prepared{}, nil, false
	}

	started := time.Now()
	block, latest, err := window.prepareLatest(entering)
	if err != nil {
		window.logger.Error(
			"preparing pre-confirmed block",
			zap.Uint64("block", entering),
			zap.Error(err),
		)
		return prepared{}, nil, false
	}

	took := time.Since(started)
	window.logger.Debug(
		"prepared pre-confirmed block",
		zap.Uint64("block", entering),
		zap.Duration("took", took),
	)
	if snapshot.interval > 0 && took > snapshot.interval {
		window.logger.Warn(
			"pre-confirmed preparation slower than the interval",
			zap.Uint64("block", entering),
			zap.Duration("took", took),
			zap.Duration("interval", snapshot.interval),
		)
	}

	return block, latest, true
}

func (window *window) publish(advance advance, block prepared, latest [][]byte, ok bool) {
	current := window.current.Load()
	next := newSnapshot(advance)
	lo, hi := next.bounds(window.config)
	for number, block := range current.blocks {
		if number >= lo && number <= hi {
			next.blocks[number] = block
		}
	}

	if ok {
		next.blocks[hi] = block
		next.latest = latest
	} else {
		next.latest = current.latest
	}

	window.current.Store(next)
}

func (window *window) prepare(number uint64) (prepared, error) {
	resource, err := preConfirmedBlock.resource(blockKey{BlockNumber: number})
	if err != nil {
		return prepared{}, err
	}

	stored, ok := window.store.get(resource.file)
	if !ok {
		return prepared{}, fmt.Errorf("%s: not in dataset", resource.file)
	}

	round, err := decodeRound(stored)
	if err != nil {
		return prepared{}, fmt.Errorf("%s: %w", resource.file, err)
	}

	return prepared{stored: stored, round: round}, nil
}

func (window *window) prepareLatest(number uint64) (prepared, [][]byte, error) {
	block, err := window.prepare(number)
	if err != nil {
		return prepared{}, nil, err
	}

	latest, err := window.latestBodies(block.round, number)
	if err != nil {
		return prepared{}, nil, err
	}

	return block, latest, nil
}

func (window *window) latestBodies(round *round, number uint64) ([][]byte, error) {
	stages := window.config.stages
	total := uint64(len(round.Transactions))
	bodies := make([][]byte, stages+1)
	for phase := range stages + 1 {
		body, err := round.reply(0, revealed(total, stages, phase), &number)
		if err != nil {
			return nil, err
		}

		bodies[phase] = body
	}

	return bodies, nil
}
