package utils

func Flatten[T any](slices ...[]T) []T {
	var res []T
	for _, slice := range slices {
		res = append(res, slice...)
	}

	return res
}

func Map[T1, T2 any](slice []T1, f func(T1) T2) []T2 {
	if slice == nil {
		return nil
	}

	result := make([]T2, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}

	return result
}
