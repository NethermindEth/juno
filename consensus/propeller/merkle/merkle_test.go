package merkle_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/consensus/propeller/merkle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeLeaves(n int) [][]byte {
	leaves := make([][]byte, n)
	for i := range n {
		leaves[i] = fmt.Appendf(nil, "leaf-%d", i)
	}
	return leaves
}

func TestNew_Empty(t *testing.T) {
	root, proofs := merkle.New(nil)
	assert.Equal(t, merkle.Hash{}, root)
	assert.Nil(t, proofs)
}

func TestNew_ProofsVerify(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 8, 16, 31} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			leaves := makeLeaves(n)
			root, proofs := merkle.New(leaves)

			require.Len(t, proofs, n)
			assert.NotEqual(t, merkle.Hash{}, root)

			for i, leaf := range leaves {
				assert.True(t,
					proofs[i].Verify(&root, leaf, uint32(i)),
					"proof for leaf %d should verify", i,
				)
			}
		})
	}
}

func TestNew_WrongDataDoesNotVerify(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 31} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			leaves := makeLeaves(n)
			root, proofs := merkle.New(leaves)

			for i := range leaves {
				assert.False(t,
					proofs[i].Verify(&root, []byte("tampered"), uint32(i)),
					"tampered data should not verify for leaf %d", i,
				)
			}
		})
	}
}

func TestVerify_Rejects(t *testing.T) {
	leaves := [][]byte{[]byte("A"), []byte("B"), []byte("C"), []byte("D")}
	root, proofs := merkle.New(leaves)

	t.Run("wrong index", func(t *testing.T) {
		assert.False(t, proofs[0].Verify(&root, leaves[0], 1))
	})

	t.Run("wrong root", func(t *testing.T) {
		fakeRoot := merkle.Hash{0xff}
		assert.False(t, proofs[0].Verify(&fakeRoot, leaves[0], 0))
	})

	t.Run("tampered sibling", func(t *testing.T) {
		badProof := merkle.Proof{Siblings: []merkle.Hash{{0xde, 0xad}}}
		assert.False(t, badProof.Verify(&root, leaves[0], 0))
	})
}

func TestNew_Deterministic(t *testing.T) {
	leaves := makeLeaves(7)

	root1, proofs1 := merkle.New(leaves)
	root2, proofs2 := merkle.New(leaves)

	assert.Equal(t, root1, root2)
	require.Len(t, proofs1, len(proofs2))
	for i := range proofs1 {
		assert.Equal(t, proofs1[i], proofs2[i], "proof %d should be identical", i)
	}
}

func TestNew_DifferentLeavesDifferentRoots(t *testing.T) {
	rootA, _ := merkle.New([][]byte{[]byte("A"), []byte("B")})
	rootB, _ := merkle.New([][]byte{[]byte("X"), []byte("Y")})

	assert.NotEqual(t, rootA, rootB)
}

func TestNew_CrossTreeIsolation(t *testing.T) {
	leavesA := [][]byte{[]byte("A"), []byte("B"), []byte("C"), []byte("D")}
	leavesB := [][]byte{[]byte("W"), []byte("X"), []byte("Y"), []byte("Z")}

	rootB, _ := merkle.New(leavesB)
	_, proofsA := merkle.New(leavesA)

	for i, leaf := range leavesA {
		assert.False(t,
			proofsA[i].Verify(&rootB, leaf, uint32(i)),
			"proof from tree A should not verify against tree B root",
		)
	}
}
