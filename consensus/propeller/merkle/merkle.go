// Package merkle implements Merkle tree construction and verification using a
// SHA-256 tagging scheme. Tags prevent second-preimage attacks by
// domain-separating leaf hashes from internal node hashes. The exact tag
// format matches the Propeller protocol specification so that all
// implementations produce identical trees.
//
// Tree layout: leaves are at the bottom, padded to the next power-of-two
// with the hash of empty data. The tree is built bottom-up by hashing pairs.
package merkle

import (
	"crypto/sha256"
	"math/bits"
)

const (
	leafOpenTag  = "<leaf>"
	leafCloseTag = "</leaf>"
	nodeOpenTag  = "<node><left>"
	nodeMidTag   = "</left><right>"
	nodeCloseTag = "</right></node>"
)

// Pre-computed domain-separator tags to avoid repeated []byte conversions.
var (
	leafOpen  = []byte(leafOpenTag)
	leafClose = []byte(leafCloseTag)
	nodeOpen  = []byte(nodeOpenTag)
	nodeMid   = []byte(nodeMidTag)
	nodeClose = []byte(nodeCloseTag)
)

// emptyLeafHash is the hash of a padding leaf (no data). We precompute it
// because the same value is used repeatedly when the leaf count is not a
// power of two.
var emptyLeafHash = merkleLeafHash(nil)

type Hash [32]byte

// Proof contains the sibling hashes needed to verify that a leaf
// belongs to a Merkle tree with a known root. Siblings are ordered from
// leaf level (index 0) up to the root.
type Proof struct {
	Siblings []Hash
}

// Verify checks that a leaf at the given index is included in a tree with
// the claimed root. The proof contains sibling hashes from the leaf level
// up to the root.
//
// The index determines the path through the tree: at each level, if the
// current bit of the index is 0 the current hash is the left child and the
// sibling is the right child, and vice versa.
func (p *Proof) Verify(root *Hash, leaf []byte, index uint32) bool {
	current := merkleLeafHash(leaf)

	idx := index
	for i := range p.Siblings {
		if idx%2 == 0 {
			current = merkleNodeHash(&current, &p.Siblings[i])
		} else {
			current = merkleNodeHash(&p.Siblings[i], &current)
		}
		idx /= 2
	}

	return current == *root
}

// Tree is a set of inclusion proofs, one per original leaf.
type Tree []Proof

// New constructs a binary Merkle tree from the given leaf data
// and returns the root hash plus one inclusion proof per original leaf.
//
// The tree is padded to the next power-of-two size with empty leaves. This
// simplifies the proof logic: every node at every level has a sibling, and
// the proof path length is always log2(paddedSize).
//
// Returns a zero root and nil Tree if leaves is empty.
func New(leaves [][]byte) (root Hash, tree Tree) {
	n := len(leaves)
	if n == 0 {
		// todo(rdr): maybe here we return a default merkle tree
		return Hash{}, nil
	}

	size := nextPowerOfTwo(n)

	// Build the bottom layer: hash each leaf, pad to power-of-two.
	layer := make([]Hash, size)
	for i := range n {
		//nolint: gosec // Everything is inbouds here
		layer[i] = merkleLeafHash(leaves[i])
	}
	for i := n; i < size; i++ {
		layer[i] = emptyLeafHash
	}

	// proofSiblings[i] accumulates the sibling hashes for leaf i's proof.
	// ancestors[i] tracks leaf i's ancestor position in the current layer.
	proofSiblings := make([][]Hash, n)
	ancestors := make([]int, n)
	for j := range n {
		ancestors[j] = j
	}

	// Build the tree bottom-up, one level at a time.
	for len(layer) > 1 {
		nextLayer := make([]Hash, len(layer)/2)
		for i := 0; i < len(layer); i += 2 {
			nextLayer[i/2] = merkleNodeHash(&layer[i], &layer[i+1])
		}
		for i := range n {
			proofSiblings[i] = append(proofSiblings[i], layer[ancestors[i]^1])
			ancestors[i] /= 2
		}
		layer = nextLayer
	}

	root = layer[0]

	tree = make([]Proof, n)
	for i := range n {
		tree[i] = Proof{Siblings: proofSiblings[i]}
	}

	return root, tree
}

// merkleLeafHash computes: SHA256("<leaf>" || data || "</leaf>")
//
// The XML-like tags are the domain separator specified by the Propeller
// protocol. They ensure a leaf hash can never collide with a node hash,
// even if an attacker controls the data.
func merkleLeafHash(data []byte) Hash {
	buf := make([]byte, len(leafOpenTag)+len(data)+len(leafCloseTag))

	n := copy(buf, leafOpen)
	n += copy(buf[n:], data)
	copy(buf[n:], leafClose)

	return sha256.Sum256(buf)
}

// merkleNodeHash computes:
//
//	SHA256("<node><left>" || left || "</left><right>" || right || "</right></node>")
//
// The nested tags ensure node hashes are in a separate domain from leaf hashes.
func merkleNodeHash(left, right *Hash) Hash {
	const size = len(nodeOpenTag) + 32 + len(nodeMidTag) + 32 + len(nodeCloseTag)
	var buf [size]byte

	n := copy(buf[:], nodeOpen)
	n += copy(buf[n:], left[:])
	n += copy(buf[n:], nodeMid)
	n += copy(buf[n:], right[:])
	copy(buf[n:], nodeClose)

	return sha256.Sum256(buf[:])
}

// nextPowerOfTwo returns the smallest power of two >= n, with a minimum of 2.
// A minimum of 2 ensures even a single-leaf tree has a sibling for its proof.
func nextPowerOfTwo(n int) int {
	if n <= 2 {
		return 2
	}
	// bits.Len returns the position of the highest set bit + 1.
	// Subtracting 1 before Len handles exact powers-of-two correctly.
	return 1 << bits.Len(uint(n-1))
}
