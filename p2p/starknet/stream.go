package starknet

// Stream represents a series of messages that can be accessed by invoking Stream a number of times.
// After stream is entirely consumed of elements, it should return false as its second return value
type Stream[T any] func() (T, bool)

func StaticStream[T any](elems ...T) Stream[T] {
	index := 0
	return func() (T, bool) {
		var zero T
		if index >= len(elems) {
			return zero, false
		}
		index++
		return elems[index-1], true
	}
}
