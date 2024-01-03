package utils

import "github.com/NethermindEth/juno/encoder"

// Clone deep copies an object by serialising and deserialising it
// Therefore it is limited to cloning public fields only.
func Clone[T any](v T) (T, error) {
	var clone T
	if encoded, err := encoder.Marshal(v); err != nil {
		return clone, err
	} else if err = encoder.Unmarshal(encoded, &clone); err != nil {
		return clone, err
	}
	return clone, nil
}
