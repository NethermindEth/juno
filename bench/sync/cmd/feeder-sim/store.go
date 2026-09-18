package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"sync"

	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type store struct {
	blocks []blockInfo
	mutex  sync.RWMutex
	bodies map[string][]byte
}

func loadStore(
	ctx context.Context,
	dataset dataset,
	config *config,
	logger *log.ZapLogger,
) (*store, error) {
	logger.Info(
		"loading dataset",
		zap.String("data", dataset.root),
		zap.Uint64("from", config.from),
		zap.Uint64("to", config.to),
	)
	store := &store{bodies: make(map[string][]byte)}
	walker := &walker{
		fetcher:     store,
		dataset:     dataset,
		config:      config,
		concurrency: runtime.GOMAXPROCS(0),
		logger:      logger,
	}
	blocks, err := walker.walk(ctx)
	if err != nil {
		return nil, err
	}
	store.blocks = blocks
	logger.Info(
		"dataset loaded",
		zap.Int("blocks", len(store.blocks)),
		zap.Int("bodies", len(store.bodies)),
	)
	return store, nil
}

func (store *store) fetch(_ context.Context, dataset dataset, resource resource) ([]byte, error) {
	body, err := dataset.read(resource.file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: missing; run with --%s to capture", resource.file, networkFlag)
	}
	if err != nil {
		return nil, err
	}
	store.mutex.Lock()
	store.bodies[resource.file] = body
	store.mutex.Unlock()
	return body, nil
}

func (store *store) get(file string) ([]byte, bool) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	body, ok := store.bodies[file]
	return body, ok
}
