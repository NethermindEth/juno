package trie2

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

type dbScheme uint8

const (
	PathScheme dbScheme = iota + 1
	HashScheme
)

type testNodeReader struct {
	id     trieutils.TrieID
	nodes  []*trienode.MergeNodeSet
	db     db.KeyValueStore
	scheme dbScheme
}

func newTestNodeReader(id trieutils.TrieID, nodes []*trienode.MergeNodeSet, db db.KeyValueStore, scheme dbScheme) *testNodeReader {
	return &testNodeReader{id: id, nodes: nodes, db: db, scheme: scheme}
}

func (n *testNodeReader) Node(owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) ([]byte, error) {
	var (
		node trienode.TrieNode
		set  *trienode.NodeSet
		ok   bool
	)
	for _, nodes := range n.nodes {
		if nodes.OwnerSet.Owner.IsZero() {
			node, ok = nodes.OwnerSet.Nodes[path]
			if !ok {
				continue
			}
		} else {
			set, ok = nodes.ChildSets[owner]
			if !ok {
				continue
			}
			node, ok = set.Nodes[path]
			if !ok {
				continue
			}
		}
		nHash := node.Hash()
		if _, ok := node.(*trienode.DeletedNode); ok {
			return nil, &MissingNodeError{owner: owner, path: path, hash: nHash}
		}
		return node.Blob(), nil
	}
	return readNode(n.db, n.id, n.scheme, path, hash, isLeaf)
}

func readNode(r db.KeyValueStore, id trieutils.TrieID, scheme dbScheme, path trieutils.Path, hash felt.Felt, isLeaf bool) ([]byte, error) {
	switch scheme {
	case PathScheme:
		return trieutils.GetNodeByPath(r, id.Bucket(), id.Owner(), path, isLeaf)
	case HashScheme:
		// TODO: implement hash scheme
	}

	return nil, &MissingNodeError{owner: id.Owner(), path: path, hash: hash}
}

func writeNode(
	r db.KeyValueStore,
	id trieutils.TrieID,
	scheme dbScheme,
	path trieutils.Path,
	hash felt.Felt, //nolint:unparam // Probably will be used by hash scheme
	isLeaf bool,
	node []byte,
) error {
	switch scheme {
	case PathScheme:
		return trieutils.WriteNodeByPath(r, id.Bucket(), id.Owner(), path, isLeaf, node)
	case HashScheme:
		// TODO: implement hash scheme
	}

	return nil
}

type TestNodeDatabase struct {
	disk      db.KeyValueStore
	root      felt.Felt
	scheme    dbScheme
	nodes     map[felt.Felt]*trienode.MergeNodeSet
	rootLinks map[felt.Felt]felt.Felt // map[child_root]parent_root - keep track of the parent root for each child root
}

func NewTestNodeDatabase(disk db.KeyValueStore, scheme dbScheme) *TestNodeDatabase {
	return &TestNodeDatabase{
		disk:      disk,
		root:      felt.Zero,
		scheme:    scheme,
		nodes:     make(map[felt.Felt]*trienode.MergeNodeSet),
		rootLinks: make(map[felt.Felt]felt.Felt),
	}
}

func (d *TestNodeDatabase) Update(root, parent felt.Felt, nodes *trienode.MergeNodeSet) error {
	if root == parent {
		return nil
	}

	if _, ok := d.nodes[root]; ok { // already exists
		return nil
	}

	d.nodes[root] = nodes
	d.rootLinks[root] = parent

	return nil
}

func (d *TestNodeDatabase) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	nodes, _ := d.dirties(id.StateComm(), true)
	return newTestNodeReader(id, nodes, d.disk, d.scheme), nil
}

func (d *TestNodeDatabase) Commit(stateComm felt.Felt) error {
	if stateComm == d.root {
		return nil
	}

	pending, roots := d.dirties(stateComm, false)
	for i, nodes := range pending {
		for owner, set := range nodes.ChildSets {
			// Write contract storage trie nodes
			if err := set.ForEach(true, func(path trieutils.Path, node trienode.TrieNode) error {
				return writeNode(d.disk, trieutils.NewContractStorageTrieID(stateComm, owner), d.scheme, path, node.Hash(), node.IsLeaf(), node.Blob())
			}); err != nil {
				return err
			}
		}

		// Write contract trie nodes
		if err := nodes.OwnerSet.ForEach(true, func(path trieutils.Path, node trienode.TrieNode) error {
			return writeNode(d.disk, trieutils.NewContractTrieID(stateComm), d.scheme, path, node.Hash(), node.IsLeaf(), node.Blob())
		}); err != nil {
			return err
		}
		d.root = roots[i]
	}

	for _, root := range roots {
		delete(d.nodes, root)
		delete(d.rootLinks, root)
	}

	return nil
}

func (d *TestNodeDatabase) dirties(root felt.Felt, newerFirst bool) ([]*trienode.MergeNodeSet, []felt.Felt) {
	var (
		pending []*trienode.MergeNodeSet
		roots   []felt.Felt
	)

	for {
		if root == d.root {
			break
		}

		nodes, ok := d.nodes[root]
		if !ok {
			break
		}

		if newerFirst {
			pending = append(pending, nodes)
			roots = append(roots, root)
		} else {
			pending = append([]*trienode.MergeNodeSet{nodes}, pending...)
			roots = append([]felt.Felt{root}, roots...)
		}

		root = d.rootLinks[root]
	}

	return pending, roots
}
