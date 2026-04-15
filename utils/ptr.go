package utils

// Deprecated: Use new(val) instead.
func HeapPtr[T any](v T) *T {
	return &v
}

func DerefSlice[T any](v *[]T) []T {
	if v == nil {
		return nil
	}
	return *v
}
