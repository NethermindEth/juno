package trienode

import (
	"fmt"
	"maps"
	"sort"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/utils"
)

type path = utils.BitArray

type NodeSet struct {
	Owner   felt.Felt
	Nodes   map[path]*Node
	updates int
	deletes int
}

func NewNodeSet(owner felt.Felt) *NodeSet {
	return &NodeSet{Owner: owner, Nodes: make(map[path]*Node)}
}

func (ns *NodeSet) Add(key path, node *Node) {
	if node.IsDeleted() {
		ns.deletes += 1
	} else {
		ns.updates += 1
	}
	ns.Nodes[key] = node
}

func (ns *NodeSet) ForEach(desc bool, callback func(key path, node *Node)) {
	paths := make([]path, 0, len(ns.Nodes))
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
		callback(key, ns.Nodes[key])
	}
}

func (ns *NodeSet) MergeSet(other *NodeSet) error {
	if ns.Owner != other.Owner {
		return fmt.Errorf("cannot merge node sets with different owners %x-%x", ns.Owner, other.Owner)
	}
	maps.Copy(ns.Nodes, other.Nodes)
	ns.updates += other.updates
	ns.deletes += other.deletes
	return nil
}

func (ns *NodeSet) Merge(owner felt.Felt, other map[path]*Node) error {
	if ns.Owner != owner {
		return fmt.Errorf("cannot merge node sets with different owners %x-%x", ns.Owner, owner)
	}

	for path, node := range other {
		prev, ok := ns.Nodes[path]
		if ok { // node already exists, revoke the counter first
			if prev.IsDeleted() {
				ns.deletes -= 1
			} else {
				ns.updates -= 1
			}
		}
		// overwrite the existing node (if it exists)
		if node.IsDeleted() {
			ns.deletes += 1
		} else {
			ns.updates += 1
		}
		ns.Nodes[path] = node
	}

	return nil
}
