package utils

import (
	"fmt"
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
	for _, v := range values {
		if e == v {
			return true
		}
	}
	return false
}

// Unique returns a new slice with duplicates removed
func Unique[T comparable](slice []T) []T {
	// check if not used with a pointer type
	if len(slice) > 0 {
		elt := slice[0]
		if reflect.TypeOf(elt).Kind() == reflect.Ptr {
			panic(fmt.Sprintf("Unique() cannot be used with a slice of pointers (%T). Use `UniqueFunc()` instead.", elt))
		}
	}

	return UniqueFunc(slice, func(t T) T { return t })
}

// UniqueFunc returns a new slice with duplicates removed, using a key function
func UniqueFunc[T, K comparable](slice []T, key func(T) K) []T {
	var result []T
	seen := make(map[K]struct{})
	for _, e := range slice {
		k := key(e)
		if _, ok := seen[k]; !ok {
			result = append(result, e)
			seen[k] = struct{}{}
		}
	}
	return result
}
