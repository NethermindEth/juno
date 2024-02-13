package iter

import "context"

// Pull is our implementation of iter.Pull from stdlib
// original impl - https://cs.opensource.google/go/go/+/refs/tags/go1.22rc2:src/iter/iter.go;l=56
// Note that seq is going to be called in a separate goroutine (original impl uses private coroutine functions)
func Pull[V any](seq Seq[V]) (next func() (V, bool), stop func()) {
	ctx, stop := context.WithCancel(context.Background())
	ch := make(chan V)

	// values producer for ch
	go func() {
		defer close(ch)

		yield := func(v V) bool {
			select {
			case ch <- v:
			case <-ctx.Done():
				return false
			}

			return true
		}
		seq(yield)
	}()
	// consumer of ch
	next = func() (v V, ok bool) {
		v, ok = <-ch
		return
	}

	return next, stop
}
