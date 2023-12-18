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

func PipelineFanIn(ctx context.Context, channels ...<-chan any) <-chan any {
	var wg sync.WaitGroup
	out := make(chan any)

	multiplex := func(ch <-chan any) {
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
