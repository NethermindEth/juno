package utils

import (
	"reflect"
	"slices"
)

func Map[T1, T2 any](slice []T1, f func(T1) T2) []T2 {
	if slice == nil {
		return nil
	}

	result := make([]T2, len(slice))
	for i, e := range slice {
		result[i] = f(e)
	}

	return result
}

func Filter[T any](slice []T, f func(T) bool) []T {
	var result []T
	for _, e := range slice {
		if f(e) {
			result = append(result, e)
		}
	}

	return result
}

// All returns true if all elements match the given predicate
func All[T any](slice []T, f func(T) bool) bool {
	return slices.IndexFunc(slice, func(e T) bool { return !f(e) }) == -1
}

func AnyOf[T comparable](e T, values ...T) bool {
	return slices.Contains(values, e)
}

// Unique returns a new slice with duplicates removed
func Unique[T comparable](slice []T) []T {
	// do not support unique on pointer types, just return the slice as it is
	if len(slice) > 0 {
		elt := slice[0]
		if reflect.TypeOf(elt).Kind() == reflect.Ptr {
			return slice
		}
	}

	result := make([]T, 0, len(slice))
	seen := make(map[T]struct{}, len(slice))
	for _, e := range slice {
		if _, ok := seen[e]; !ok {
			result = append(result, e)
			seen[e] = struct{}{}
		}
	}

	return result
}
