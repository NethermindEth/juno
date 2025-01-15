package trie2

import (
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type ProofNodeSet = utils.OrderedSet[felt.Felt, node]

func NewProofNodeSet() *ProofNodeSet {
	return utils.NewOrderedSet[felt.Felt, node]()
}

// Prove generates a Merkle proof for a given key in the trie.
// The result contains the proof nodes on the path from the root to the leaf.
// The value is included in the proof if the key is present in the trie.
// If the key is not present, the proof will contain the nodes on the path to the closest ancestor.
func (t *Trie) Prove(key *felt.Felt, proof *ProofNodeSet) error {
	if t.committed {
		return ErrCommitted
	}

	path := t.FeltToPath(key)
	k := &path

	var (
		prefix *Path
		nodes  []node
		rn     = t.root
	)

	for k.Len() > 0 && rn != nil {
		switch n := rn.(type) {
		case *edgeNode:
			if !n.pathMatches(k) {
				rn = nil // Trie doesn't contain the key
			} else {
				rn = n.child
				prefix.Append(prefix, n.path)
				k.LSBs(k, n.path.Len())
			}
			nodes = append(nodes, n)
		case *binaryNode:
			bit := k.MSB()
			prefix.AppendBit(prefix, bit)
			k.LSBs(k, 1)
			nodes = append(nodes, n)
		case *hashNode:
			resolved, err := t.resolveNode(n, *prefix)
			if err != nil {
				return err
			}
			rn = resolved
		default:
			panic(fmt.Sprintf("unknown node type: %T", n))
		}
	}

	h := newHasher(t.hashFn, false)

	for _, n := range nodes {
		hashed, cached := h.hash(n) // subsequent nodes are cached
		proof.Put(hashed.(*hashNode).Felt, cached)
	}

	return nil
}

// GetRangeProof generates a range proof for the given range of keys.
// The proof contains the proof nodes on the path from the root to the closest ancestor of the left and right keys.
func (t *Trie) GetRangeProof(leftKey, rightKey *felt.Felt, proofSet *ProofNodeSet) error {
	err := t.Prove(leftKey, proofSet)
	if err != nil {
		return err
	}

	// If they are the same key, don't need to generate the proof again
	if leftKey.Equal(rightKey) {
		return nil
	}

	err = t.Prove(rightKey, proofSet)
	if err != nil {
		return err
	}

	return nil
}

// VerifyProof verifies that a proof path is valid for a given key in a binary trie.
// It walks through the proof nodes, verifying each step matches the expected path to reach the key.
//
// The proof is considered invalid if:
//   - Any proof node is missing from the node set
//   - Any node's computed hash doesn't match its expected hash
//   - The path bits don't match the key bits
//   - The proof ends before processing all key bits
func VerifyProof(root, key *felt.Felt, proof *ProofNodeSet, hash crypto.HashFn) (felt.Felt, error) {
	keyBits := new(Path).SetFelt(contractClassTrieHeight, key)
	expected := *root
	h := newHasher(hash, false)

	for {
		node, ok := proof.Get(expected)
		if !ok {
			return felt.Zero, fmt.Errorf("proof node not found, expected hash: %s", expected.String())
		}

		nHash, _ := h.hash(node)

		// Verify the hash matches
		if !nHash.(*hashNode).Felt.Equal(&expected) {
			return felt.Zero, fmt.Errorf("proof node hash mismatch, expected hash: %s, got hash: %s", expected.String(), nHash.String())
		}

		child := get(node, keyBits, false)
		switch cld := child.(type) {
		case nil:
			return felt.Zero, nil
		case *hashNode:
			expected = cld.Felt
		case *valueNode:
			return cld.Felt, nil
		}
	}
}

func get(rn node, key *Path, skipResolved bool) node {
	for {
		switch n := rn.(type) {
		case *edgeNode:
			if !n.pathMatches(key) {
				return nil
			}
			rn = n.child
			key.LSBs(key, n.path.Len())
		case *binaryNode:
			bit := key.MSB()
			rn = n.children[bit]
			key.LSBs(key, 1)
		case *hashNode:
			return n
		case *valueNode:
			return n
		case nil:
			return nil
		}

		if !skipResolved {
			return rn
		}
	}
}
