package trie2

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

type Trie struct {
	height uint8
	root   node
	reader interface{} // TODO(weiihann): implement reader
	// committed bool
}

// TODO(weiihann): implement this
func NewTrie(height uint8) *Trie {
	return &Trie{height: height}
}

func (t *Trie) Update(key, value *felt.Felt) error {
	// if t.commited {
	// 	return ErrCommitted
	// }
	return t.update(key, value)
}

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

func (t *Trie) Delete(key *felt.Felt) error {
	panic("TODO(weiihann): implement me")
}

// Traverses the trie recursively to find the value that corresponds to the key.
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

func (t *Trie) update(key, value *felt.Felt) error {
	k := t.FeltToKey(key)
	if value.IsZero() {
		_, n, err := t.delete(t.root, &k)
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
		match := n.commonPath(key)
		// If the whole key matches, just keep this edge node as it is and update the value
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
		// Otherwise branch out at the bit index where they differ
		branch := &binaryNode{flags: newFlag()}
		var err error
		_, branch.children[n.path.BitSet(match.Len())], err = t.insert(nil, new(BitArray).LSBs(n.path, match.Len()+1), n.child)
		if err != nil {
			return false, n, err
		}

		_, branch.children[key.BitSet(match.Len())], err = t.insert(nil, new(BitArray).LSBs(key, match.Len()+1), value)
		if err != nil {
			return false, n, err
		}

		// Replace this edge node with the new binary node if it occurs at the current MSB
		if match.IsEmpty() {
			return true, branch, nil
		}

		return true, &edgeNode{path: new(BitArray).MSBs(key, match.Len()), child: branch, flags: newFlag()}, nil

	case *binaryNode:
		bit := key.MSB()
		dirty, newNode, err := t.insert(n.children[bit], new(BitArray).LSBs(key, 1), value)
		if !dirty || err != nil {
			return false, n, err
		}
		n = n.copy()
		n.flags = newFlag()
		n.children[bit] = newNode
		return true, n, nil
	case nil:
		if key.IsEmpty() {
			return true, value, nil
		}
		return true, &edgeNode{path: key, child: value, flags: newFlag()}, nil
	case hashNode:
		panic("TODO(weiihann): implement me")
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

func (t *Trie) delete(n node, key *BitArray) (bool, node, error) {
	switch n := n.(type) {
	case *edgeNode:
		match := n.commonPath(key)
		// Mismatched, don't do anything
		if match.Len() < n.path.Len() {
			return false, n, nil
		}
		// If the whole key matches, just delete the edge node
		if match.Len() == key.Len() {
			return true, nil, nil
		}

		// Otherwise, we need to delete the child node
		dirty, child, err := t.delete(n.child, key.LSBs(key, match.Len()))
		if !dirty || err != nil {
			return false, n, err
		}
		switch child := child.(type) {
		case *edgeNode:
			return true, &edgeNode{path: n.path, child: child.child, flags: newFlag()}, nil
		default:
			return true, &edgeNode{path: n.path, child: child, flags: newFlag()}, nil
		}
	case *binaryNode:
		bit := key.MSB()
		dirty, newNode, err := t.delete(n.children[bit], key.LSBs(key, 1))
		if !dirty || err != nil {
			return false, n, err
		}
		n = n.copy()
		n.flags = newFlag()
		n.children[bit] = newNode

		if newNode != nil {
			return true, n, nil
		}

		// TODO(weiihann): combine this binary node with the child

		return true, n, nil
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

func (t *Trie) String() string {
	if t.root == nil {
		return ""
	}
	return t.root.String()
}

func (t *Trie) FeltToKey(f *felt.Felt) BitArray {
	var key BitArray
	key.SetFelt(t.height, f)
	return key
}
