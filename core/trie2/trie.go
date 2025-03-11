package trie2

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

const contractClassTrieHeight = 251

type Path = trieutils.Path

type Trie struct {
	// Height of the trie
	height uint8

	// The owner of the trie, only used for contract trie. If not empty, this is a storage trie.
	owner felt.Felt

	// The root node of the trie
	root Node

	// Hash function used to hash the trie nodes
	hashFn crypto.HashFn

	// The underlying database to store and retrieve trie nodes
	db *triedb.Database

	// Check if the trie has been committed. Trie is unusable once committed.
	committed bool

	// Maintains the records of trie changes, ensuring all nodes are modified or garbage collected properly
	nodeTracer *nodeTracer

	// Tracks the number of leaves inserted since the last hashing operation
	pendingHashes int

	// Tracks the total number of updates (inserts/deletes) since the last commit
	pendingUpdates int
}

// A unique identifier for a trie
type TrieID interface {
	Bucket() db.Bucket
	Owner() felt.Felt
}

// Creates a new trie
func New(id TrieID, height uint8, hashFn crypto.HashFn, txn db.Transaction) (*Trie, error) {
	database := triedb.New(txn, id.Bucket(), &triedb.Config{PathConfig: &pathdb.Config{}, HashConfig: nil}) // TODO: handle both pathdb and hashdb, default hashdb config for now
	tr := &Trie{
		owner:      id.Owner(),
		height:     height,
		hashFn:     hashFn,
		db:         database,
		nodeTracer: newTracer(),
	}

	root, err := tr.resolveNode(nil, Path{})
	if err == nil {
		tr.root = root
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	return tr, nil
}

// Creates an empty trie, only used for temporary trie construction
func NewEmpty(height uint8, hashFn crypto.HashFn) *Trie {
	return &Trie{
		height:     height,
		hashFn:     hashFn,
		root:       nil,
		nodeTracer: newTracer(),
		db:         triedb.NewEmptyPathDatabase(),
	}
}

// Modifies or inserts a key-value pair in the trie.
// If value is zero, the key is deleted from the trie.
func (t *Trie) Update(key, value *felt.Felt) error {
	if t.committed {
		return ErrCommitted
	}

	if err := t.update(key, value); err != nil {
		return err
	}
	t.pendingUpdates++
	t.pendingHashes++

	return nil
}

// Retrieves the value associated with the given key.
// Returns felt.Zero if the key doesn't exist.
// May update the trie's internal structure if nodes need to be resolved.
func (t *Trie) Get(key *felt.Felt) (felt.Felt, error) {
	if t.committed {
		return felt.Zero, ErrCommitted
	}

	k := t.FeltToPath(key)

	var ret felt.Felt
	val, root, didResolve, err := t.get(t.root, new(Path), &k)
	// In Starknet, a non-existent key is mapped to felt.Zero
	if val == nil {
		ret = felt.Zero
	} else {
		ret = *val
	}

	if err == nil && didResolve {
		t.root = root
	}

	return ret, err
}

// Removes the given key from the trie.
func (t *Trie) Delete(key *felt.Felt) error {
	if t.committed {
		return ErrCommitted
	}

	k := t.FeltToPath(key)
	_, n, err := t.delete(t.root, new(Path), &k)
	if err != nil {
		return err
	}
	t.root = n
	t.pendingUpdates++
	t.pendingHashes++
	return nil
}

// Returns the root hash of the trie. Calling this method will also cache the hash of each node in the trie.
func (t *Trie) Hash() felt.Felt {
	hash, cached := t.hashRoot()
	t.root = cached
	return hash.(*HashNode).Felt
}

// Collapses the trie into a single hash node and flush the node changes to the database.
func (t *Trie) Commit() (felt.Felt, error) {
	defer func() {
		t.committed = true
	}()

	// Trie is empty and can be classified into two types of situations:
	// (a) The trie was empty and no update happens => return empty root
	// (b) The trie was non-empty and all nodes are dropped => commit and return empty root
	if t.root == nil {
		paths := t.nodeTracer.deletedNodes()
		if len(paths) == 0 { // case (a)
			return felt.Zero, nil
		}
		// case (b)
		nodes := trienode.NewNodeSet(t.owner)
		for _, path := range paths {
			nodes.Add(path, trienode.NewDeleted(path.Len() == t.height))
		}
		err := nodes.ForEach(true, func(key trieutils.Path, node trienode.TrieNode) error {
			return t.db.Delete(t.owner, key, node.Hash(), node.IsLeaf())
		})
		return felt.Zero, err
	}

	rootHash := t.Hash()
	if hashedNode, dirty := t.root.cache(); !dirty {
		t.root = hashedNode
		return rootHash, nil
	}

	nodes := trienode.NewNodeSet(t.owner)
	for _, path := range t.nodeTracer.deletedNodes() {
		nodes.Add(path, trienode.NewDeleted(path.Len() == t.height))
	}

	t.root = newCollector(nodes).Collect(t.root, t.pendingUpdates > 100) //nolint:mnd // TODO(weiihann): 100 is arbitrary
	t.pendingUpdates = 0

	err := nodes.ForEach(true, func(key trieutils.Path, node trienode.TrieNode) error {
		if dn, ok := node.(*trienode.DeletedNode); ok {
			return t.db.Delete(t.owner, key, node.Hash(), dn.IsLeaf())
		}
		return t.db.Put(t.owner, key, node.Hash(), node.Blob(), node.IsLeaf())
	})
	if err != nil {
		return felt.Felt{}, err
	}

	return rootHash, nil
}

func (t *Trie) NodeIterator() (db.Iterator, error) {
	return t.db.NewIterator(t.owner)
}

func (t *Trie) Copy() *Trie {
	return &Trie{
		height:         t.height,
		owner:          t.owner,
		root:           t.root,
		hashFn:         t.hashFn,
		committed:      t.committed,
		db:             t.db,
		nodeTracer:     t.nodeTracer.copy(),
		pendingHashes:  t.pendingHashes,
		pendingUpdates: t.pendingUpdates,
	}
}

func (t *Trie) HashFn() crypto.HashFn {
	return t.hashFn
}

// Traverses the trie to find a value associated with a given key.
// It handles different node types:
// - EdgeNode: Checks if the path matches the key, then recursively traverses the child node
// - BinaryNode: Determines which child to follow based on the most significant bit of the key
// - HashNode: Resolves the actual node from the database before continuing traversal
// - ValueNode: Returns the stored value when found
// - nil: Returns nil when no value exists
//
// The method returns four values: the found value (or nil), the possibly updated node,
// a flag indicating if node resolution occurred, and any error encountered.
// When nodes are resolved from the database, the trie structure is updated to cache the resolved nodes.
func (t *Trie) get(n Node, prefix, key *Path) (*felt.Felt, Node, bool, error) {
	switch n := n.(type) {
	case *EdgeNode:
		if !n.pathMatches(key) {
			return nil, n, false, nil
		}
		val, child, didResolve, err := t.get(n.Child, new(Path).Append(prefix, n.Path), key.LSBs(key, n.Path.Len()))
		if err == nil && didResolve {
			n = n.copy()
			n.Child = child
		}
		return val, n, didResolve, err
	case *BinaryNode:
		bit := key.MSB()
		val, child, didResolve, err := t.get(n.Children[bit], new(Path).AppendBit(prefix, bit), key.LSBs(key, 1))
		if err == nil && didResolve {
			n = n.copy()
			n.Children[bit] = child
		}
		return val, n, didResolve, err
	case *HashNode:
		child, err := t.resolveNode(n, *prefix)
		if err != nil {
			return nil, nil, false, err
		}
		value, newNode, _, err := t.get(child, prefix, key)
		return value, newNode, true, err
	case *ValueNode:
		return &n.Felt, n, false, nil
	case nil:
		return nil, nil, false, nil
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

// Modifies the trie by either inserting/updating a value or deleting a key.
// The operation is determined by whether the value is zero (delete) or non-zero (insert/update).
func (t *Trie) update(key, value *felt.Felt) error {
	k := t.FeltToPath(key)
	if value.IsZero() {
		_, n, err := t.delete(t.root, new(Path), &k)
		if err != nil {
			return err
		}
		t.root = n
	} else {
		_, n, err := t.insert(t.root, new(Path), &k, &ValueNode{Felt: *value})
		if err != nil {
			return err
		}
		t.root = n
	}
	return nil
}

// Inserts a value into the trie. Handles different node types:
// - EdgeNode: Creates branch nodes when paths diverge, or updates existing paths
// - BinaryNode: Follows the appropriate child based on the key's MSB
// - HashNode: Resolves the actual node before insertion
// - nil: Creates a new edge or value node depending on key length
// Returns whether the trie was modified, the new/updated node, and any error.
//
//nolint:gocyclo,funlen
func (t *Trie) insert(n Node, prefix, key *Path, value Node) (bool, Node, error) {
	// We reach the end of the key
	if key.Len() == 0 {
		if v, ok := n.(*ValueNode); ok {
			vFelt := value.(*ValueNode).Felt
			return !v.Equal(&vFelt), value, nil
		}
		return true, value, nil
	}

	switch n := n.(type) {
	case *EdgeNode:
		match := n.commonPath(key) // get the matching bits between the current node and the key
		// If the match is the same as the Path, just keep this edge node as it is and update the value
		if match.Len() == n.Path.Len() {
			dirty, newNode, err := t.insert(n.Child, new(Path).Append(prefix, n.Path), key.LSBs(key, match.Len()), value)
			if !dirty || err != nil {
				return false, n, err
			}
			return true, &EdgeNode{
				Path:  n.Path,
				Child: newNode,
				flags: newFlag(),
			}, nil
		}
		// Otherwise branch out at the bit position where they differ
		branch := &BinaryNode{flags: newFlag()}
		var err error
		pathPrefix := new(Path).MSBs(n.Path, match.Len()+1)
		_, branch.Children[n.Path.Bit(match.Len())], err = t.insert(
			nil, new(Path).Append(prefix, pathPrefix), new(Path).LSBs(n.Path, match.Len()+1), n.Child,
		)
		if err != nil {
			return false, n, err
		}

		keyPrefix := new(Path).MSBs(key, match.Len()+1)
		_, branch.Children[key.Bit(match.Len())], err = t.insert(
			nil, new(Path).Append(prefix, keyPrefix), new(Path).LSBs(key, match.Len()+1), value,
		)
		if err != nil {
			return false, n, err
		}

		// Replace this edge node with the new binary node if it occurs at the current MSB
		if match.IsEmpty() {
			return true, branch, nil
		}
		matchPrefix := new(Path).MSBs(key, match.Len())
		t.nodeTracer.onInsert(new(Path).Append(prefix, matchPrefix))

		// Otherwise, create a new edge node with the Path being the common Path and the branch as the child
		return true, &EdgeNode{Path: matchPrefix, Child: branch, flags: newFlag()}, nil

	case *BinaryNode:
		// Go to the child node based on the MSB of the key
		bit := key.MSB()
		dirty, newNode, err := t.insert(
			n.Children[bit], new(Path).AppendBit(prefix, bit), new(Path).LSBs(key, 1), value,
		)
		if !dirty || err != nil {
			return false, n, err
		}
		// Replace the child node with the new node
		n = n.copy()
		n.flags = newFlag()
		n.Children[bit] = newNode
		return true, n, nil
	case nil:
		t.nodeTracer.onInsert(prefix)
		// We reach the end of the key, return the value node
		if key.IsEmpty() {
			return true, value, nil
		}
		// Otherwise, return a new edge node with the Path being the key and the value as the child
		return true, &EdgeNode{Path: key, Child: value, flags: newFlag()}, nil
	case *HashNode:
		child, err := t.resolveNode(n, *prefix)
		if err != nil {
			return false, n, err
		}
		dirty, newNode, err := t.insert(child, prefix, key, value)
		if !dirty || err != nil {
			return false, child, err
		}
		return true, newNode, nil
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

// Deletes a key from the trie. Handles different node types:
// - EdgeNode: Removes the node if path matches, or recursively deletes from child
// - BinaryNode: Follows the appropriate child and may collapse the node if a child is removed
// - HashNode: Resolves the actual node before deletion
// - ValueNode: Removes the value node when found
// - nil: Returns false as there's nothing to delete
// Returns whether the trie was modified, the new/updated node, and any error.
//
//nolint:gocyclo,funlen
func (t *Trie) delete(n Node, prefix, key *Path) (bool, Node, error) {
	switch n := n.(type) {
	case *EdgeNode:
		match := n.commonPath(key)
		// Mismatched, don't do anything
		if match.Len() < n.Path.Len() {
			return false, n, nil
		}
		// If the whole key matches, remove the entire edge node and its child
		if match.Len() == key.Len() {
			t.nodeTracer.onDelete(prefix)                        // delete edge node
			t.nodeTracer.onDelete(new(Path).Append(prefix, key)) // delete value node
			return true, nil, nil
		}

		// Otherwise, key is longer than current node path, so we need to delete the child.
		// Child can never be nil because it's guaranteed that we have at least 2 other values in the subtrie.
		keyPrefix := new(Path).MSBs(key, n.Path.Len())
		dirty, child, err := t.delete(n.Child, new(Path).Append(prefix, keyPrefix), new(Path).LSBs(key, n.Path.Len()))
		if !dirty || err != nil {
			return false, n, err
		}
		switch child := child.(type) {
		case *EdgeNode:
			t.nodeTracer.onDelete(new(Path).Append(prefix, n.Path))
			return true, &EdgeNode{Path: new(Path).Append(n.Path, child.Path), Child: child.Child, flags: newFlag()}, nil
		default:
			return true, &EdgeNode{Path: new(Path).Set(n.Path), Child: child, flags: newFlag()}, nil
		}
	case *BinaryNode:
		bit := key.MSB()
		keyPrefix := new(Path).MSBs(key, 1)
		dirty, newNode, err := t.delete(n.Children[bit], new(Path).Append(prefix, keyPrefix), key.LSBs(key, 1))
		if !dirty || err != nil {
			return false, n, err
		}
		n = n.copy()
		n.flags = newFlag()
		n.Children[bit] = newNode

		// If the child node is not nil, that means we still have 2 children in this binary node
		if newNode != nil {
			return true, n, nil
		}

		// Otherwise, we need to combine this binary node with the other child
		// If it's a hash node, we need to resolve it first.
		// If the other child is an edge node, we prepend the bit prefix to the other child path
		other := bit ^ 1
		bitPrefix := new(Path).SetBit(other)

		if hn, ok := n.Children[other].(*HashNode); ok {
			var cPath Path
			cPath.Append(prefix, bitPrefix)
			cNode, err := t.resolveNode(hn, cPath)
			if err != nil {
				return false, nil, err
			}
			n.Children[other] = cNode
		}

		if cn, ok := n.Children[other].(*EdgeNode); ok {
			t.nodeTracer.onDelete(new(Path).Append(prefix, bitPrefix))
			return true, &EdgeNode{
				Path:  new(Path).Append(bitPrefix, cn.Path),
				Child: cn.Child,
				flags: newFlag(),
			}, nil
		}

		// other child is not an edge node, create a new edge node with the bit prefix as the Path
		// containing the other child as the child
		return true, &EdgeNode{Path: bitPrefix, Child: n.Children[other], flags: newFlag()}, nil
	case *ValueNode:
		t.nodeTracer.onDelete(key)
		return true, nil, nil
	case nil:
		return false, nil, nil
	case *HashNode:
		child, err := t.resolveNode(n, *prefix)
		if err != nil {
			return false, nil, err
		}

		dirty, newNode, err := t.delete(child, prefix, key)
		if !dirty || err != nil {
			return false, child, err
		}
		return true, newNode, nil
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

// Resolves the node at the given path from the database
func (t *Trie) resolveNode(hn *HashNode, path Path) (Node, error) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	var hashNodeHash *felt.Felt
	if hn != nil {
		hashNodeHash = hn.Hash(t.hashFn)
	} else {
		hashNodeHash = new(felt.Felt).SetUint64(0)
	}
	_, err := t.db.Get(buf, t.owner, path, *hashNodeHash, path.Len() == t.height)
	if err != nil {
		return nil, err
	}

	blob := buf.Bytes()

	var hash *felt.Felt
	if hn != nil {
		hash = &hn.Felt
	}
	return decodeNode(blob, hash, path.Len(), t.height)
}

// Calculates the hash of the root node
func (t *Trie) hashRoot() (Node, Node) {
	if t.root == nil {
		return &HashNode{Felt: felt.Zero}, nil
	}
	h := newHasher(t.hashFn, t.pendingHashes > 100) //nolint:mnd //TODO(weiihann): 100 is arbitrary
	hashed, cached := h.hash(t.root)
	t.pendingHashes = 0
	return hashed, cached
}

// Converts a Felt value into a Path representation suitable to
// use as a trie key with the specified height.
func (t *Trie) FeltToPath(f *felt.Felt) Path {
	var key Path
	key.SetFelt(t.height, f)
	return key
}

func (t *Trie) String() string {
	if t.root == nil {
		return ""
	}
	return t.root.String()
}

func NewEmptyPedersen() (*Trie, error) {
	return New(NewEmptyTrieID(), contractClassTrieHeight, crypto.Pedersen, db.NewMemTransaction())
}

func NewEmptyPoseidon() (*Trie, error) {
	return New(NewEmptyTrieID(), contractClassTrieHeight, crypto.Poseidon, db.NewMemTransaction())
}

func RunOnTempTriePedersen(height uint8, do func(*Trie) error) error {
	trie, err := New(NewEmptyTrieID(), height, crypto.Pedersen, db.NewMemTransaction())
	if err != nil {
		return err
	}
	return do(trie)
}

func RunOnTempTriePoseidon(height uint8, do func(*Trie) error) error {
	trie, err := New(NewEmptyTrieID(), height, crypto.Poseidon, db.NewMemTransaction())
	if err != nil {
		return err
	}
	return do(trie)
}
