package utils

// This function allocates a value into the heap and returns
// a pointer to it
func HeapPtr[T any](v T) *T {
	return &v
}

func DerefSlice[T any](v *[]T) []T {
	if v == nil {
		return nil
	}
	return *v
}
