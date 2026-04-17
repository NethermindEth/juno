package trieutils

import "github.com/NethermindEth/juno/core/felt"

// Converts a Felt value into a Path representation suitable to
// use as a trie key with the specified height.
func FeltToPath(f *felt.Felt, height uint8) Path {
	var key Path
	key.SetFelt(height, f)
	return key
}
