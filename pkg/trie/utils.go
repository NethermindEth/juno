package trie

import (
	"bytes"
	"math/big"
	"strconv"
)

// prefix returns a byte representation of a binary key up to a given
// height in the tree.
func prefix(key *big.Int, height int) []byte {
	var buf bytes.Buffer
	for i := 0; i < height; i++ {
		buf.WriteString(strconv.FormatUint(uint64(key.Bit(i)), 10))
	}
	return buf.Bytes()
}

// reversed returns a copy of a *big.Int where the bits are in reverse
// order up to a certain length n. For example, a 4-bit representation
// of the number 11 as a *big.Int would be 0b1101, this function returns
// a *big.Int represented by the number 0b1011 which is 13. This
// function is generally used in situations where the most significant
// bit has to be in the 0th position.
func reversed(x *big.Int, n int) *big.Int {
	rev := new(big.Int)
	for i, j := n-1, 0; j < n; i, j = i-1, j+1 {
		rev.SetBit(rev, j, x.Bit(i))
	}
	return rev
}
