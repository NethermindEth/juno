package utils

func NonNilSlice[T any](sl []T) []T {
	if sl == nil {
		return []T{}
	}

	return sl
}
