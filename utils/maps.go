package utils

import (
	"cmp"
	"iter"
	"maps"
	"slices"
)

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

func SortedMap[K cmp.Ordered, T any](m map[K]T) iter.Seq2[K, T] {
	// 1. collect keys
	keys := maps.Keys(m)
	// 2. sort them
	sortedKeys := slices.Sorted(keys)
	return func(yield func(K, T) bool) {
		// 3. pass key-value pairs in sorted order
		for _, k := range sortedKeys {
			if !yield(k, m[k]) {
				return
			}
		}
	}
}
