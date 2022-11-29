package blockchain

import "sync"

// Blockchain is responsible for keeping track of all things related to the StarkNet blockchain
type Blockchain struct {
	lock *sync.RWMutex

	height uint64
	// todo: much more
}

func NewBlockchain() *Blockchain {
	return &Blockchain{
		lock:   new(sync.RWMutex),
		height: 0,
	}
}

// Height returns the latest block height
func (s *Blockchain) Height() uint64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.height
}
