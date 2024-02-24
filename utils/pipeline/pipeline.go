package pipeline

import (
	"context"
	"sync"
)

func Stage[From any, To any](ctx context.Context, in <-chan From, f func(From) To) <-chan To {
	out := make(chan To)

	if in == nil {
		close(out)
		return out
	}

	// todo handle panic?
	go func() {
		defer close(out)
		for v := range in {
			select {
			case <-ctx.Done():
				return
			default:
				out <- f(v)
			}
		}
	}()

	return out
}

func FanIn[T any](ctx context.Context, channels ...<-chan T) <-chan T {
	var wg sync.WaitGroup
	out := make(chan T)

	multiplex := func(ch <-chan T) {
		defer wg.Done()

		if ch == nil {
			return
		}

		for i := range ch {
			select {
			case <-ctx.Done():
				return
			case out <- i:
			}
		}
	}

	wg.Add(len(channels))
	for _, ch := range channels {
		go multiplex(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func Bridge[T any](ctx context.Context, chanCh <-chan <-chan T) <-chan T {
	out := make(chan T)
	if chanCh == nil {
		close(out)
		return out
	}
	go func() {
		defer close(out)
		for {
			var ch <-chan T
			select {
			case <-ctx.Done():
				return
			case gotCh, ok := <-chanCh:
				if !ok {
					return
				}
				ch = gotCh
			innerLoop:
				for {
					select {
					case <-ctx.Done():
						break innerLoop
					case val, ok := <-ch:
						if !ok {
							break innerLoop
						}
						select {
						case <-ctx.Done():
						case out <- val:
						}
					}
				}
			}
		}
	}()
	return out
}
