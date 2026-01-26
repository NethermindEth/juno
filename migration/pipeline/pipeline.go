package pipeline

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type State[I, O any] interface {
	Run(index int, input I, outputs chan<- O) error
	Done(index int, outputs chan<- O) error
}

type pipeline[I, O any, S State[I, O]] struct {
	g       *errgroup.Group
	ctx     context.Context
	outputs chan O
}

func New[I, O any, S State[I, O]](
	ctx context.Context,
	inputs <-chan I,
	concurrency int,
	state S,
) pipeline[I, O, S] {
	g, pipelineCtx := errgroup.WithContext(ctx)
	p := pipeline[I, O, S]{
		g:       g,
		ctx:     pipelineCtx,
		outputs: make(chan O),
	}

	for i := range concurrency {
		p.g.Go(func() error {
			for input := range inputs {
				if err := state.Run(i, input, p.outputs); err != nil {
					return err
				}
			}
			return state.Done(i, p.outputs)
		})
	}

	return p
}

// Context returns the pipeline's context, which is cancelled when any worker errors.
// The generator should use this context to stop producing inputs when workers fail.
func (p *pipeline[I, O, S]) Context() context.Context {
	return p.ctx
}

func (p *pipeline[I, O, S]) Wait() error {
	defer close(p.outputs)
	return p.g.Wait()
}

func (p *pipeline[I, O, S]) Outputs() <-chan O {
	return p.outputs
}
