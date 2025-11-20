// Package trie implements a dense Merkle Patricia Trie. See the documentation on [Trie] for details.
package trie

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
)

const globalTrieHeight = 251 // TODO(weiihann): this is declared in core also, should be moved to a common place

type TrieReader struct {
	height      uint8
	rootKey     *BitArray
	maxKey      *felt.Felt
	readStorage *ReadStorage
	hash        crypto.HashFn
}

func NewTrieReaderPedersen(
	r db.KeyValueReader,
	prefix []byte,
	height uint8,
) (*TrieReader, error) {
	return newTrieReader(r, prefix, height, crypto.Pedersen)
}

func NewTrieReaderPoseidon(
	r db.KeyValueReader,
	prefix []byte,
	height uint8,
) (*TrieReader, error) {
	return newTrieReader(r, prefix, height, crypto.Poseidon)
}

func newTrieReader(
	r db.KeyValueReader,
	prefix []byte,
	height uint8,
	hash crypto.HashFn,
) (*TrieReader, error) {
	if height > felt.Bits {
		return nil, fmt.Errorf("max trie height is %d, got: %d", felt.Bits, height)
	}

	// maxKey is 2^height - 1
	maxKey := new(felt.Felt).Exp(new(felt.Felt).SetUint64(2), new(big.Int).SetUint64(uint64(height)))
	maxKey.Sub(maxKey, new(felt.Felt).SetUint64(1))

	readStorage := NewReadStorage(r, prefix)
	rootKey, err := readStorage.RootKey()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	return &TrieReader{
		readStorage: readStorage,
		height:      height,
		rootKey:     rootKey,
		maxKey:      maxKey,
		hash:        hash,
	}, nil
}

// Trie is a dense Merkle Patricia Trie (i.e., all internal nodes have two children).
//
// This implementation allows for a "flat" storage by keying nodes on their path rather than
// their hash, resulting in O(1) accesses and O(log n) insertions.
//
// The state trie [specification] describes a sparse Merkle Trie.
// Note that this dense implementation results in an equivalent commitment.
//
// Terminology:
//   - path: represents the path as defined in the specification. Together with len,
//     represents a relative path from the current node to the node's nearest non-empty child.
//   - len: represents the len as defined in the specification. The number of bits to take
//     from the fixed-length path to reach the nearest non-empty child.
//   - key: represents the storage key for trie [Node]s. It is the full path to the node from the
//     root.
//
// [specification]: https://docs.starknet.io/architecture-and-concepts/network-architecture/starknet-state/#merkle_patricia_trie
type Trie struct {
	*TrieReader
	storage        *Storage
	dirtyNodes     []*BitArray
	rootKeyIsDirty bool
}

type NewTrieFunc func(db.IndexedBatch, []byte, uint8) (*Trie, error)

func NewTriePedersen(txn db.IndexedBatch, prefix []byte, height uint8) (*Trie, error) {
	return newTrie(txn, prefix, height, crypto.Pedersen)
}

func NewTriePoseidon(txn db.IndexedBatch, prefix []byte, height uint8) (*Trie, error) {
	return newTrie(txn, prefix, height, crypto.Poseidon)
}

func newTrie(
	txn db.IndexedBatch,
	prefix []byte,
	height uint8,
	hash crypto.HashFn,
) (*Trie, error) {
	if height > felt.Bits {
		return nil, fmt.Errorf("max trie height is %d, got: %d", felt.Bits, height)
	}

	// maxKey is 2^height - 1
	maxKey := new(felt.Felt).Exp(new(felt.Felt).SetUint64(2), new(big.Int).SetUint64(uint64(height)))
	maxKey.Sub(maxKey, new(felt.Felt).SetUint64(1))

	storage := NewStorage(txn, prefix)
	rootKey, err := storage.RootKey()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	readOnlyStorage := NewReadStorage(storage.txn, storage.prefix)
	return &Trie{
		TrieReader: &TrieReader{
			readStorage: readOnlyStorage,
			height:      height,
			rootKey:     rootKey,
			maxKey:      maxKey,
			hash:        hash,
		},
		storage: storage,
	}, nil
}

// RunOnTempTriePedersen creates an in-memory Trie of height `height` and runs `do` on that Trie
func RunOnTempTriePedersen(height uint8, do func(*Trie) error) error {
	memoryDB := memory.New()
	txn := memoryDB.NewIndexedBatch()
	trie, err := NewTriePedersen(txn, nil, height)
	if err != nil {
		return err
	}
	return do(trie)
}

func RunOnTempTriePoseidon(height uint8, do func(*Trie) error) error {
	memoryDB := memory.New()
	txn := memoryDB.NewIndexedBatch()
	trie, err := NewTriePoseidon(txn, nil, height)
	if err != nil {
		return err
	}
	return do(trie)
}

// FeltToKey converts a key, given in felt, to a trie.Key which when followed on a [Trie],
// leads to the corresponding [Node]
func (t *TrieReader) FeltToKey(k *felt.Felt) BitArray {
	var ba BitArray
	ba.SetFelt(t.height, k)
	return ba
}

// path returns the path as mentioned in the [specification] for commitment calculations.
// path is suffix of key that diverges from parentKey. For example,
// for a key 0b1011 and parentKey 0b10, this function would return the path object of 0b0.
func path(key, parentKey *BitArray) BitArray {
	// drop parent key, and one more MSB since left/right relation already encodes that information
	if parentKey == nil {
		return key.Copy()
	}

	var pathKey BitArray
	pathKey.LSBs(key, parentKey.Len()+1)
	return pathKey
}

// storageNode is the on-disk representation of a [Node],
// where key is the storage key and node is the value.
type StorageNode struct {
	key  *BitArray
	node *Node
}

func (sn *StorageNode) Key() *BitArray {
	return sn.key
}

func (sn *StorageNode) Value() *felt.Felt {
	return sn.node.Value
}

func (sn *StorageNode) String() string {
	return fmt.Sprintf("StorageNode{key: %s, node: %s}", sn.key, sn.node)
}

func (sn *StorageNode) Update(other *StorageNode) error {
	// First validate all fields for conflicts
	if sn.key != nil && other.key != nil && !sn.key.Equal(emptyBitArray) && !other.key.Equal(emptyBitArray) {
		if !sn.key.Equal(other.key) {
			return fmt.Errorf("keys do not match: %s != %s", sn.key, other.key)
		}
	}

	// Validate node updates
	if sn.node != nil && other.node != nil {
		if err := sn.node.Update(other.node); err != nil {
			return err
		}
	}

	// After validation, perform update
	if other.key != nil && !other.key.Equal(emptyBitArray) {
		sn.key = other.key
	}

	return nil
}

func NewStorageNode(key *BitArray, node *Node) *StorageNode {
	return &StorageNode{key: key, node: node}
}

// NewPartialStorageNode creates a new StorageNode with a given key and value,
// where the right and left children are nil.
func NewPartialStorageNode(key *BitArray, value *felt.Felt) *StorageNode {
	return &StorageNode{
		key: key,
		node: &Node{
			Value: value,
			Left:  emptyBitArray,
			Right: emptyBitArray,
		},
	}
}

// StorageNodeSet wraps OrderedSet to provide specific functionality for StorageNodes
type StorageNodeSet struct {
	set *utils.OrderedSet[BitArray, *StorageNode]
}

func NewStorageNodeSet() *StorageNodeSet {
	return &StorageNodeSet{
		set: utils.NewOrderedSet[BitArray, *StorageNode](),
	}
}

func (s *StorageNodeSet) Get(key BitArray) (*StorageNode, bool) {
	return s.set.Get(key)
}

// Put adds a new StorageNode or updates an existing one.
func (s *StorageNodeSet) Put(key BitArray, node *StorageNode) error {
	if node == nil {
		return errors.New("cannot put nil node")
	}

	// If key exists, update the node
	if existingNode, exists := s.set.Get(key); exists {
		if err := existingNode.Update(node); err != nil {
			return fmt.Errorf("failed to update node for key %v: %w", key, err)
		}
		return nil
	}

	// Add new node if key doesn't exist
	s.set.Put(key, node)
	return nil
}

// List returns the list of StorageNodes in the set.
func (s *StorageNodeSet) List() []*StorageNode {
	return s.set.List()
}

func (s *StorageNodeSet) Size() int {
	return s.set.Size()
}

// nodesFromRoot enumerates the set of [Node] objects that need to be traversed from the root
// of the Trie to the node which is given by the key.
// The [storageNode]s are returned in descending order beginning with the root.
func (t *TrieReader) nodesFromRoot(key *BitArray) ([]StorageNode, error) {
	var nodes []StorageNode
	cur := t.rootKey
	for cur != nil {
		// proof nodes set "nil" nodes to zero
		if len(nodes) > 0 && cur.len == 0 {
			return nodes, nil
		}

		node, err := t.readStorage.Get(cur)
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, StorageNode{
			key:  cur,
			node: node,
		})

		if cur.Len() >= key.Len() || !key.EqualMSBs(cur) {
			return nodes, nil
		}

		if key.IsBitSet(cur.Len()) {
			cur = node.Right
		} else {
			cur = node.Left
		}
	}

	return nodes, nil
}

// Get the corresponding `value` for a `key`
func (t *TrieReader) Get(key *felt.Felt) (felt.Felt, error) {
	storageKey := t.FeltToKey(key)
	value, err := t.readStorage.Get(&storageKey)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return felt.Zero, nil
		}
		return felt.Zero, err
	}
	defer nodePool.Put(value)
	leafValue := *value.Value
	return leafValue, nil
}

// GetNodeFromKey returns the node for a given key.
func (t *TrieReader) GetNodeFromKey(key *BitArray) (*Node, error) {
	return t.readStorage.Get(key)
}

// RootKey returns db key of the [Trie] root node
func (t *TrieReader) RootKey() *BitArray {
	return t.rootKey
}

func (t *TrieReader) Dump() {
	t.dump(0, nil)
}

func (t *TrieReader) HashFn() crypto.HashFn {
	return t.hash
}

// check if we are updating an existing leaf, if yes avoid traversing the trie
func (t *Trie) updateLeaf(nodeKey BitArray, node *Node, value *felt.Felt) (*felt.Felt, error) {
	// Check if we are updating an existing leaf
	if !value.IsZero() {
		if existingLeaf, err := t.readStorage.Get(&nodeKey); err == nil {
			old := *existingLeaf.Value // record old value to return to caller
			if err = t.storage.Put(&nodeKey, node); err != nil {
				return nil, err
			}
			t.dirtyNodes = append(t.dirtyNodes, &nodeKey)
			return &old, nil
		} else if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
	}
	return nil, nil
}

func (t *Trie) handleEmptyTrie(old felt.Felt, nodeKey BitArray, node *Node, value *felt.Felt) (*felt.Felt, error) {
	if value.IsZero() {
		return nil, nil // no-op
	}

	if err := t.storage.Put(&nodeKey, node); err != nil {
		return nil, err
	}
	t.setRootKey(&nodeKey)
	return &old, nil
}

func (t *Trie) deleteExistingKey(sibling StorageNode, nodeKey BitArray, nodes []StorageNode) (*felt.Felt, error) {
	if nodeKey.Equal(sibling.key) {
		// we have to deference the Value, since the Node can released back
		// to the NodePool and be reused anytime
		old := *sibling.node.Value // record old value to return to caller
		if err := t.deleteLast(nodes); err != nil {
			return nil, err
		}
		return &old, nil
	}
	return nil, nil
}

func (t *Trie) replaceLinkWithNewParent(key *BitArray, commonKey BitArray, siblingParent StorageNode) {
	if siblingParent.node.Left.Equal(key) {
		*siblingParent.node.Left = commonKey
	} else {
		*siblingParent.node.Right = commonKey
	}
}

// TODO(weiihann): not a good idea to couple proof verification logic with trie logic
func (t *Trie) insertOrUpdateValue(
	nodeKey *BitArray,
	node *Node,
	nodes []StorageNode,
	sibling StorageNode,
	siblingIsParentProof bool,
) error {
	var commonKey BitArray
	commonKey.CommonMSBs(nodeKey, sibling.key)

	newParent := &Node{}
	var leftChild, rightChild *Node
	var err error

	// Update the (proof) parents child and hash
	if siblingIsParentProof {
		newParent, err = t.GetNodeFromKey(&commonKey)
		if err != nil {
			return err
		}
		if nodeKey.IsBitSet(commonKey.Len()) {
			newParent.Right = nodeKey
			rightHash := node.Hash(nodeKey, t.hash)
			newParent.RightHash = &rightHash
		} else {
			newParent.Left = nodeKey
			leftHash := node.Hash(nodeKey, t.hash)
			newParent.LeftHash = &leftHash
		}
		if err := t.storage.Put(&commonKey, newParent); err != nil {
			return err
		}
		t.dirtyNodes = append(t.dirtyNodes, &commonKey)
	} else {
		if nodeKey.IsBitSet(commonKey.Len()) {
			newParent.Left, newParent.Right = sibling.key, nodeKey
			leftChild, rightChild = sibling.node, node
		} else {
			newParent.Left, newParent.Right = nodeKey, sibling.key
			leftChild, rightChild = node, sibling.node
		}

		leftPath := path(newParent.Left, &commonKey)
		rightPath := path(newParent.Right, &commonKey)

		leftChildHash := leftChild.Hash(&leftPath, t.hash)
		rightChildHash := rightChild.Hash(&rightPath, t.hash)
		hash := t.hash(&leftChildHash, &rightChildHash)
		newParent.Value = &hash
		if err := t.storage.Put(&commonKey, newParent); err != nil {
			return err
		}

		if len(nodes) > 1 { // sibling has a parent
			siblingParent := (nodes)[len(nodes)-2]

			t.replaceLinkWithNewParent(sibling.key, commonKey, siblingParent)
			if err := t.storage.Put(siblingParent.key, siblingParent.node); err != nil {
				return err
			}
			t.dirtyNodes = append(t.dirtyNodes, &commonKey)
		} else {
			t.setRootKey(&commonKey)
		}
	}

	if err := t.storage.Put(nodeKey, node); err != nil {
		return err
	}
	return nil
}

// Put updates the corresponding `value` for a `key`
func (t *Trie) Put(key, value *felt.Felt) (*felt.Felt, error) {
	if key.Cmp(t.maxKey) > 0 {
		return nil, fmt.Errorf("key %s exceeds trie height %d", key, t.height)
	}

	old := felt.Zero
	nodeKey := t.FeltToKey(key)
	node := &Node{
		Value: value,
	}

	oldValue, err := t.updateLeaf(nodeKey, node, value)
	// xor operation, because we don't want to return result if the error is nil and the old value is nil
	if (err != nil) != (oldValue != nil) {
		return oldValue, err
	}

	nodes, err := t.nodesFromRoot(&nodeKey) // correct for key,value
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, n := range nodes {
			nodePool.Put(n.node)
		}
	}()

	// empty trie, make new value root
	if len(nodes) == 0 {
		return t.handleEmptyTrie(old, nodeKey, node, value)
	} else {
		// Since we short-circuit in leaf updates, we will only end up here for deletions
		// Delete if key already exist
		sibling := nodes[len(nodes)-1]
		oldValue, err = t.deleteExistingKey(sibling, nodeKey, nodes)
		// xor operation, because we don't want to return if the error is nil and the old value is nil
		if (err != nil) != (oldValue != nil) {
			return oldValue, err
		} else if value.IsZero() {
			// trying to insert 0 to a key that does not exist
			return nil, nil // no-op
		}
		err := t.insertOrUpdateValue(&nodeKey, node, nodes, sibling, false)
		if err != nil {
			return nil, err
		}
		return &old, nil
	}
}

func (t *Trie) Update(key, value *felt.Felt) error {
	_, err := t.Put(key, value)
	return err
}

// PutWithProof updates the corresponding `value` for a `key` using a proof
func (t *Trie) PutWithProof(key, value *felt.Felt, proof []*StorageNode) (*felt.Felt, error) {
	if key.Cmp(t.maxKey) > 0 {
		return nil, fmt.Errorf("key %s exceeds trie height %d", key, t.height)
	}

	old := felt.Zero
	nodeKey := t.FeltToKey(key)
	node := &Node{
		Value: value,
	}

	oldValue, err := t.updateLeaf(nodeKey, node, value)
	// xor operation, because we don't want to return result if the error is nil and the old value is nil
	if (err != nil) != (oldValue != nil) {
		return oldValue, err
	}

	nodes, err := t.nodesFromRoot(&nodeKey) // correct for key,value
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, n := range nodes {
			nodePool.Put(n.node)
		}
	}()

	// empty trie, make new value root
	if len(nodes) == 0 {
		return t.handleEmptyTrie(old, nodeKey, node, value)
	} else {
		// Since we short-circuit in leaf updates, we will only end up here for deletions
		// Delete if key already exist
		sibling := nodes[len(nodes)-1]
		oldValue, err = t.deleteExistingKey(sibling, nodeKey, nodes)
		// xor operation, because we don't want to return if the error is nil and the old value is nil
		if (err != nil) != (oldValue != nil) {
			return oldValue, err
		} else if value.IsZero() {
			// trying to insert 0 to a key that does not exist
			return nil, nil // no-op
		}

		// override the sibling to be the parent if it's a proof
		parentIsProof := false
		for _, proofNode := range proof {
			if proofNode.key.Equal(sibling.key) {
				sibling = *proofNode
				parentIsProof = true
				break
			}
		}

		err := t.insertOrUpdateValue(&nodeKey, node, nodes, sibling, parentIsProof)
		if err != nil {
			return nil, err
		}
		return &old, nil
	}
}

// PutInner updates an inner node in the trie
func (t *Trie) PutInner(key *BitArray, node *Node) error {
	if err := t.storage.Put(key, node); err != nil {
		return err
	}
	return nil
}

func (t *Trie) setRootKey(newRootKey *BitArray) {
	t.rootKey = newRootKey
	t.rootKeyIsDirty = true
}

func (t *Trie) updateValueIfDirty(key *BitArray) (*Node, error) { //nolint:gocyclo
	node, err := t.readStorage.Get(key)
	if err != nil {
		return nil, err
	}

	// leaf node
	if key.Len() == t.height {
		return node, nil
	}

	shouldUpdate := false
	for _, dirtyNode := range t.dirtyNodes {
		if key.Len() < dirtyNode.Len() {
			shouldUpdate = key.EqualMSBs(dirtyNode)
			if shouldUpdate {
				break
			}
		}
	}

	// Update inner proof nodes
	if node.Left.Equal(emptyBitArray) && node.Right.Equal(emptyBitArray) { // leaf
		shouldUpdate = false
	} else if node.Left.Equal(emptyBitArray) || node.Right.Equal(emptyBitArray) { // inner
		shouldUpdate = true
	}
	if !shouldUpdate {
		return node, nil
	}

	var leftIsProof, rightIsProof bool
	var leftHash, rightHash *felt.Felt
	if node.Left.Equal(emptyBitArray) { // key could be nil but hash cannot be
		leftIsProof = true
		leftHash = node.LeftHash
	}
	if node.Right.Equal(emptyBitArray) {
		rightIsProof = true
		rightHash = node.RightHash
	}

	// To avoid over-extending, only use concurrent updates when we are not too
	// deep in to traversing the trie.
	const concurrencyMaxDepth = 8 // heuristically selected value
	var leftChild, rightChild *Node
	if key.len <= concurrencyMaxDepth {
		leftChild, rightChild, err = t.updateChildTriesConcurrently(node, leftIsProof, rightIsProof)
	} else {
		leftChild, rightChild, err = t.updateChildTriesSerially(node, leftIsProof, rightIsProof)
	}
	if err != nil {
		return nil, err
	}

	if !leftIsProof {
		leftPath := path(node.Left, key)
		defer nodePool.Put(leftChild)
		leftChildHash := leftChild.Hash(&leftPath, t.hash)
		leftHash = &leftChildHash
	}
	if !rightIsProof {
		rightPath := path(node.Right, key)
		defer nodePool.Put(rightChild)
		rightChildHash := rightChild.Hash(&rightPath, t.hash)
		rightHash = &rightChildHash
	}
	hash := t.hash(leftHash, rightHash)
	node.Value = &hash
	if err = t.storage.Put(key, node); err != nil {
		return nil, err
	}
	return node, nil
}

func (t *Trie) updateChildTriesSerially(root *Node, leftIsProof, rightIsProof bool) (*Node, *Node, error) {
	var leftChild, rightChild *Node
	var err error
	if !leftIsProof {
		leftChild, err = t.updateValueIfDirty(root.Left)
		if err != nil {
			return nil, nil, err
		}
	}
	if !rightIsProof {
		rightChild, err = t.updateValueIfDirty(root.Right)
		if err != nil {
			return nil, nil, err
		}
	}
	return leftChild, rightChild, nil
}

func (t *Trie) updateChildTriesConcurrently(root *Node, leftIsProof, rightIsProof bool) (*Node, *Node, error) {
	var leftChild, rightChild *Node
	var lErr, rErr error

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if !leftIsProof {
			leftChild, lErr = t.updateValueIfDirty(root.Left)
		}
	}()
	if !rightIsProof {
		rightChild, rErr = t.updateValueIfDirty(root.Right)
	}
	wg.Wait()

	if lErr != nil {
		return nil, nil, lErr
	}
	if rErr != nil {
		return nil, nil, rErr
	}
	return leftChild, rightChild, nil
}

// deleteLast deletes the last node in the given list
func (t *Trie) deleteLast(nodes []StorageNode) error {
	last := nodes[len(nodes)-1]
	if err := t.storage.Delete(last.key); err != nil {
		return err
	}

	if len(nodes) == 1 { // deleted node was root
		t.setRootKey(nil)
		return nil
	}

	// parent now has only a single child, so delete
	parent := nodes[len(nodes)-2]
	if err := t.storage.Delete(parent.key); err != nil {
		return err
	}

	var siblingKey BitArray
	if parent.node.Left.Equal(last.key) {
		siblingKey = *parent.node.Right
	} else {
		siblingKey = *parent.node.Left
	}

	if len(nodes) == 2 { // sibling should become root
		t.setRootKey(&siblingKey)
		return nil
	}
	// sibling should link to grandparent (len(affectedNodes) > 2)
	grandParent := &nodes[len(nodes)-3]
	// replace link to parent with a link to sibling
	if grandParent.node.Left.Equal(parent.key) {
		*grandParent.node.Left = siblingKey
	} else {
		*grandParent.node.Right = siblingKey
	}

	if err := t.storage.Put(grandParent.key, grandParent.node); err != nil {
		return err
	}
	t.dirtyNodes = append(t.dirtyNodes, &siblingKey)
	return nil
}

// Root returns the commitment of a [Trie]
func (t *Trie) Hash() (felt.Felt, error) {
	// We are careful to update the root key before returning.
	// Otherwise, a new trie will not be able to find the root node.
	if t.rootKeyIsDirty {
		if t.rootKey == nil {
			if err := t.storage.DeleteRootKey(); err != nil {
				return felt.Zero, err
			}
		} else if err := t.storage.PutRootKey(t.rootKey); err != nil {
			return felt.Zero, err
		}
		t.rootKeyIsDirty = false
	}

	if t.rootKey == nil {
		return felt.Zero, nil
	}

	storage := t.storage
	t.storage = storage.SyncedStorage()
	defer func() { t.storage = storage }()
	root, err := t.updateValueIfDirty(t.rootKey)
	if err != nil {
		return felt.Zero, err
	}
	defer nodePool.Put(root)
	t.dirtyNodes = nil

	path := path(t.rootKey, nil)
	return root.Hash(&path, t.hash), nil
}

// TODO: remove this in the followup PR
func (t *Trie) Root() (felt.Felt, error) {
	return t.Hash()
}

// Commit forces root calculation
func (t *Trie) Commit() error {
	_, err := t.Hash()
	return err
}

// Try to print a [Trie] in a somewhat human-readable form
/*
Todo: create more meaningful representation of trie. In the current format string, storage is being
printed but the value that is printed is the bitset of the trie node this is different from the
storage of the trie. Also, consider renaming the function name to something other than dump.

The following can be printed:
- key (which represents the storage key)
- path (as specified in the documentation)
- len (as specified in the documentation)
- bottom (as specified in the documentation)

The spacing to represent the levels of the trie can remain the same.
*/
func (t *TrieReader) dump(level int, parentP *BitArray) {
	if t.rootKey == nil {
		fmt.Printf("%sEMPTY\n", strings.Repeat("\t", level))
		return
	}

	root, err := t.readStorage.Get(t.rootKey)
	if err != nil {
		return
	}
	defer nodePool.Put(root)
	path := path(t.rootKey, parentP)

	left := ""
	right := ""
	leftHash := ""
	rightHash := ""

	if root.Left != nil {
		left = root.Left.String()
	}
	if root.Right != nil {
		right = root.Right.String()
	}
	if root.LeftHash != nil {
		leftHash = root.LeftHash.String()
	}
	if root.RightHash != nil {
		rightHash = root.RightHash.String()
	}

	fmt.Printf("%skey : \"%s\" path: \"%s\" left: \"%s\" right: \"%s\" LH: \"%s\" RH: \"%s\" value: \"%s\" \n",
		strings.Repeat("\t", level),
		t.rootKey.String(),
		path.String(),
		left,
		right,
		leftHash,
		rightHash,
		root.Value.String(),
	)
	(&TrieReader{
		rootKey:     root.Left,
		readStorage: t.readStorage,
	}).dump(level+1, t.rootKey)
	(&TrieReader{
		rootKey:     root.Right,
		readStorage: t.readStorage,
	}).dump(level+1, t.rootKey)
}
