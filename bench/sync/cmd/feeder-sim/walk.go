package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	"github.com/sourcegraph/conc/pool"
	"go.uber.org/zap"
)

const progressInterval = 2 * time.Second

type fetcher interface {
	fetch(ctx context.Context, dataset dataset, resource resource) ([]byte, error)
}

type walker struct {
	feeder       fetcher
	preConfirmed fetcher
	dataset      dataset
	config       *config
	concurrency  int
	logger       *log.ZapLogger
}

func (walker *walker) walk(ctx context.Context) ([]blockInfo, error) {
	if err := walker.walkContractAddresses(ctx); err != nil {
		return nil, err
	}
	blocks, err := walker.walkBlocks(ctx)
	if err != nil {
		return nil, err
	}
	return blocks, walker.walkClasses(ctx, uniqueClassHashes(blocks))
}

func (walker *walker) walkContractAddresses(ctx context.Context) error {
	body, err := walker.visit(ctx, walker.feeder, contractAddresses, struct{}{})
	if err != nil {
		return err
	}

	expected := walker.config.network
	if expected == nil {
		return nil
	}

	coreContract, err := coreContractAddress(body)
	if err != nil {
		return err
	}

	if coreContract != expected.CoreContractAddress {
		return fmt.Errorf(
			"%s holds core contract %s, not --%s %s's %s",
			walker.dataset.root,
			hexAddress(coreContract),
			networkFlag,
			expected.Name,
			hexAddress(expected.CoreContractAddress),
		)
	}

	return nil
}

func (walker *walker) walkBlocks(ctx context.Context) ([]blockInfo, error) {
	blocks := make([]blockInfo, walker.config.to-walker.config.from+1)
	err := walker.each(ctx, "blocks", len(blocks), func(ctx context.Context, index int) (err error) {
		blocks[index], err = walker.walkBlock(ctx, walker.config.from+uint64(index))
		return err
	})
	return blocks, err
}

func (walker *walker) walkBlock(ctx context.Context, number uint64) (blockInfo, error) {
	key := blockKey{BlockNumber: number}
	if _, err := walker.visit(ctx, walker.feeder, block, key); err != nil {
		return blockInfo{}, err
	}

	body, err := walker.visit(ctx, walker.feeder, stateUpdate, key)
	if err != nil {
		return blockInfo{}, err
	}

	if walker.preConfirmed != nil {
		if _, err := walker.visit(ctx, walker.preConfirmed, preConfirmedBlock, key); err != nil {
			return blockInfo{}, err
		}
	}

	return newBlockInfo(body, number)
}

func (walker *walker) walkClasses(ctx context.Context, hashes []string) error {
	return walker.each(ctx, "classes", len(hashes), func(ctx context.Context, index int) error {
		return walker.walkClass(ctx, hashes[index])
	})
}

func (walker *walker) walkClass(ctx context.Context, hash string) error {
	key := classKey{ClassHash: hash}
	class, err := walker.visit(ctx, walker.feeder, classByHash, key)
	if err != nil {
		return err
	}

	sierra, err := isSierra(class, hash)
	if err != nil || !sierra {
		return err
	}

	_, err = walker.visit(ctx, walker.feeder, compiledClass, key)
	return err
}

func (walker *walker) visit[K, F comparable](
	ctx context.Context,
	fetcher fetcher,
	endpoint *endpoint[K, F],
	key K,
) ([]byte, error) {
	resource, err := endpoint.resource(key)
	if err != nil {
		return nil, err
	}

	return fetcher.fetch(ctx, walker.dataset, resource)
}

func (walker *walker) each(
	ctx context.Context,
	stage string,
	count int,
	visit func(ctx context.Context, index int) error,
) error {
	var done atomic.Uint64
	progressCtx, stopProgress := context.WithCancel(ctx)
	defer stopProgress()
	go walker.logProgress(progressCtx, stage, &done, count)

	workers := pool.New().
		WithContext(ctx).
		WithMaxGoroutines(walker.concurrency).
		WithCancelOnError().
		WithFirstError()
	for index := range count {
		workers.Go(func(ctx context.Context) error {
			if err := visit(ctx, index); err != nil {
				return err
			}

			done.Add(1)
			return nil
		})
	}

	return workers.Wait()
}

func (walker *walker) logProgress(
	ctx context.Context,
	stage string,
	done *atomic.Uint64,
	total int,
) {
	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			walker.logger.Info("progress", zap.String(stage, fmt.Sprintf("%d/%d", done.Load(), total)))
		}
	}
}
