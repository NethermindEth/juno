package trienode

import (
	"fmt"
	"maps"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

// Contains a set of nodes, which are indexed by their path in the trie.
// It is not thread safe.
type NodeSet struct {
	Owner   felt.Address // The owner (i.e. contract address)
	Nodes   map[trieutils.Path]TrieNode
	updates int // the count of updated and inserted nodes
	deletes int // the count of deleted nodes
}

func NewNodeSet(owner felt.Address) NodeSet {
	return NodeSet{
		Owner: owner,
		Nodes: make(map[trieutils.Path]TrieNode),
	}
}

func (ns *NodeSet) Add(key *trieutils.Path, node TrieNode) {
	if _, ok := node.(*DeletedNode); ok {
		ns.deletes += 1
	} else {
		ns.updates += 1
	}
	ns.Nodes[*key] = node
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
func (ns *NodeSet) Merge(owner felt.Address, other map[trieutils.Path]TrieNode) error {
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

type MergeNodeSet struct {
	OwnerSet  *NodeSet                  // the node set of contract or class nodes
	ChildSets map[felt.Address]*NodeSet // each node set is indexed by the owner
}

func NewMergeNodeSet(nodes *NodeSet) *MergeNodeSet {
	ns := &MergeNodeSet{
		OwnerSet:  &NodeSet{Nodes: make(map[trieutils.Path]TrieNode)},
		ChildSets: make(map[felt.Address]*NodeSet),
	}
	if nodes == nil {
		return ns
	}
	if (*felt.Felt)(&nodes.Owner).IsZero() {
		ns.OwnerSet = nodes
	} else {
		ns.ChildSets[nodes.Owner] = nodes
	}
	return ns
}

func (m *MergeNodeSet) Merge(other *NodeSet) error {
	if (*felt.Felt)(&other.Owner).IsZero() {
		return m.OwnerSet.Merge(other.Owner, other.Nodes)
	}

	subset, present := m.ChildSets[other.Owner]
	if present {
		return subset.Merge(other.Owner, other.Nodes)
	}
	m.ChildSets[other.Owner] = other

	return nil
}

func (m *MergeNodeSet) Flatten() (
	map[trieutils.Path]TrieNode,
	map[felt.Address]map[trieutils.Path]TrieNode,
) {
	childFlat := make(map[felt.Address]map[trieutils.Path]TrieNode, len(m.ChildSets))
	for owner, set := range m.ChildSets {
		childFlat[owner] = set.Nodes
	}
	return m.OwnerSet.Nodes, childFlat
}
