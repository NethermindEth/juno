package trie

import (
	"context"
	"sync"
)

func TraverseBinary[T any](
	ctx context.Context,
	depth uint8,
	maxConcurrentDepth uint8,
	leftFn func(ctx context.Context) (T, error),
	rightFn func(ctx context.Context) (T, error),
) (left, right T, err error) {
	if depth <= maxConcurrentDepth {
		return traverseConcurrently(ctx, leftFn, rightFn)
	}
	return traverseSequentially(ctx, leftFn, rightFn)
}

func traverseConcurrently[T any](
	ctx context.Context,
	leftFn func(ctx context.Context) (T, error),
	rightFn func(ctx context.Context) (T, error),
) (left, right T, err error) {
	var leftErr, rightErr error
	var wg sync.WaitGroup

	wg.Go(func() {
		left, leftErr = leftFn(ctx)
	})

	right, rightErr = rightFn(ctx)
	wg.Wait()

	if leftErr != nil {
		return left, right, leftErr
	}
	if rightErr != nil {
		return left, right, rightErr
	}
	return left, right, nil
}

func traverseSequentially[T any](
	ctx context.Context,
	leftFn func(ctx context.Context) (T, error),
	rightFn func(ctx context.Context) (T, error),
) (left, right T, err error) {
	left, err = leftFn(ctx)
	if err != nil {
		return left, right, err
	}
	right, err = rightFn(ctx)
	return left, right, err
}
