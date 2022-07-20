package trie

import (
	"bytes"
	"strconv"

	"github.com/NethermindEth/juno/pkg/felt"
)

// Prefix returns a byte representation of a binary key up to a given
// height in the tree.
func Prefix(key *felt.Felt, height int) []byte {
	reg := key.ToRegular()
	var buf bytes.Buffer
	for i := 0; i < height; i++ {
		buf.WriteString(strconv.FormatUint(reg.Bit(uint64(i)), 10))
	}
	return buf.Bytes()
}

// Reversed returns a copy of a *felt.Felt where the bits are in reverse
// order up to a certain length n. For example, a 4-bit representation
// of the number 11 as a *felt.Felt would be 0b1101, this function returns
// a *felt.Felt represented by the number 0b1011 which is 13. This
// function is generally used in situations where the most significant
// bit has to be in the 0th position.
func Reversed(x *felt.Felt, n int) *felt.Felt {
	rev := new(felt.Felt)
	_x := *x
	_x.FromMont()
	for i, j := n-1, 0; j < n; i, j = i-1, j+1 {
		rev.SetBit(uint64(j), _x.Bit(uint64(i)))
	}
	return rev.ToMont()
}
