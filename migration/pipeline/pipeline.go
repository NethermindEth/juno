package pipeline

import (
	"context"
	"errors"
	"iter"

	"golang.org/x/sync/errgroup"
)

type resources struct {
	g      errgroup.Group
	ctx    context.Context
	cancel context.CancelFunc
	isDone bool
}

type Result struct {
	IsDone bool
	Err    error
}

type Pipeline[I any] func(*resources) <-chan I

func (p Pipeline[I]) Run(ctx context.Context) (<-chan I, func() Result) {
	ctx, cancel := context.WithCancel(ctx)
	r := resources{
		g:      errgroup.Group{},
		ctx:    ctx,
		cancel: cancel,
		isDone: false,
	}
	wait := func() Result {
		err := r.g.Wait()
		return Result{
			IsDone: r.isDone,
			Err:    err,
		}
	}
	return p(&r), wait
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
	return func(r *resources) <-chan O {
		inputs := inputs(r)
		outputs := make(chan O)
		r.g.Go(func() error {
			defer close(outputs)

			g := errgroup.Group{}
			for i := range concurrency {
				g.Go(func() error {
					var allErr error
					for input := range inputs {
						if err := state.Run(i, input, outputs); err != nil {
							allErr = errors.Join(allErr, err)
							r.cancel()
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
	return func(r *resources) <-chan I {
		outputs := make(chan I)
		r.g.Go(func() error {
			defer close(outputs)
			for input := range it {
				select {
				case <-r.ctx.Done():
					return nil
				case outputs <- input:
				}
			}
			r.isDone = true
			return nil
		})
		return outputs
	}
}
