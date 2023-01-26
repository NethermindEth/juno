package utils

import "github.com/NethermindEth/juno/core/felt"

func HexToFelt(hex string) *felt.Felt {
	// We know our test hex values are valid, so we'll ignore the potential error
	f, _ := new(felt.Felt).SetString(hex)
	return f
}
