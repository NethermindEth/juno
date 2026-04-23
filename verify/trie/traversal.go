package trie

import (
	"context"

	"golang.org/x/sync/errgroup"
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
	eg, gCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		var err error
		left, err = leftFn(gCtx)
		return err
	})

	eg.Go(func() error {
		var err error
		right, err = rightFn(gCtx)
		return err
	})

	err = eg.Wait()
	return left, right, err
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
