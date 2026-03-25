package merkle_test

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerkleLeafHash(t *testing.T) {
	data := []byte("hello")
	hash := merkleLeafHash(data)

	// Manually compute expected: SHA256("<leaf>hello</leaf>")
	h := sha256.New()
	h.Write([]byte("<leaf>hello</leaf>"))
	var expected [32]byte
	h.Sum(expected[:0])

	assert.Equal(t, expected, hash)
}

func TestMerkleLeafHash_Empty(t *testing.T) {
	hash := merkleLeafHash(nil)

	h := sha256.New()
	h.Write([]byte("<leaf></leaf>"))
	var expected [32]byte
	h.Sum(expected[:0])

	assert.Equal(t, expected, hash)
}

func TestMerkleNodeHash(t *testing.T) {
	left := merkleLeafHash([]byte("L"))
	right := merkleLeafHash([]byte("R"))
	node := merkleNodeHash(left, right)

	h := sha256.New()
	h.Write([]byte("<node><left>"))
	h.Write(left[:])
	h.Write([]byte("</left><right>"))
	h.Write(right[:])
	h.Write([]byte("</right></node>"))
	var expected [32]byte
	h.Sum(expected[:0])

	assert.Equal(t, expected, node)
}

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		n        int
		expected int
	}{
		{0, 2},
		{1, 2},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{16, 16},
		{17, 32},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, nextPowerOfTwo(tc.n), "nextPowerOfTwo(%d)", tc.n)
	}
}

func TestBuildMerkleTree_Empty(t *testing.T) {
	root, proofs := BuildMerkleTree(nil)
	assert.Equal(t, [32]byte{}, root)
	assert.Nil(t, proofs)
}

func TestBuildMerkleTree_SingleLeaf(t *testing.T) {
	leaves := [][]byte{[]byte("only")}
	root, proofs := BuildMerkleTree(leaves)

	require.Len(t, proofs, 1)

	// With one leaf padded to 2, the tree is:
	//       root
	//      /    \
	//   leaf0  empty
	leafHash := merkleLeafHash([]byte("only"))
	expectedRoot := merkleNodeHash(leafHash, emptyLeafHash)
	assert.Equal(t, expectedRoot, root)

	// Proof for leaf 0 should contain the empty leaf as sibling.
	assert.Len(t, proofs[0].Siblings, 1)
	assert.Equal(t, emptyLeafHash, proofs[0].Siblings[0])
}

func TestBuildMerkleTree_TwoLeaves(t *testing.T) {
	leaves := [][]byte{[]byte("A"), []byte("B")}
	root, proofs := BuildMerkleTree(leaves)

	require.Len(t, proofs, 2)

	h0 := merkleLeafHash([]byte("A"))
	h1 := merkleLeafHash([]byte("B"))
	expectedRoot := merkleNodeHash(h0, h1)
	assert.Equal(t, expectedRoot, root)

	// Leaf 0's sibling is leaf 1.
	assert.Equal(t, h1, proofs[0].Siblings[0])
	// Leaf 1's sibling is leaf 0.
	assert.Equal(t, h0, proofs[1].Siblings[0])
}

func TestBuildMerkleTree_FourLeaves(t *testing.T) {
	leaves := [][]byte{[]byte("A"), []byte("B"), []byte("C"), []byte("D")}
	root, proofs := BuildMerkleTree(leaves)

	require.Len(t, proofs, 4)

	// Build expected tree manually:
	//        root
	//       /    \
	//     n01     n23
	//    / \     / \
	//   h0  h1  h2  h3
	h0 := merkleLeafHash([]byte("A"))
	h1 := merkleLeafHash([]byte("B"))
	h2 := merkleLeafHash([]byte("C"))
	h3 := merkleLeafHash([]byte("D"))
	n01 := merkleNodeHash(h0, h1)
	n23 := merkleNodeHash(h2, h3)
	expectedRoot := merkleNodeHash(n01, n23)
	assert.Equal(t, expectedRoot, root)

	// Proof for leaf 0: siblings [h1, n23]
	require.Len(t, proofs[0].Siblings, 2)
	assert.Equal(t, h1, proofs[0].Siblings[0])
	assert.Equal(t, n23, proofs[0].Siblings[1])

	// Proof for leaf 2: siblings [h3, n01]
	require.Len(t, proofs[2].Siblings, 2)
	assert.Equal(t, h3, proofs[2].Siblings[0])
	assert.Equal(t, n01, proofs[2].Siblings[1])
}

func TestBuildMerkleTree_ThreeLeaves(t *testing.T) {
	// Three leaves means padding to 4: the fourth leaf is empty.
	leaves := [][]byte{[]byte("A"), []byte("B"), []byte("C")}
	root, proofs := BuildMerkleTree(leaves)

	require.Len(t, proofs, 3)

	h0 := merkleLeafHash([]byte("A"))
	h1 := merkleLeafHash([]byte("B"))
	h2 := merkleLeafHash([]byte("C"))
	h3 := emptyLeafHash
	n01 := merkleNodeHash(h0, h1)
	n23 := merkleNodeHash(h2, h3)
	expectedRoot := merkleNodeHash(n01, n23)
	assert.Equal(t, expectedRoot, root)

	// Proof for leaf 2: siblings [emptyLeaf, n01]
	require.Len(t, proofs[2].Siblings, 2)
	assert.Equal(t, h3, proofs[2].Siblings[0])
	assert.Equal(t, n01, proofs[2].Siblings[1])
}

func TestVerifyMerkleProof_ValidProofs(t *testing.T) {
	// Build a tree with several leaves, then verify every proof.
	data := [][]byte{
		[]byte("alpha"),
		[]byte("bravo"),
		[]byte("charlie"),
		[]byte("delta"),
		[]byte("echo"),
	}
	root, proofs := BuildMerkleTree(data)
	require.Len(t, proofs, len(data))

	for i, d := range data {
		ok := VerifyMerkleProof(root, d, uint32(i), proofs[i])
		assert.True(t, ok, "proof for leaf %d should verify", i)
	}
}

func TestVerifyMerkleProof_WrongData(t *testing.T) {
	leaves := [][]byte{[]byte("real"), []byte("data")}
	root, proofs := BuildMerkleTree(leaves)

	// Tamper with the data.
	ok := VerifyMerkleProof(root, []byte("fake"), 0, proofs[0])
	assert.False(t, ok, "tampered data should not verify")
}

func TestVerifyMerkleProof_WrongIndex(t *testing.T) {
	leaves := [][]byte{[]byte("A"), []byte("B"), []byte("C"), []byte("D")}
	root, proofs := BuildMerkleTree(leaves)

	// Use leaf 0's data with leaf 1's index.
	ok := VerifyMerkleProof(root, []byte("A"), 1, proofs[0])
	assert.False(t, ok, "wrong index should not verify")
}

func TestVerifyMerkleProof_WrongRoot(t *testing.T) {
	leaves := [][]byte{[]byte("A"), []byte("B")}
	_, proofs := BuildMerkleTree(leaves)

	fakeRoot := [32]byte{0xff}
	ok := VerifyMerkleProof(fakeRoot, []byte("A"), 0, proofs[0])
	assert.False(t, ok, "wrong root should not verify")
}

func TestVerifyMerkleProof_TamperedSibling(t *testing.T) {
	leaves := [][]byte{[]byte("A"), []byte("B")}
	root, _ := BuildMerkleTree(leaves)

	// Tamper with a sibling hash in the proof.
	badProof := MerkleProof{Siblings: [][32]byte{{0xde, 0xad}}}
	ok := VerifyMerkleProof(root, []byte("A"), 0, badProof)
	assert.False(t, ok, "tampered sibling should not verify")
}

func TestBuildAndVerify_LargeTree(t *testing.T) {
	// Build a tree with a non-power-of-two count to exercise padding.
	n := 31
	leaves := make([][]byte, n)
	for i := range n {
		leaves[i] = []byte{byte(i), byte(i >> 8)}
	}

	root, proofs := BuildMerkleTree(leaves)
	require.Len(t, proofs, n)

	for i, leaf := range leaves {
		assert.True(t,
			VerifyMerkleProof(root, leaf, uint32(i), proofs[i]),
			"proof for leaf %d in 31-leaf tree should verify", i,
		)
	}
}
