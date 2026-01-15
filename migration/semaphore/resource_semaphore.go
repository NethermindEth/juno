package semaphore

import (
	"context"

	"golang.org/x/sync/semaphore"
)

// ResourceSemaphore manages concurrent access to resources by limiting the number of active
// operations using a semaphore.
type ResourceSemaphore[T any] struct {
	sem *semaphore.Weighted
	fn  func() T
}

func New[T any](concurrency int, fn func() T) ResourceSemaphore[T] {
	return ResourceSemaphore[T]{
		sem: semaphore.NewWeighted(int64(concurrency)),
		fn:  fn,
	}
}

func (s ResourceSemaphore[T]) GetBlocking() T {
	res, _ := s.Get(context.Background())
	return res
}

func (s ResourceSemaphore[T]) Get(ctx context.Context) (T, error) {
	if err := s.sem.Acquire(ctx, 1); err != nil {
		return *new(T), err
	}
	return s.fn(), nil
}

func (s ResourceSemaphore[T]) Put() {
	s.sem.Release(1)
}
