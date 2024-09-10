package utils

func ToMap[T any, K comparable, V any](sl []T, f func(T) (K, V)) map[K]V {
	m := make(map[K]V, len(sl))
	for _, item := range sl {
		k, v := f(item)
		m[k] = v
	}

	return m
}

func ToSlice[K comparable, V any, T any](m map[K]V, f func(K, V) T) []T {
	if m == nil {
		return nil
	}

	sl := make([]T, 0, len(m))
	for k, v := range m {
		sl = append(sl, f(k, v))
	}

	return sl
}
