package trie2

import (
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

type Trie struct {
	height uint8
	root   node
	reader interface{} // TODO(weiihann): implement reader
	// committed bool
	hashFn crypto.HashFn
}

// TODO(weiihann): implement this
func NewTrie(height uint8, hashFn crypto.HashFn) *Trie {
	return &Trie{height: height, hashFn: hashFn}
}

// Modifies or inserts a key-value pair in the trie.
// If value is zero, the key is deleted from the trie.
func (t *Trie) Update(key, value *felt.Felt) error {
	// if t.commited {
	// 	return ErrCommitted
	// }
	return t.update(key, value)
}

// Retrieves the value associated with the given key.
// Returns felt.Zero if the key doesn't exist.
// May update the trie's internal structure if nodes need to be resolved.
func (t *Trie) Get(key *felt.Felt) (*felt.Felt, error) {
	k := t.FeltToKey(key)
	// TODO(weiihann): get the value directly from the reader
	val, root, didResolve, err := t.get(t.root, &k)
	// In Starknet, a non-existent key is mapped to felt.Zero
	if val == nil {
		val = &felt.Zero
	}
	if err == nil && didResolve {
		t.root = root
	}
	return val, err
}

// Removes the given key from the trie.
func (t *Trie) Delete(key *felt.Felt) error {
	k := t.FeltToKey(key)
	_, n, err := t.delete(t.root, new(BitArray), &k)
	if err != nil {
		return err
	}
	t.root = n
	return nil
}

// Returns the root hash of the trie. Calling this method will also cache the hash of each node in the trie.
func (t *Trie) Hash() *felt.Felt {
	hash, cached := t.hashRoot()
	t.root = cached
	return hash.(*hashNode).Felt
}

func (t *Trie) get(n node, key *BitArray) (*felt.Felt, node, bool, error) {
	switch n := n.(type) {
	case *edgeNode:
		if !n.pathMatches(key) {
			return nil, nil, false, nil
		}
		val, child, didResolve, err := t.get(n.child, key.LSBs(key, n.path.Len()))
		if err == nil && didResolve {
			n = n.copy()
			n.child = child
		}
		return val, n, didResolve, err
	case *binaryNode:
		bit := key.MSB()
		val, child, didResolve, err := t.get(n.children[bit], key.LSBs(key, 1))
		if err == nil && didResolve {
			n = n.copy()
			n.children[bit] = child
		}
		return val, n, didResolve, err
	case hashNode:
		panic("TODO(weiihann): implement me")
	case valueNode:
		return n.Felt, n, false, nil
	case nil:
		return nil, nil, false, nil
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

// Modifies the trie by either inserting/updating a value or deleting a key.
// The operation is determined by whether the value is zero (delete) or non-zero (insert/update).
func (t *Trie) update(key, value *felt.Felt) error {
	k := t.FeltToKey(key)
	if value.IsZero() {
		_, n, err := t.delete(t.root, new(BitArray), &k)
		if err != nil {
			return err
		}
		t.root = n
	} else {
		_, n, err := t.insert(t.root, &k, valueNode{Felt: value})
		if err != nil {
			return err
		}
		t.root = n
	}
	return nil
}

func (t *Trie) insert(n node, key *BitArray, value node) (bool, node, error) {
	// We reach the end of the key
	if key.Len() == 0 {
		if v, ok := n.(valueNode); ok {
			return v.Equal(value.(valueNode).Felt), value, nil
		}
		return true, value, nil
	}

	switch n := n.(type) {
	case *edgeNode:
		match := n.commonPath(key) // get the matching bits between the current node and the key
		// If the match is the same as the path, just keep this edge node as it is and update the value
		if match.Len() == n.path.Len() {
			dirty, newNode, err := t.insert(n.child, key.LSBs(key, match.Len()), value)
			if !dirty || err != nil {
				return false, n, err
			}
			return true, &edgeNode{
				path:  n.path,
				child: newNode,
				flags: newFlag(),
			}, nil
		}
		// Otherwise branch out at the bit position where they differ
		branch := &binaryNode{flags: newFlag()}
		var err error
		_, branch.children[n.path.Bit(match.Len())], err = t.insert(nil, new(BitArray).LSBs(n.path, match.Len()+1), n.child)
		if err != nil {
			return false, n, err
		}

		_, branch.children[key.Bit(match.Len())], err = t.insert(nil, new(BitArray).LSBs(key, match.Len()+1), value)
		if err != nil {
			return false, n, err
		}

		// Replace this edge node with the new binary node if it occurs at the current MSB
		if match.IsEmpty() {
			return true, branch, nil
		}

		// Otherwise, create a new edge node with the path being the common path and the branch as the child
		return true, &edgeNode{path: new(BitArray).MSBs(key, match.Len()), child: branch, flags: newFlag()}, nil

	case *binaryNode:
		// Go to the child node based on the MSB of the key
		bit := key.MSB()
		dirty, newNode, err := t.insert(n.children[bit], new(BitArray).LSBs(key, 1), value)
		if !dirty || err != nil {
			return false, n, err
		}
		// Replace the child node with the new node
		n = n.copy()
		n.flags = newFlag()
		n.children[bit] = newNode
		return true, n, nil
	case nil:
		// We reach the end of the key, return the value node
		if key.IsEmpty() {
			return true, value, nil
		}
		// Otherwise, return a new edge node with the path being the key and the value as the child
		return true, &edgeNode{path: key, child: value, flags: newFlag()}, nil
	case hashNode:
		panic("TODO(weiihann): implement me")
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

func (t *Trie) delete(n node, prefix, key *BitArray) (bool, node, error) {
	switch n := n.(type) {
	case *edgeNode:
		match := n.commonPath(key)
		// Mismatched, don't do anything
		if match.Len() < n.path.Len() {
			return false, n, nil
		}
		// If the whole key matches, remove the entire edge node
		if match.Len() == key.Len() {
			return true, nil, nil
		}

		// Otherwise, key is longer than current node path, so we need to delete the child.
		// Child can never be nil because it's guaranteed that we have at least 2 other values in the subtrie.
		keyPrefix := new(BitArray).MSBs(key, n.path.Len())
		dirty, child, err := t.delete(n.child, new(BitArray).Append(prefix, keyPrefix), key.LSBs(key, n.path.Len()))
		if !dirty || err != nil {
			return false, n, err
		}
		switch child := child.(type) {
		case *edgeNode:
			return true, &edgeNode{path: new(BitArray).Append(n.path, child.path), child: child.child, flags: newFlag()}, nil
		default:
			return true, &edgeNode{path: new(BitArray).Set(n.path), child: child, flags: newFlag()}, nil
		}
	case *binaryNode:
		bit := key.MSB()
		keyPrefix := new(BitArray).MSBs(key, 1)
		dirty, newNode, err := t.delete(n.children[bit], new(BitArray).Append(prefix, keyPrefix), key.LSBs(key, 1))
		if !dirty || err != nil {
			return false, n, err
		}
		n = n.copy()
		n.flags = newFlag()
		n.children[bit] = newNode

		// If the child node is not nil, that means we still have 2 children in this binary node
		if newNode != nil {
			return true, n, nil
		}

		// Otherwise, we need to combine this binary node with the other child
		other := bit ^ 1
		bitPrefix := new(BitArray).SetBit(other == 1)
		if cn, ok := n.children[other].(*edgeNode); ok { // other child is an edge node, append the bit prefix to the child path
			return true, &edgeNode{
				path:  new(BitArray).Append(bitPrefix, cn.path),
				child: cn.child,
				flags: newFlag(),
			}, nil
		}

		// other child is not an edge node, create a new edge node with the bit prefix as the path
		// containing the other child as the child
		return true, &edgeNode{path: bitPrefix, child: n.children[other], flags: newFlag()}, nil
	case valueNode:
		return true, nil, nil
	case nil:
		return false, nil, nil
	case hashNode:
		panic("TODO(weiihann): implement me")
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

func (t *Trie) hashRoot() (node, node) {
	h := newHasher(t.hashFn, false) // TODO(weiihann): handle parallel hashing
	return h.hash(t.root)
}

// Converts a Felt value into a BitArray representation suitable for
// use as a trie key with the specified height.
func (t *Trie) FeltToKey(f *felt.Felt) BitArray {
	var key BitArray
	key.SetFelt(t.height, f)
	return key
}

func (t *Trie) String() string {
	if t.root == nil {
		return ""
	}
	return t.root.String()
}
