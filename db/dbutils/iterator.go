package dbutils

const maxByte = byte(255)

// Calculates the next possible prefix after the given prefix bytes.
// It's used to establish an upper boundary for prefix-based database scans.
// Examples:
//
//	[1]     	  -> [2]
//	[1, 255, 255] -> [2]
//	[1, 2, 255]   -> [1, 3]
//	[255, 255]    -> nil
func UpperBound(prefix []byte) []byte {
	var ub []byte

	for i := len(prefix) - 1; i >= 0; i-- {
		if prefix[i] == maxByte {
			continue
		}
		ub = make([]byte, i+1)
		copy(ub, prefix)
		ub[i]++
		return ub
	}

	return nil
}
