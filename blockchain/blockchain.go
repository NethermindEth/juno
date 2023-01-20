package blockchain

import "sync"

// Blockchain is responsible for keeping track of all things related to the StarkNet blockchain
type Blockchain struct {
	sync.RWMutex

	height uint64
	// todo: much more
}

func NewBlockchain() *Blockchain {
	return &Blockchain{
		RWMutex: sync.RWMutex{},
		height:  0,
	}
}

// Height returns the latest block height
func (s *Blockchain) Height() uint64 {
	s.RLock()
	defer s.RUnlock()
	return s.height
}
