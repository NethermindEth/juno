package trie2

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
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
	root trienode.Node

	// Hash function used to hash the trie nodes
	hashFn crypto.HashFn

	// The underlying reader to retrieve trie nodes
	nodeReader nodeReader

	// Check if the trie has been committed. Trie is unusable once committed.
	committed bool

	// Maintains the records of trie changes, ensuring all nodes are modified or garbage collected properly
	nodeTracer nodeTracer

	// Tracks the number of leaves inserted since the last hashing operation
	pendingHashes int

	// Tracks the total number of updates (inserts/deletes) since the last commit
	pendingUpdates int
}

// Creates a new trie, with the arguments:
// - id: the trie id
// - height: the height of the trie, 251 for contract and class trie
// - hashFn: the hash function to use
// - nodeDB: database interface, which provides methods to read and write trie nodes
func New(
	id trieutils.TrieID,
	height uint8,
	hashFn crypto.HashFn,
	nodeDB database.NodeDatabase,
) (*Trie, error) {
	nodeReader, err := newNodeReader(id, nodeDB)
	if err != nil {
		return nil, err
	}

	tr := &Trie{
		owner:      id.Owner(),
		height:     height,
		hashFn:     hashFn,
		nodeReader: nodeReader,
		nodeTracer: newTracer(),
	}

	stateComm := id.StateComm()
	if stateComm.IsZero() {
		return tr, nil
	}

	root, err := tr.resolveNode(nil, Path{})
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}
	tr.root = root

	return tr, nil
}

// Similar to New, creates a new trie, but additionally takes one more argument:
// - rootHash: the root hash of the trie (provided only for hash scheme of trieDB, needed to properly recreate the trie)
func NewFromRootHash(
	id trieutils.TrieID,
	height uint8,
	hashFn crypto.HashFn,
	nodeDB database.NodeDatabase,
	rootHash *felt.Felt,
) (*Trie, error) {
	nodeReader, err := newNodeReader(id, nodeDB)
	if err != nil {
		return nil, err
	}

	tr := &Trie{
		owner:      id.Owner(),
		height:     height,
		hashFn:     hashFn,
		nodeReader: nodeReader,
		nodeTracer: newTracer(),
	}

	stateComm := id.StateComm()
	if stateComm.IsZero() {
		return tr, nil
	}

	root, err := tr.resolveNodeWithHash(new(Path), rootHash)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}
	tr.root = root

	return tr, nil
}

// Creates an empty trie, only used for temporary trie construction
func NewEmpty(height uint8, hashFn crypto.HashFn) *Trie {
	return &Trie{
		height:     height,
		hashFn:     hashFn,
		root:       nil,
		nodeTracer: newTracer(),
		nodeReader: NewEmptyNodeReader(),
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
	n, _, err := t.delete(t.root, new(Path), &k)
	if err != nil {
		return err
	}
	t.root = n
	t.pendingUpdates++
	t.pendingHashes++
	return nil
}

// Returns the root hash of the trie. Calling this method will also cache the hash of each node in the trie.
func (t *Trie) Hash() (felt.Felt, error) {
	hash, cached := t.hashRoot()
	t.root = cached
	return felt.Felt(*hash.(*trienode.HashNode)), nil
}

// Collapses the trie into a single hash node and flush the node changes to the database.
func (t *Trie) Commit() (felt.Felt, *trienode.NodeSet) {
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
			nodes.Add(&path, trienode.NewDeleted(path.Len() == t.height))
		}
		return felt.Zero, &nodes
	}

	// If the root node is not dirty, that means we don't actually need to commit
	rootHash, _ := t.Hash()
	if hashedNode, dirty := t.root.Cache(); !dirty {
		t.root = hashedNode
		return rootHash, nil
	}

	nodes := trienode.NewNodeSet(t.owner)
	for _, path := range t.nodeTracer.deletedNodes() {
		nodes.Add(&path, trienode.NewDeleted(path.Len() == t.height))
	}

	t.root = newCollector(&nodes).Collect(t.root, t.pendingUpdates > 100) //nolint:mnd // TODO(weiihann): 100 is arbitrary
	t.pendingUpdates = 0
	return rootHash, &nodes
}

func (t *Trie) Copy() *Trie {
	return &Trie{
		height:         t.height,
		owner:          t.owner,
		root:           t.root,
		hashFn:         t.hashFn,
		committed:      t.committed,
		nodeReader:     t.nodeReader,
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
func (t *Trie) get(n trienode.Node, prefix, key *Path) (*felt.Felt, trienode.Node, bool, error) {
	switch n := n.(type) {
	case *trienode.EdgeNode:
		if !n.PathMatches(key) {
			return nil, n, false, nil
		}
		val, child, didResolve, err := t.get(n.Child, new(Path).Append(prefix, n.Path), key.LSBs(key, n.Path.Len()))
		if err == nil && didResolve {
			n = n.Copy()
			n.Child = child
		}
		return val, n, didResolve, err
	case *trienode.BinaryNode:
		bit := key.MSB()
		val, child, didResolve, err := t.get(n.Children[bit], new(Path).AppendBit(prefix, bit), key.LSBs(key, 1))
		if err == nil && didResolve {
			n = n.Copy()
			n.Children[bit] = child
		}
		return val, n, didResolve, err
	case *trienode.HashNode:
		child, err := t.resolveNode(n, *prefix)
		if err != nil {
			return nil, nil, false, err
		}
		value, newNode, _, err := t.get(child, prefix, key)
		return value, newNode, true, err
	case *trienode.ValueNode:
		return (*felt.Felt)(n), n, false, nil
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
		n, _, err := t.delete(t.root, new(Path), &k)
		if err != nil {
			return err
		}
		t.root = n
	} else {
		n, _, err := t.insert(t.root, new(Path), &k, (*trienode.ValueNode)(value))
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
func (t *Trie) insert(n trienode.Node, prefix, key *Path, value trienode.Node) (trienode.Node, bool, error) {
	// We reach the end of the key
	if key.Len() == 0 {
		if v, ok := n.(*trienode.ValueNode); ok {
			vFelt := felt.Felt(*value.(*trienode.ValueNode))
			vVal := felt.Felt(*v)
			return value, !vVal.Equal(&vFelt), nil
		}
		return value, true, nil
	}

	switch n := n.(type) {
	case nil:
		t.nodeTracer.onInsert(prefix)
		// We reach the end of the key, return the value node
		if key.IsEmpty() {
			return value, true, nil
		}
		// Otherwise, return a new edge node with the Path being the key and the value as the child
		return &trienode.EdgeNode{Path: key, Child: value, Flags: trienode.NewNodeFlag()}, true, nil
	case *trienode.EdgeNode:
		match := n.CommonPath(key) // get the matching bits between the current node and the key
		// If the match is the same as the Path, just keep this edge node as it is and update the value
		if match.Len() == n.Path.Len() {
			newNode, dirty, err := t.insert(n.Child, new(Path).Append(prefix, n.Path), key.LSBs(key, match.Len()), value)
			if !dirty || err != nil {
				return n, false, err
			}
			return &trienode.EdgeNode{
				Path:  n.Path,
				Child: newNode,
				Flags: trienode.NewNodeFlag(),
			}, true, nil
		}
		// Otherwise branch out at the bit position where they differ
		branch := &trienode.BinaryNode{Flags: trienode.NewNodeFlag()}
		var err error
		pathPrefix := new(Path).MSBs(n.Path, match.Len()+1)
		branch.Children[n.Path.Bit(match.Len())], _, err = t.insert(
			nil, new(Path).Append(prefix, pathPrefix), new(Path).LSBs(n.Path, match.Len()+1), n.Child,
		)
		if err != nil {
			return n, false, err
		}

		keyPrefix := new(Path).MSBs(key, match.Len()+1)
		branch.Children[key.Bit(match.Len())], _, err = t.insert(
			nil, new(Path).Append(prefix, keyPrefix), new(Path).LSBs(key, match.Len()+1), value,
		)
		if err != nil {
			return n, false, err
		}

		// Replace this edge node with the new binary node if it occurs at the current MSB
		if match.IsEmpty() {
			return branch, true, nil
		}
		matchPrefix := new(Path).MSBs(key, match.Len())
		t.nodeTracer.onInsert(new(Path).Append(prefix, matchPrefix))

		// Otherwise, create a new edge node with the Path being the common Path and the branch as the child
		return &trienode.EdgeNode{Path: matchPrefix, Child: branch, Flags: trienode.NewNodeFlag()}, true, nil

	case *trienode.BinaryNode:
		// Go to the child node based on the MSB of the key
		bit := key.MSB()
		newNode, dirty, err := t.insert(
			n.Children[bit], new(Path).AppendBit(prefix, bit), new(Path).LSBs(key, 1), value,
		)
		if !dirty || err != nil {
			return n, false, err
		}
		// Replace the child node with the new node
		n = n.Copy()
		n.Flags = trienode.NewNodeFlag()
		n.Children[bit] = newNode
		return n, true, nil
	case *trienode.HashNode:
		child, err := t.resolveNode(n, *prefix)
		if err != nil {
			return n, false, err
		}
		newNode, dirty, err := t.insert(child, prefix, key, value)
		if !dirty || err != nil {
			return child, false, err
		}
		return newNode, true, nil
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
func (t *Trie) delete(n trienode.Node, prefix, key *Path) (trienode.Node, bool, error) {
	switch n := n.(type) {
	case nil:
		return nil, false, nil
	case *trienode.EdgeNode:
		match := n.CommonPath(key)
		// Mismatched, don't do anything
		if match.Len() < n.Path.Len() {
			return n, false, nil
		}
		// If the whole key matches, remove the entire edge node and its child
		if match.Len() == key.Len() {
			t.nodeTracer.onDelete(prefix)                        // delete edge node
			t.nodeTracer.onDelete(new(Path).Append(prefix, key)) // delete value node
			return nil, true, nil
		}

		// Otherwise, key is longer than current node path, so we need to delete the child.
		// Child can never be nil because it's guaranteed that we have at least 2 other values in the subtrie.
		keyPrefix := new(Path).MSBs(key, n.Path.Len())
		child, dirty, err := t.delete(n.Child, new(Path).Append(prefix, keyPrefix), new(Path).LSBs(key, n.Path.Len()))
		if !dirty || err != nil {
			return n, false, err
		}
		switch child := child.(type) {
		case *trienode.EdgeNode:
			t.nodeTracer.onDelete(new(Path).Append(prefix, n.Path))
			return &trienode.EdgeNode{Path: new(Path).Append(n.Path, child.Path), Child: child.Child, Flags: trienode.NewNodeFlag()}, true, nil
		default:
			return &trienode.EdgeNode{Path: new(Path).Set(n.Path), Child: child, Flags: trienode.NewNodeFlag()}, true, nil
		}
	case *trienode.BinaryNode:
		bit := key.MSB()
		keyPrefix := new(Path).MSBs(key, 1)
		newNode, dirty, err := t.delete(n.Children[bit], new(Path).Append(prefix, keyPrefix), key.LSBs(key, 1))
		if !dirty || err != nil {
			return n, false, err
		}
		n = n.Copy()
		n.Flags = trienode.NewNodeFlag()
		n.Children[bit] = newNode

		// If the child node is not nil, that means we still have 2 children in this binary node
		if newNode != nil {
			return n, true, nil
		}

		// Otherwise, we need to combine this binary node with the other child
		// If it's a hash node, we need to resolve it first.
		// If the other child is an edge node, we prepend the bit prefix to the other child path
		other := bit ^ 1
		bitPrefix := new(Path).SetBit(other)

		if hn, ok := n.Children[other].(*trienode.HashNode); ok {
			var cPath Path
			cPath.Append(prefix, bitPrefix)
			cNode, err := t.resolveNode(hn, cPath)
			if err != nil {
				return nil, false, err
			}
			n.Children[other] = cNode
		}

		if cn, ok := n.Children[other].(*trienode.EdgeNode); ok {
			t.nodeTracer.onDelete(new(Path).Append(prefix, bitPrefix))
			return &trienode.EdgeNode{
				Path:  new(Path).Append(bitPrefix, cn.Path),
				Child: cn.Child,
				Flags: trienode.NewNodeFlag(),
			}, true, nil
		}

		// other child is not an edge node, create a new edge node with the bit prefix as the Path
		// containing the other child as the child
		return &trienode.EdgeNode{Path: bitPrefix, Child: n.Children[other], Flags: trienode.NewNodeFlag()}, true, nil
	case *trienode.ValueNode:
		t.nodeTracer.onDelete(key)
		return nil, true, nil
	case *trienode.HashNode:
		child, err := t.resolveNode(n, *prefix)
		if err != nil {
			return nil, false, err
		}

		newNode, dirty, err := t.delete(child, prefix, key)
		if !dirty || err != nil {
			return child, false, err
		}
		return newNode, true, nil
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

// Resolves the node at the given path from the database
func (t *Trie) resolveNode(hn *trienode.HashNode, path Path) (trienode.Node, error) {
	var hash felt.Felt
	if hn != nil {
		hash = felt.Felt(*hn)
	}

	blob, err := t.nodeReader.node(path, &hash, path.Len() == t.height)
	if err != nil {
		return nil, err
	}

	return trienode.DecodeNode(blob, &hash, path.Len(), t.height)
}

// Resolves the node at the given path from the database
func (t *Trie) resolveNodeWithHash(path *Path, hash *felt.Felt) (trienode.Node, error) {
	isLeaf := path.Len() == t.height
	blob, err := t.nodeReader.node(*path, hash, isLeaf)
	if err != nil {
		return nil, err
	}

	return trienode.DecodeNode(blob, hash, path.Len(), t.height)
}

// Calculates the hash of the root node
func (t *Trie) hashRoot() (trienode.Node, trienode.Node) {
	if t.root == nil {
		zero := felt.Zero
		return (*trienode.HashNode)(&zero), nil
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
	return New(trieutils.NewEmptyTrieID(felt.Zero), contractClassTrieHeight, crypto.Pedersen, triedb.NewEmptyNodeDatabase())
}

func NewEmptyPoseidon() (*Trie, error) {
	return New(trieutils.NewEmptyTrieID(felt.Zero), contractClassTrieHeight, crypto.Poseidon, triedb.NewEmptyNodeDatabase())
}

func RunOnTempTriePedersen(height uint8, do func(*Trie) error) error {
	trie, err := New(trieutils.NewEmptyTrieID(felt.Zero), height, crypto.Pedersen, triedb.NewEmptyNodeDatabase())
	if err != nil {
		return err
	}
	return do(trie)
}

func RunOnTempTriePoseidon(height uint8, do func(*Trie) error) error {
	trie, err := New(trieutils.NewEmptyTrieID(felt.Zero), height, crypto.Poseidon, triedb.NewEmptyNodeDatabase())
	if err != nil {
		return err
	}
	return do(trie)
}
