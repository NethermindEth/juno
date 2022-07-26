package models

import "github.com/NethermindEth/juno/pkg/felt"

// Strings is a convenience function that maps a slice of field elements
// to a slice of strings that are hexadecimal representations of the
// field element with a "0x" prefix. This is useful for formatting the
// JSON output for a slice of *felt.Felt.
func Strings(fs []*felt.Felt) []string {
	const prefix = "0x"
	s := make([]string, 0, len(fs))
	for _, f := range fs {
		s = append(s, prefix+f.Hex())
	}
	return s
}
