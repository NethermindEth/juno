// Package validate contains helpers that validate input strings against
// certain regular expressions.
package validate

import (
	"regexp"
	"sync"
)

var (
	compileReOnce sync.Once
	fe            *regexp.Regexp
)

func compileRe() {
	fe = regexp.MustCompile(`^0x0[a-fA-F0-9]{1,63}$`)
}

func init() {
	compileReOnce.Do(compileRe)
}

// Felt returns true if s conforms to the regular expression
// ^0x0[a-fA-F0-9]{1,63}$ and false otherwise.
func Felt(s string) bool {
	return fe.MatchString(s)
}

// Felt returns true if each string in ss conforms to the regular
// expression ^0x0[a-fA-F0-9]{1,63}$ and false otherwise.
func Felts(ss []string) bool {
	for _, s := range ss {
		if ok := Felt(s); !ok {
			return false
		}
	}
	return true
}
