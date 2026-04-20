package utils

func DerefSlice[T any](v *[]T) []T {
	if v == nil {
		return nil
	}
	return *v
}
