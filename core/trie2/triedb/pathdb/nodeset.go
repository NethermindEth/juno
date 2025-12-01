package pathdb

import (
	"io"
	"maps"
	"math"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

// Contains the set of trie nodes for all the trie types
type nodeSet struct {
	classNodes           classNodesMap
	contractNodes        contractNodesMap
	contractStorageNodes contractStorageNodesMap

	size uint64 // Approximate size of the node set
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

func (s *nodeSet) node(
	owner *felt.Address, path *trieutils.Path, isClass bool,
) (trienode.TrieNode, bool) {
	// class trie nodes
	if isClass {
		node, ok := s.classNodes[*path]
		return node, ok
	}

	// contract trie nodes
	if owner.IsZero() {
		node, ok := s.contractNodes[*path]
		return node, ok
	}

	// contract storage trie nodes
	nodes, ok := s.contractStorageNodes[*owner]
	if !ok {
		return nil, false
	}

	node, ok := nodes[*path]
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

// Commits the in-memory nodes to the DB and updates the cache
func (s *nodeSet) write(w db.KeyValueWriter, cleans *cleanCache) error {
	return writeNodes(w, s.classNodes, s.contractNodes, s.contractStorageNodes, cleans)
}

// Represents a node when writing to the journal
type journalNode struct {
	Path   []byte
	Blob   []byte
	Hash   felt.Felt
	IsLeaf bool
}

// Represents a set of journal nodes for a specific trie type and owner
type journalNodes struct {
	TrieType trieutils.TrieType
	Owner    felt.Address
	Nodes    []journalNode
}

// Represents all the journal nodeset
type JournalNodeSet struct {
	Nodes []journalNodes
}

// Writes the node set to the journal
func (s *nodeSet) encode(w io.Writer) error {
	nodes := make([]journalNodes, 0, len(s.contractStorageNodes)+2) // 2 because of class and contract nodes

	classEntry := journalNodes{
		TrieType: trieutils.Class,
		Owner:    felt.Address{},
		Nodes:    make([]journalNode, 0, len(s.classNodes)),
	}
	for path, n := range s.classNodes {
		classEntry.Nodes = append(classEntry.Nodes, journalNode{
			Path:   path.EncodedBytes(),
			Blob:   n.Blob(),
			Hash:   n.Hash(),
			IsLeaf: n.IsLeaf(),
		})
	}
	nodes = append(nodes, classEntry)

	contractEntry := journalNodes{
		TrieType: trieutils.Contract,
		Owner:    felt.Address{},
		Nodes:    make([]journalNode, 0, len(s.contractNodes)),
	}
	for path, n := range s.contractNodes {
		contractEntry.Nodes = append(contractEntry.Nodes, journalNode{
			Path:   path.EncodedBytes(),
			Blob:   n.Blob(),
			Hash:   n.Hash(),
			IsLeaf: n.IsLeaf(),
		})
	}
	nodes = append(nodes, contractEntry)

	for owner, sn := range s.contractStorageNodes {
		entry := journalNodes{
			TrieType: trieutils.ContractStorage,
			Owner:    owner,
			Nodes:    make([]journalNode, 0, len(sn)),
		}
		for path, n := range sn {
			entry.Nodes = append(entry.Nodes, journalNode{
				Path:   path.EncodedBytes(),
				Blob:   n.Blob(),
				Hash:   n.Hash(),
				IsLeaf: n.IsLeaf(),
			})
		}
		nodes = append(nodes, entry)
	}

	enc, err := encoder.Marshal(&JournalNodeSet{Nodes: nodes})
	if err != nil {
		return err
	}

	_, err = w.Write(enc)
	return err
}

// Decodes the journal nodeset from the encoded bytes
func (s *nodeSet) decode(data []byte) error {
	var encoded JournalNodeSet
	if err := encoder.Unmarshal(data, &encoded); err != nil {
		return err
	}
	s.classNodes = make(classNodesMap)
	s.contractNodes = make(contractNodesMap)
	s.contractStorageNodes = make(contractStorageNodesMap)

	for _, entry := range encoded.Nodes {
		switch entry.TrieType {
		case trieutils.Class:
			for _, n := range entry.Nodes {
				var path trieutils.Path
				if err := path.UnmarshalBinary(n.Path); err != nil {
					return err
				}
				s.classNodes[path] = decodeJournalNode(n.Blob, &n.Hash, n.IsLeaf)
			}
		case trieutils.Contract:
			for _, n := range entry.Nodes {
				var path trieutils.Path
				if err := path.UnmarshalBinary(n.Path); err != nil {
					return err
				}
				s.contractNodes[path] = decodeJournalNode(n.Blob, &n.Hash, n.IsLeaf)
			}
		case trieutils.ContractStorage:
			s.contractStorageNodes[entry.Owner] = make(map[trieutils.Path]trienode.TrieNode)
			for _, n := range entry.Nodes {
				var path trieutils.Path
				if err := path.UnmarshalBinary(n.Path); err != nil {
					return err
				}
				s.contractStorageNodes[entry.Owner][path] = decodeJournalNode(n.Blob, &n.Hash, n.IsLeaf)
			}
		}
	}
	s.computeSize()

	return nil
}

// Computes the approximate size of the node set in bytes
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

	s.size = size
}

// Updates the size of the node set
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

//nolint:dupl
func writeNodes(
	w db.KeyValueWriter,
	classNodes classNodesMap,
	contractNodes contractNodesMap,
	contractStorageNodes contractStorageNodesMap,
	cleans *cleanCache,
) error {
	for path, n := range classNodes {
		if _, deleted := n.(*trienode.DeletedNode); deleted {
			err := trieutils.DeleteNodeByPath(
				w,
				db.ClassTrie,
				&felt.Address{},
				&path,
				n.IsLeaf(),
			)
			if err != nil {
				return err
			}
			cleans.deleteNode(&felt.Address{}, &path, true)
		} else {
			err := trieutils.WriteNodeByPath(
				w,
				db.ClassTrie,
				&felt.Address{},
				&path,
				n.IsLeaf(),
				n.Blob(),
			)
			if err != nil {
				return err
			}
			cleans.putNode(&felt.Address{}, &path, true, n.Blob())
		}
	}

	for path, n := range contractNodes {
		if _, deleted := n.(*trienode.DeletedNode); deleted {
			err := trieutils.DeleteNodeByPath(
				w,
				db.ContractTrieContract,
				&felt.Address{},
				&path,
				n.IsLeaf(),
			)
			if err != nil {
				return err
			}
			cleans.deleteNode(&felt.Address{}, &path, false)
		} else {
			err := trieutils.WriteNodeByPath(
				w,
				db.ContractTrieContract,
				&felt.Address{},
				&path,
				n.IsLeaf(),
				n.Blob(),
			)
			if err != nil {
				return err
			}
			cleans.putNode(&felt.Address{}, &path, false, n.Blob())
		}
	}

	for owner, nodes := range contractStorageNodes {
		for path, n := range nodes {
			if _, deleted := n.(*trienode.DeletedNode); deleted {
				err := trieutils.DeleteNodeByPath(w, db.ContractTrieStorage, &owner, &path, n.IsLeaf())
				if err != nil {
					return err
				}
				cleans.deleteNode(&owner, &path, false)
			} else {
				err := trieutils.WriteNodeByPath(w, db.ContractTrieStorage, &owner, &path, n.IsLeaf(), n.Blob())
				if err != nil {
					return err
				}
				cleans.putNode(&owner, &path, false, n.Blob())
			}
		}
	}

	return nil
}

func decodeJournalNode(blob []byte, hash *felt.Felt, isLeaf bool) trienode.TrieNode {
	if len(blob) == 0 {
		return trienode.NewDeleted(isLeaf)
	}

	if isLeaf {
		return trienode.NewLeaf(*hash, blob)
	}
	return trienode.NewNonLeaf(*hash, blob)
}
