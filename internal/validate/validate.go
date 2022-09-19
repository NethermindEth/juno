// Package validate contains helpers that validate input strings against
// certain regular expressions.
package validate

import (
	"math/big"
	"regexp"
	"strings"
	"sync"
)

var (
	compileReOnce sync.Once
	re            *regexp.Regexp

	p *big.Int
)

func compileRe() {
	re = regexp.MustCompile(`^0x0[a-fA-F0-9]{1,63}$`)
	p, _ = new(big.Int).SetString("800000000000011000000000000000000000000000000000000000000000001", 16)
}

func init() {
	compileReOnce.Do(compileRe)
}

// Felt returns true if s conforms to the regular expression
// ^0x0[a-fA-F0-9]{1,63}$ and represents a number in the range 0 ≤ s < p
// where p = 2²⁵¹ + 17 ⋅ 2¹⁹² + 1.
func Felt(s string) bool {
	if re.MatchString(s) {
		// XXX: The following assumes that any string that satisfies re is a
		// valid hexadecimal number which makes it okay to ignore the error.
		f, _ := new(big.Int).SetString(strings.TrimPrefix(s, "0x"), 16)
		return f.Cmp(p) == -1
	}
	return false
}

// Felt returns true if each string in ss conforms to the regular
// expression ^0x0[a-fA-F0-9]{1,63}$ and represents a number in the
// range 0 ≤ s < p where p = 2²⁵¹ + 17 ⋅ 2¹⁹² + 1.
func Felts(ss []string) bool {
	for _, s := range ss {
		if ok := Felt(s); !ok {
			return false
		}
	}
	return true
}
