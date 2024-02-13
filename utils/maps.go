package utils

func MapValues[K comparable, V any](m map[K]V) []V {
	sl := make([]V, 0, len(m))
	for _, v := range m {
		sl = append(sl, v)
	}

	return sl
}

func MapKeys[K comparable, V any](m map[K]V) []K {
	sl := make([]K, 0, len(m))
	for k := range m {
		sl = append(sl, k)
	}

	return sl
}

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
