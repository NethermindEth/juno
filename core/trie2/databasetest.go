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

func (n *testNodeReader) Node(owner *felt.Felt, path *trieutils.Path, hash *felt.Felt, isLeaf bool) ([]byte, error) {
	for _, nodes := range n.nodes {
		var (
			node trienode.TrieNode
			ok   bool
		)
		node, ok = nodes.OwnerSet.Nodes[*path]
		if !ok {
			continue
		}
		if _, ok := node.(*trienode.DeletedNode); ok {
			return nil, &MissingNodeError{owner: *owner, path: *path, hash: node.Hash()}
		}
		return node.Blob(), nil
	}
	return readNode(n.db, n.id, n.scheme, path, hash, isLeaf)
}

func readNode(
	r db.KeyValueStore,
	id trieutils.TrieID,
	scheme dbScheme,
	path *trieutils.Path,
	hash *felt.Felt,
	isLeaf bool,
) ([]byte, error) {
	switch scheme {
	case PathScheme:
		owner := id.Owner()
		return trieutils.GetNodeByPath(r, id.Bucket(), &owner, path, isLeaf)
	case HashScheme:
		// TODO: implement hash scheme
	}

	owner := id.Owner()
	return nil, &MissingNodeError{owner: owner, path: *path, hash: *hash}
}

type TestNodeDatabase struct {
	disk      db.KeyValueStore
	root      felt.Felt
	scheme    dbScheme
	nodes     map[felt.Felt]*trienode.MergeNodeSet
	rootLinks map[felt.Felt]felt.Felt // map[child_root]parent_root - keep track of the parent root for each child root
}

func NewTestNodeDatabase(disk db.KeyValueStore, scheme dbScheme) TestNodeDatabase {
	return TestNodeDatabase{
		disk:      disk,
		root:      felt.Zero,
		scheme:    scheme,
		nodes:     make(map[felt.Felt]*trienode.MergeNodeSet),
		rootLinks: make(map[felt.Felt]felt.Felt),
	}
}

func (d *TestNodeDatabase) Update(root, parent *felt.Felt, nodes *trienode.MergeNodeSet) error {
	if root == parent {
		return nil
	}

	rootVal := *root
	parentVal := *parent

	if _, ok := d.nodes[rootVal]; ok { // already exists
		return nil
	}

	d.nodes[rootVal] = nodes
	d.rootLinks[rootVal] = parentVal

	return nil
}

func (d *TestNodeDatabase) NodeReader(id trieutils.TrieID) (database.NodeReader, error) {
	root := id.StateComm()
	nodes, _ := d.dirties(&root, true)
	return newTestNodeReader(id, nodes, d.disk, d.scheme), nil
}

func (d *TestNodeDatabase) dirties(root *felt.Felt, newerFirst bool) ([]*trienode.MergeNodeSet, []felt.Felt) {
	var (
		pending []*trienode.MergeNodeSet
		roots   []felt.Felt
	)

	rootVal := *root

	for {
		if rootVal == d.root {
			break
		}

		nodes, ok := d.nodes[rootVal]
		if !ok {
			break
		}

		if newerFirst {
			pending = append(pending, nodes)
			roots = append(roots, rootVal)
		} else {
			pending = append([]*trienode.MergeNodeSet{nodes}, pending...)
			roots = append([]felt.Felt{rootVal}, roots...)
		}

		rootVal = d.rootLinks[rootVal]
	}

	return pending, roots
}
