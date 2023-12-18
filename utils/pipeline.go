package utils

import (
	"context"
	"sync"
)

// todo: Consider moving this and the test file to its own package
func PriorityQueue[T any](highPriority, lowPriority <-chan T) <-chan T {
	out := make(chan T)

	go func() {
		defer close(out)

		for highPriority != nil || lowPriority != nil {
			// first we always check highPriority channel for data
			select {
			case v, ok := <-highPriority:
				if ok {
					out <- v
				} else {
					highPriority = nil
				}
			default:
			}

			// order of cases in select stmt doesn't guarantee processing order
			select {
			case v, ok := <-highPriority:
				if ok {
					out <- v
				} else {
					highPriority = nil
				}
			case v, ok := <-lowPriority:
				if ok {
					out <- v
				} else {
					lowPriority = nil
				}
			}
		}
	}()

	return out
}

func PipelineStage[From any, To any](ctx context.Context, in <-chan From, f func(From) To) <-chan To {
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

func PipelineFanIn[T any](ctx context.Context, channels ...<-chan T) <-chan T {
	var wg sync.WaitGroup
	out := make(chan T)

	multiplex := func(ch <-chan T) {
		defer wg.Done()
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

func PipelineEnd[T any](in <-chan T, f func(T)) <-chan struct{} {
	done := make(chan struct{})

	if in == nil {
		close(done)
		return done
	}

	// todo handle panic?
	go func() {
		defer close(done)
		for v := range in {
			f(v)
		}
	}()

	return done
}

func PipelineBridge[T any](ctx context.Context, chanCh <-chan <-chan T) <-chan T {
	out := make(chan T)
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
