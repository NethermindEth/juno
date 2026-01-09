package blocktransactions

import (
	"golang.org/x/sync/errgroup"
)

type state[I, O any] interface {
	run(index int, input I, outputs chan<- O) error
	done(index int, outputs chan<- O) error
}

type pipeline[I, O any, S state[I, O]] struct {
	g       *errgroup.Group
	outputs chan O
}

func newPipeline[I, O any, S state[I, O]](
	inputs <-chan I,
	concurrency int,
	state S,
) pipeline[I, O, S] {
	p := pipeline[I, O, S]{
		g:       &errgroup.Group{},
		outputs: make(chan O),
	}

	for i := range concurrency {
		p.g.Go(func() error {
			for input := range inputs {
				if err := state.run(i, input, p.outputs); err != nil {
					return err
				}
			}
			return state.done(i, p.outputs)
		})
	}

	return p
}

func (p *pipeline[I, O, S]) wait() error {
	defer close(p.outputs)
	return p.g.Wait()
}
