package gateway

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

var (
	errInvalidHex     = fmt.Errorf("gateway: invalid hex string")
	errOutOfRangeFelt = fmt.Errorf("gateway: out of range field element")

	// feltRegexp is the regular expression representing the StarkNet
	// field element.
	feltRegexp = regexp.MustCompile(`^0x[a-fA-F0-9]{1,64}$`)
)

// isValid returns true if felt is a valid hexadecimal representation of
// the field element a ∈ 𝔽²ₚ where p = 2²⁵¹ + 17·2¹⁹² + 1 with prefix
// "0x".
func isValid(felt string) error {
	if feltRegexp.MatchString(felt) {
		p, _ := new(big.Int).SetString("800000000000011000000000000000000000000000000000000000000000001", 16)
		// XXX: Assumes that any string that satisfies feltRegexp is a valid
		// hexadecimal string.
		f, _ := new(big.Int).SetString(strings.TrimPrefix(felt, "0x"), 16)
		if f.Cmp(p) != -1 {
			return errOutOfRangeFelt
		}
		return nil
	}
	return errInvalidHex
}
