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
