package utils

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

func Pipeline[From any, To any](in <-chan From, f func(From) To) <-chan To {
	out := make(chan To)

	if in == nil {
		close(out)
		return out
	}

	// todo handle panic?
	go func() {
		defer close(out)
		for v := range in {
			out <- f(v)
		}
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
