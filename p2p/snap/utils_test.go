package snap

import (
	"github.com/NethermindEth/juno/core/trie"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_BitsetSerialization(t *testing.T) {
	bset := trie.NewKey(0, []byte{0})
	assertBitsetTheSame(t, &bset)

	bset = trie.NewKey(100, []byte{1, 2, 3})
	assertBitsetTheSame(t, &bset)

	bset = trie.NewKey(200, []byte{1, 2, 3})
	assertBitsetTheSame(t, &bset)

	bset = trie.NewKey(16, []byte{0})
	assertBitsetTheSame(t, &bset)
}

func assertBitsetTheSame(t *testing.T, bset *trie.Key) {
	assert.True(t, protoToBitset(bitsetToProto(bset)).Equal(bset))
}
