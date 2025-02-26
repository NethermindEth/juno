package trienode

import (
	"fmt"
	"maps"
	"sort"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

type TrieNode interface {
	Blob() []byte
	Hash() felt.Felt
	IsLeaf() bool
}

// Contains a set of nodes, which are indexed by their path in the trie.
// It is not thread safe.
type NodeSet struct {
	Owner   felt.Felt // The owner (i.e. contract address)
	Nodes   map[trieutils.BitArray]TrieNode
	updates int // the count of updated and inserted nodes
	deletes int // the count of deleted nodes
}

func NewNodeSet(owner felt.Felt) *NodeSet {
	return &NodeSet{Owner: owner, Nodes: make(map[trieutils.BitArray]TrieNode)}
}

func (ns *NodeSet) Add(key trieutils.BitArray, node TrieNode) {
	if _, ok := node.(*DeletedNode); ok {
		ns.deletes += 1
	} else {
		ns.updates += 1
	}
	ns.Nodes[key] = node
}

// Iterates over the nodes in a sorted order and calls the callback for each node.
func (ns *NodeSet) ForEach(desc bool, callback func(key trieutils.BitArray, node TrieNode) error) error {
	paths := make([]trieutils.BitArray, 0, len(ns.Nodes))
	for key := range ns.Nodes {
		paths = append(paths, key)
	}

	if desc { // longest path first
		sort.Slice(paths, func(i, j int) bool {
			return paths[i].Cmp(&paths[j]) > 0
		})
	} else {
		sort.Slice(paths, func(i, j int) bool {
			return paths[i].Cmp(&paths[j]) < 0
		})
	}

	for _, key := range paths {
		if err := callback(key, ns.Nodes[key]); err != nil {
			return err
		}
	}

	return nil
}

// Merges the other node set into the current node set.
// The owner of both node sets must be the same.
func (ns *NodeSet) MergeSet(other *NodeSet) error {
	if ns.Owner != other.Owner {
		return fmt.Errorf("cannot merge node sets with different owners %x-%x", ns.Owner, other.Owner)
	}
	maps.Copy(ns.Nodes, other.Nodes)
	ns.updates += other.updates
	ns.deletes += other.deletes
	return nil
}

// Adds a set of nodes to the current node set.
func (ns *NodeSet) Merge(owner felt.Felt, other map[trieutils.BitArray]TrieNode) error {
	if ns.Owner != owner {
		return fmt.Errorf("cannot merge node sets with different owners %x-%x", ns.Owner, owner)
	}

	for path, node := range other {
		prev, ok := ns.Nodes[path]
		if ok { // node already exists, revoke the counter first
			if _, ok := prev.(*DeletedNode); ok {
				ns.deletes -= 1
			} else {
				ns.updates -= 1
			}
		}
		// overwrite the existing node (if it exists)
		if _, ok := node.(*DeletedNode); ok {
			ns.deletes += 1
		} else {
			ns.updates += 1
		}
		ns.Nodes[path] = node
	}

	return nil
}
