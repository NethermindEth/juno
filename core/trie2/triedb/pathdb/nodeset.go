package pathdb

import (
	"maps"
	"math"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

// Contains the set of trie nodes for all the trie types
type nodeSet struct {
	classNodes           classNodesMap
	contractNodes        contractNodesMap
	contractStorageNodes contractStorageNodesMap

	size uint64
}

func newNodeSet(classNodes classNodesMap, contractNodes contractNodesMap, contractStorageNodes contractStorageNodesMap) *nodeSet {
	ns := &nodeSet{
		classNodes:           make(classNodesMap, len(classNodes)),
		contractNodes:        make(contractNodesMap, len(contractNodes)),
		contractStorageNodes: make(contractStorageNodesMap, len(contractStorageNodes)),
	}

	maps.Copy(ns.classNodes, classNodes)
	maps.Copy(ns.contractNodes, contractNodes)

	for owner, nodes := range contractStorageNodes {
		ns.contractStorageNodes[owner] = maps.Clone(nodes)
	}

	ns.computeSize()
	return ns
}

func (s *nodeSet) node(owner felt.Felt, path trieutils.Path, isClass bool) (trienode.TrieNode, bool) {
	// class trie nodes
	if isClass {
		node, ok := s.classNodes[path]
		return node, ok
	}

	// contract trie nodes
	if owner.IsZero() {
		node, ok := s.contractNodes[path]
		return node, ok
	}

	// contract storage trie nodes
	nodes, ok := s.contractStorageNodes[owner]
	if !ok {
		return nil, false
	}

	node, ok := nodes[path]
	return node, ok
}

func (s *nodeSet) merge(other *nodeSet) {
	var delta int64

	for path, n := range other.classNodes {
		if ori, ok := s.classNodes[path]; !ok {
			delta += int64(len(n.Blob()) + trieutils.PathSize)
		} else {
			delta += int64(len(n.Blob()) - len(ori.Blob()))
		}
		s.classNodes[path] = n
	}

	for path, n := range other.contractNodes {
		if ori, ok := s.contractNodes[path]; !ok {
			delta += int64(len(n.Blob()) + trieutils.PathSize)
		} else {
			delta += int64(len(n.Blob()) - len(ori.Blob()))
		}
		s.contractNodes[path] = n
	}

	for owner, nodes := range other.contractStorageNodes {
		current, exist := s.contractStorageNodes[owner]
		if !exist {
			for _, n := range nodes {
				delta += ownerSize + int64(len(n.Blob())+trieutils.PathSize)
			}
			s.contractStorageNodes[owner] = maps.Clone(nodes)
			continue
		}

		for path, n := range nodes {
			if ori, ok := current[path]; !ok {
				delta += ownerSize + int64(len(n.Blob())+trieutils.PathSize)
			} else {
				delta += int64(len(n.Blob()) - len(ori.Blob()))
			}
			current[path] = n
		}
		s.contractStorageNodes[owner] = current
	}

	s.updateSize(delta)
}

func (s *nodeSet) write(w db.KeyValueWriter, cleans *cleanCache) error {
	return writeNodes(w, s.classNodes, s.contractNodes, s.contractStorageNodes, cleans)
}

func (s *nodeSet) computeSize() {
	var size uint64

	for _, node := range s.classNodes {
		size += uint64(len(node.Blob()) + trieutils.PathSize)
	}

	for _, node := range s.contractNodes {
		size += uint64(len(node.Blob()) + trieutils.PathSize)
	}

	for _, nodes := range s.contractStorageNodes {
		size += ownerSize
		for _, node := range nodes {
			size += uint64(len(node.Blob()) + trieutils.PathSize)
		}
	}

	s.size = uint64(size)
}

func (s *nodeSet) updateSize(delta int64) {
	if delta > 0 && s.size > math.MaxUint64-uint64(delta) { // Overflow occurred
		s.size = math.MaxUint64 // Set to max uint64 value
		return
	} else if delta < 0 && uint64(-delta) > s.size { // Underflow occurred
		s.size = 0
		return
	}

	// Safe to update
	s.size += uint64(delta)
}

// Returns the approximate size of the node set to be stored in the db
func (s *nodeSet) dbSize() int {
	var size int

	size += len(s.classNodes)
	size += len(s.contractNodes)
	for _, nodes := range s.contractStorageNodes {
		size += len(nodes)
	}

	return size + int(s.size)
}

func (s *nodeSet) reset() {
	s.size = 0
	s.classNodes = make(classNodesMap)
	s.contractNodes = make(contractNodesMap)
	s.contractStorageNodes = make(contractStorageNodesMap)
}

func writeNodes(
	w db.KeyValueWriter,
	classNodes classNodesMap,
	contractNodes contractNodesMap,
	contractStorageNodes contractStorageNodesMap,
	cleans *cleanCache,
) error {
	for path, n := range classNodes {
		if _, deleted := n.(*trienode.DeletedNode); deleted {
			if err := trieutils.DeleteNodeByPath(w, db.ClassTrie, felt.Zero, path, n.IsLeaf()); err != nil {
				return err
			}
			cleans.deleteNode(felt.Zero, path, true)
		} else {
			if err := trieutils.WriteNodeByPath(w, db.ClassTrie, felt.Zero, path, n.IsLeaf(), n.Blob()); err != nil {
				return err
			}
			cleans.putNode(felt.Zero, path, true, n.Blob())
		}
	}

	for path, n := range contractNodes {
		if _, deleted := n.(*trienode.DeletedNode); deleted {
			if err := trieutils.DeleteNodeByPath(w, db.ContractTrieContract, felt.Zero, path, n.IsLeaf()); err != nil {
				return err
			}
			cleans.deleteNode(felt.Zero, path, false)
		} else {
			if err := trieutils.WriteNodeByPath(w, db.ContractTrieContract, felt.Zero, path, n.IsLeaf(), n.Blob()); err != nil {
				return err
			}
			cleans.putNode(felt.Zero, path, false, n.Blob())
		}
	}

	for owner, nodes := range contractStorageNodes {
		for path, n := range nodes {
			if _, deleted := n.(*trienode.DeletedNode); deleted {
				if err := trieutils.DeleteNodeByPath(w, db.ContractTrieStorage, owner, path, n.IsLeaf()); err != nil {
					return err
				}
				cleans.deleteNode(owner, path, false)
			} else {
				if err := trieutils.WriteNodeByPath(w, db.ContractTrieStorage, owner, path, n.IsLeaf(), n.Blob()); err != nil {
					return err
				}
				cleans.putNode(owner, path, false, n.Blob())
			}
		}
	}

	return nil
}
