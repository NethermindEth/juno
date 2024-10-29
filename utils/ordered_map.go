package utils

import (
	"cmp"
	"iter"
	"slices"
)

func OrderMap[K cmp.Ordered, T any](m map[K]T) iter.Seq2[K, T] {
	// 1. collect keys
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	// 2. sort them
	slices.Sort(keys)
	return func(yield func(K, T) bool) {
		// 3. because keys are sorted now, we can iterate over them in order
		for _, k := range keys {
			if !yield(k, m[k]) {
				return
			}
		}
	}
}
