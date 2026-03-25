package pool

import (
	"context"
	"time"
)

type Pool[T any] struct {
	ctx           context.Context
	taskTimeout   time.Duration
	activeWorkers uint64
	maxWorkers    uint64
}

func New[T any](
	ctx context.Context,
	taskTimeout time.Duration,
	maxWorkers uint64,
) *Pool[T] {
	return &Pool[T]{
		ctx:           ctx,
		taskTimeout:   taskTimeout,
		activeWorkers: 0,
		maxWorkers:    maxWorkers,
	}
}
