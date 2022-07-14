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
)

// isValid returns true if felt is a valid hexadecimal representation of
// the field element a ∈ 𝔽²ₚ where p = 2²⁵¹ + 17·2¹⁹² + 1 with prefix
// "0x".
func isValid(felt string) error {
	re := regexp.MustCompile(`^0x[a-fA-F0-9]{1,64}$`)
	if re.MatchString(felt) {
		p, _ := new(big.Int).SetString("800000000000011000000000000000000000000000000000000000000000001", 16)
		// XXX: Can this check for ok be omitted given the regular
		// expression guarantees (?) that this will be a valid hexadecimal
		// string?
		f, ok := new(big.Int).SetString(strings.TrimPrefix(felt, "0x"), 16)
		if !ok {
			return errInvalidHex
		}
		if f.Cmp(p) != -1 {
			return errOutOfRangeFelt
		}
		return nil
	}
	return errInvalidHex
}
