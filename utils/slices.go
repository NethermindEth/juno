package utils

import "slices"

func Flatten[T any](sl ...[]T) []T {
	var result []T
	for _, slice := range sl {
		result = append(result, slice...)
	}

	return result
}

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

// MapWithErr works the same way as Map, but returns an error if any of the calls to f return an error
// in this case the first argument is nil
func MapWithErr[T1, T2 any](slice []T1, f func(T1) (T2, error)) ([]T2, error) {
	if slice == nil {
		return nil, nil
	}

	var err error
	result := make([]T2, len(slice))
	for i, e := range slice {
		result[i], err = f(e)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
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
