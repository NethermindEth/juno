package trie

import (
	"bytes"
	"fmt"
	"math/big"
)

// prefix returns a byte representation of a binary key up to a given
// height in the tree.
func prefix(key *big.Int, height int) []byte {
	var buf bytes.Buffer
	for i := 0; i < height; i++ {
		buf.WriteString(fmt.Sprintf("%d", key.Bit(i)))
	}
	return buf.Bytes()
}

// reversed returns a copy of a *big.Int where the bits are in reverse
// order.
func reversed(x *big.Int) *big.Int {
	rev := new(big.Int)
	for i, j := keyLen-1, 0; j < keyLen; i, j = i-1, j+1 {
		rev.SetBit(rev, j, x.Bit(i))
	}
	return rev
}
