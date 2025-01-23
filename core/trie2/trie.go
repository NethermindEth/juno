package trie2

import (
	"bytes"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

const contractClassTrieHeight = 251

var emptyRoot = felt.Felt{}

type Path = trieutils.BitArray

type Trie struct {
	// Height of the trie
	height uint8

	// The owner of the trie, only used for contract trie. If not empty, this is a storage trie.
	owner felt.Felt

	// The root node of the trie
	root node

	// Hash function used to hash the trie nodes
	hashFn crypto.HashFn

	// The underlying database to store and retrieve trie nodes
	db triedb.TrieDB

	// Check if the trie has been committed. Trie is unusable once committed.
	committed bool

	// Maintains the records of trie changes, ensuring all nodes are modified or garbage collected properly
	nodeTracer *nodeTracer

	// Tracks the number of leaves inserted since the last hashing operation
	pendingHashes int

	// Tracks the total number of updates (inserts/deletes) since the last commit
	pendingUpdates int
}

func New(id *ID, height uint8, hashFn crypto.HashFn, txn db.Transaction) (*Trie, error) {
	database := triedb.New(txn, id.Bucket())
	tr := &Trie{
		owner:      id.Owner,
		height:     height,
		hashFn:     hashFn,
		db:         database,
		nodeTracer: newTracer(),
	}

	if id.Root != emptyRoot {
		root, err := tr.resolveNode(&hashNode{Felt: id.Root}, Path{})
		if err != nil {
			return nil, err
		}
		tr.root = root
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
		db:         triedb.EmptyDatabase{},
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
// TODO(weiihann):
// The State should keep track of the modified key and values, so that we can avoid traversing the trie
// No action needed for the Trie, remove this once State provides the functionality
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
	return hash.(*hashNode).Felt
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
			nodes.Add(path, trienode.NewDeleted())
		}
		err := nodes.ForEach(true, func(key trieutils.BitArray, node *trienode.Node) error {
			return t.db.Delete(t.owner, key)
		})
		return felt.Zero, err
	}

	rootHash := t.Hash()
	if hashedNode, dirty := t.root.cache(); !dirty {
		t.root = hashedNode
		return rootHash, nil
	}

	nodes := trienode.NewNodeSet(t.owner)
	for _, Path := range t.nodeTracer.deletedNodes() {
		nodes.Add(Path, trienode.NewDeleted())
	}

	t.root = newCollector(nodes).Collect(t.root, t.pendingUpdates > 100) //nolint:mnd // TODO(weiihann): 100 is arbitrary
	t.pendingUpdates = 0

	err := nodes.ForEach(true, func(key trieutils.BitArray, node *trienode.Node) error {
		if node.IsDeleted() {
			return t.db.Delete(t.owner, key)
		}
		return t.db.Put(t.owner, key, node.Blob())
	})
	if err != nil {
		return felt.Felt{}, err
	}

	return rootHash, nil
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

func (t *Trie) get(n node, prefix, key *Path) (*felt.Felt, node, bool, error) {
	switch n := n.(type) {
	case *edgeNode:
		if !n.pathMatches(key) {
			return nil, n, false, nil
		}
		val, child, didResolve, err := t.get(n.child, new(Path).Append(prefix, n.path), key.LSBs(key, n.path.Len()))
		if err == nil && didResolve {
			n = n.copy()
			n.child = child
		}
		return val, n, didResolve, err
	case *binaryNode:
		bit := key.MSB()
		val, child, didResolve, err := t.get(n.children[bit], new(Path).AppendBit(prefix, bit), key.LSBs(key, 1))
		if err == nil && didResolve {
			n = n.copy()
			n.children[bit] = child
		}
		return val, n, didResolve, err
	case *hashNode:
		child, err := t.resolveNode(n, *prefix)
		if err != nil {
			return nil, nil, false, err
		}
		value, newNode, _, err := t.get(child, prefix, key)
		return value, newNode, true, err
	case *valueNode:
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
		_, n, err := t.insert(t.root, new(Path), &k, &valueNode{Felt: *value})
		if err != nil {
			return err
		}
		t.root = n
	}
	return nil
}

//nolint:gocyclo,funlen
func (t *Trie) insert(n node, prefix, key *Path, value node) (bool, node, error) {
	// We reach the end of the key
	if key.Len() == 0 {
		if v, ok := n.(*valueNode); ok {
			vFelt := value.(*valueNode).Felt
			return !v.Equal(&vFelt), value, nil
		}
		return true, value, nil
	}

	switch n := n.(type) {
	case *edgeNode:
		match := n.commonPath(key) // get the matching bits between the current node and the key
		// If the match is the same as the Path, just keep this edge node as it is and update the value
		if match.Len() == n.path.Len() {
			dirty, newNode, err := t.insert(n.child, new(Path).Append(prefix, n.path), key.LSBs(key, match.Len()), value)
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
		pathPrefix := new(Path).MSBs(n.path, match.Len()+1)
		_, branch.children[n.path.Bit(match.Len())], err = t.insert(
			nil, new(Path).Append(prefix, pathPrefix), new(Path).LSBs(n.path, match.Len()+1), n.child,
		)
		if err != nil {
			return false, n, err
		}

		keyPrefix := new(Path).MSBs(key, match.Len()+1)
		_, branch.children[key.Bit(match.Len())], err = t.insert(
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
		return true, &edgeNode{path: matchPrefix, child: branch, flags: newFlag()}, nil

	case *binaryNode:
		// Go to the child node based on the MSB of the key
		bit := key.MSB()
		dirty, newNode, err := t.insert(
			n.children[bit], new(Path).AppendBit(prefix, bit), new(Path).LSBs(key, 1), value,
		)
		if !dirty || err != nil {
			return false, n, err
		}
		// Replace the child node with the new node
		n = n.copy()
		n.flags = newFlag()
		n.children[bit] = newNode
		return true, n, nil
	case nil:
		t.nodeTracer.onInsert(prefix)
		// We reach the end of the key, return the value node
		if key.IsEmpty() {
			return true, value, nil
		}
		// Otherwise, return a new edge node with the Path being the key and the value as the child
		return true, &edgeNode{path: key, child: value, flags: newFlag()}, nil
	case *hashNode:
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

//nolint:gocyclo,funlen
func (t *Trie) delete(n node, prefix, key *Path) (bool, node, error) {
	switch n := n.(type) {
	case *edgeNode:
		match := n.commonPath(key)
		// Mismatched, don't do anything
		if match.Len() < n.path.Len() {
			return false, n, nil
		}
		// If the whole key matches, remove the entire edge node
		if match.Len() == key.Len() {
			t.nodeTracer.onDelete(prefix)
			return true, nil, nil
		}

		// Otherwise, key is longer than current node path, so we need to delete the child.
		// Child can never be nil because it's guaranteed that we have at least 2 other values in the subtrie.
		keyPrefix := new(Path).MSBs(key, n.path.Len())
		dirty, child, err := t.delete(n.child, new(Path).Append(prefix, keyPrefix), new(Path).LSBs(key, n.path.Len()))
		if !dirty || err != nil {
			return false, n, err
		}
		switch child := child.(type) {
		case *edgeNode:
			t.nodeTracer.onDelete(new(Path).Append(prefix, n.path))
			return true, &edgeNode{path: new(Path).Append(n.path, child.path), child: child.child, flags: newFlag()}, nil
		default:
			return true, &edgeNode{path: new(Path).Set(n.path), child: child, flags: newFlag()}, nil
		}
	case *binaryNode:
		bit := key.MSB()
		keyPrefix := new(Path).MSBs(key, 1)
		dirty, newNode, err := t.delete(n.children[bit], new(Path).Append(prefix, keyPrefix), key.LSBs(key, 1))
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
		// If it's a hash node, we need to resolve it first.
		// If the other child is an edge node, we prepend the bit prefix to the other child path
		other := bit ^ 1
		bitPrefix := new(Path).SetBit(other)

		if hn, ok := n.children[other].(*hashNode); ok {
			var cPath Path
			cPath.Append(prefix, bitPrefix)
			cNode, err := t.resolveNode(hn, cPath)
			if err != nil {
				return false, nil, err
			}
			n.children[other] = cNode
		}

		if cn, ok := n.children[other].(*edgeNode); ok {
			t.nodeTracer.onDelete(new(Path).Append(prefix, bitPrefix))
			return true, &edgeNode{
				path:  new(Path).Append(bitPrefix, cn.path),
				child: cn.child,
				flags: newFlag(),
			}, nil
		}

		// other child is not an edge node, create a new edge node with the bit prefix as the Path
		// containing the other child as the child
		return true, &edgeNode{path: bitPrefix, child: n.children[other], flags: newFlag()}, nil
	case *valueNode:
		return true, nil, nil
	case nil:
		return false, nil, nil
	case *hashNode:
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

func (t *Trie) resolveNode(hash *hashNode, path Path) (node, error) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	_, err := t.db.Get(buf, t.owner, path)
	if err != nil {
		return nil, err
	}

	blob := buf.Bytes()
	return decodeNode(blob, hash.Felt, path.Len(), t.height)
}

func (t *Trie) hashRoot() (node, node) {
	if t.root == nil {
		return &hashNode{Felt: felt.Zero}, nil
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
	return New(TrieID(felt.Zero), contractClassTrieHeight, crypto.Pedersen, db.NewMemTransaction())
}

func NewEmptyPoseidon() (*Trie, error) {
	return New(TrieID(felt.Zero), contractClassTrieHeight, crypto.Poseidon, db.NewMemTransaction())
}
