package trie

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
)

// ProofSet represents a set of trie nodes used in a Merkle proof verification process.
// Rather than relying on only either map of list, ProofSet provides both for the following reasons:
// - map allows for unique node insertion
// - list allows for ordered iteration over the proof nodes
// It also supports concurrent read and write operations.
type ProofSet struct {
	nodeSet  map[felt.Felt]ProofNode
	nodeList []ProofNode
	size     int
	lock     sync.RWMutex
}

func NewProofSet() *ProofSet {
	return &ProofSet{
		nodeSet: make(map[felt.Felt]ProofNode),
	}
}

func (ps *ProofSet) Put(key felt.Felt, node ProofNode) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	ps.nodeSet[key] = node
	ps.nodeList = append(ps.nodeList, node)
	ps.size++
}

func (ps *ProofSet) Get(key felt.Felt) (ProofNode, bool) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	node, ok := ps.nodeSet[key]
	return node, ok
}

func (ps *ProofSet) Size() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.size
}

// List returns a shallow copy of the proof set's node list.
func (ps *ProofSet) List() []ProofNode {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	nodes := make([]ProofNode, len(ps.nodeList))
	copy(nodes, ps.nodeList)

	return nodes
}
