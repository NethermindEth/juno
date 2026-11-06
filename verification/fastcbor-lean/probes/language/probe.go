package language

type Limbs [4]uint64

func Wrap[T any](read func([]byte, *T) ([]byte, error)) func([]byte, *T) ([]byte, error) {
	return func(data []byte, dst *T) ([]byte, error) { return read(data, dst) }
}
func Uint[T ~uint32](x uint64) (T, bool) { return T(x), x <= 1<<32-1 }
func Arithmetic(head, want byte) (byte, byte, uint64) {
	return head - want, head & 31, uint64(1) << (head & 31)
}
func Arrays(dst *[4]uint64, x uint64) {
	for i := range 4 {
		(*dst)[i] = x + uint64(i)
	}
}
func Maps(xs []Limbs) map[Limbs]*Limbs {
	out := make(map[Limbs]*Limbs, len(xs))
	for i := range xs {
		out[xs[i]] = &xs[i]
	}
	return out
}
func Recover(f func()) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	f()
	return true
}
