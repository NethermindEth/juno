package utils

import (
	"reflect"
	"slices"
	"strings"

	"github.com/NethermindEth/juno/core/felt"
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

// Unique returns a new slice with duplicates removed.
// Panics if the slice contains pointer types.
func Set[T comparable](slice []T) []T {
	if len(slice) == 0 {
		return slice
	}

	if reflect.TypeOf(slice[0]).Kind() == reflect.Ptr {
		panic("Set does not support pointer types")
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

func FeltArrToString(arr []*felt.Felt) string {
	res := make([]string, len(arr))
	for i, felt := range arr {
		res[i] = felt.String()
	}
	return strings.Join(res, ", ")
}
