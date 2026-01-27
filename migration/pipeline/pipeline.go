package pipeline

import (
	"context"
	"errors"
	"iter"

	"github.com/sourcegraph/conc/pool"
	"golang.org/x/sync/errgroup"
)

type Pipeline[I any] func(*pool.ContextPool, context.CancelFunc) <-chan I

func (p Pipeline[I]) Run(ctx context.Context) (<-chan I, func() error) {
	ctx, cancel := context.WithCancel(ctx)
	g := pool.New().WithContext(ctx)
	return p(g, cancel), g.Wait
}

type State[I, O any] interface {
	Run(index int, input I, outputs chan<- O) error
	Done(index int, outputs chan<- O) error
}

func New[I, O any, S State[I, O]](
	inputs Pipeline[I],
	concurrency int,
	state S,
) Pipeline[O] {
	return func(g *pool.ContextPool, cancel context.CancelFunc) <-chan O {
		inputs := inputs(g, cancel)
		outputs := make(chan O)
		g.Go(func(context.Context) error {
			defer close(outputs)

			g := errgroup.Group{}
			for i := range concurrency {
				g.Go(func() error {
					var allErr error
					for input := range inputs {
						if err := state.Run(i, input, outputs); err != nil {
							allErr = errors.Join(allErr, err)
							cancel()
						}
					}
					return errors.Join(allErr, state.Done(i, outputs))
				})
			}

			return g.Wait()
		})

		return outputs
	}
}

func Source[I any](it iter.Seq[I]) Pipeline[I] {
	return func(g *pool.ContextPool, cancel context.CancelFunc) <-chan I {
		outputs := make(chan I)
		g.Go(func(ctx context.Context) error {
			defer close(outputs)
			for input := range it {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case outputs <- input:
				}
			}
			return nil
		})
		return outputs
	}
}
