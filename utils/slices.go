package utils

func Flatten[T any](slices ...[]T) []T {
	var res []T
	for _, slice := range slices {
		res = append(res, slice...)
	}

	return res
}
