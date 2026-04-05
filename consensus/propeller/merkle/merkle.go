package merkle

import (
	"crypto/sha256"
	"math/bits"
)

type Hash [32]byte

// Proof contains the sibling hashes needed to verify that a leaf
// belongs to a Merkle tree with a known root. Siblings are ordered from
// leaf level (index 0) up to the root.
type Proof struct {
	Siblings []Hash
}

// VerifyProof checks that a leaf at the given index is included in a
// tree with the claimed root. The proof contains sibling hashes from the leaf
// level up to the root.
//
// The index determines the path through the tree: at each level, if the
// current bit of the index is 0 the current hash is the left child and the
// sibling is the right child, and vice versa.
func (p *Proof) Verify(root *Hash, leaf []byte, index uint32) bool {
	current := merkleLeafHash(leaf)

	idx := index
	for _, sibling := range p.Siblings {
		if idx%2 == 0 {
			// Current node is left child, sibling is right.
			current = merkleNodeHash(current, sibling)
		} else {
			// Current node is right child, sibling is left.
			current = merkleNodeHash(sibling, current)
		}
		idx /= 2
	}

	return current == *root
}

// Represents a Merkle Tree
type Tree []Proof

// emptyLeafHash is the hash of a padding leaf (no data). We precompute it
// because the same value is used repeatedly when the leaf count is not a
// power of two.
var emptyLeafHash = merkleLeafHash(nil)

// New constructs a binary Merkle tree from the given leaf data
// and returns the root hash plus one inclusion proof per original leaf.
//
// The tree is padded to the next power-of-two size with empty leaves. This
// simplifies the proof logic: every node at every level has a sibling, and
// the proof path length is always log2(paddedSize).
//
// Returns a zero root and nil proofs if leaves is empty.
func New(leaves [][]byte) (root Hash, tree Tree) {
	n := len(leaves)
	if n == 0 {
		// todo(rdr): maybe here we return a default merkle tree
		return [32]byte{}, nil
	}

	size := nextPowerOfTwo(n)

	// Build the bottom layer: hash each leaf, pad to power-of-two.
	layer := make([]Hash, size)
	for i := range n {
		layer[i] = merkleLeafHash(leaves[i])
	}
	for i := n; i < size; i++ {
		layer[i] = emptyLeafHash
	}

	// proofSiblings[i] accumulates the sibling hashes for leaf i's proof.
	// We collect them bottom-up as we build the tree.
	proofSiblings := make([][]Hash, n)

	// Build the tree bottom-up, one level at a time.
	for len(layer) > 1 {
		nextLayer := make([]Hash, len(layer)/2)
		for i := 0; i < len(layer); i += 2 {
			left, right := layer[i], layer[i+1]
			nextLayer[i/2] = merkleNodeHash(left, right)

			// Record siblings for any original leaves still tracked at
			// this level. Leaf j at this level has its sibling at j^1
			// (XOR flips the last bit to get the pair partner).
			for j := range n {
				// Which position in the current layer does leaf j's
				// ancestor occupy? It's j >> (current depth), but we
				// track this implicitly: at depth d the ancestor of
				// leaf j is at position j >> d. Since we've already
				// collected d levels of siblings, d == len(proofSiblings[j]).
				d := len(proofSiblings[j])
				ancestorPos := j >> d
				if ancestorPos/2 == i/2 {
					// This pair contains leaf j's ancestor. The sibling
					// is the other element of the pair.
					sibling := ancestorPos ^ 1
					proofSiblings[j] = append(proofSiblings[j], layer[sibling])
				}
			}
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

// Merkle tree construction and verification using a specific SHA-256 tagging
// scheme. Tags prevent second-preimage attacks by domain-separating leaf
// hashes from internal node hashes. The exact tag format matches the Propeller
// protocol specification so that all implementations produce identical trees.
//
// Tree layout: leaves are at the bottom, padded to the next power-of-two
// with the hash of empty data. The tree is built bottom-up by hashing pairs.

// merkleLeafHash computes: SHA256("<leaf>" || data || "</leaf>")
//
// The XML-like tags are the domain separator specified by the Propeller
// protocol. They ensure a leaf hash can never collide with a node hash,
// even if an attacker controls the data.
func merkleLeafHash(data []byte) Hash {
	h := sha256.New()
	h.Write([]byte("<leaf>"))
	h.Write(data)
	h.Write([]byte("</leaf>"))
	var out [32]byte
	h.Sum(out[:0])
	return out
}

// merkleNodeHash computes:
//
//	SHA256("<node><left>" || left || "</left><right>" || right || "</right></node>")
//
// The nested tags ensure node hashes are in a separate domain from leaf hashes.
func merkleNodeHash(left, right [32]byte) Hash {
	h := sha256.New()
	h.Write([]byte("<node><left>"))
	h.Write(left[:])
	h.Write([]byte("</left><right>"))
	h.Write(right[:])
	h.Write([]byte("</right></node>"))
	var out [32]byte
	h.Sum(out[:0])
	return out
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
