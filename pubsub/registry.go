package pubsub

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sync"

	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/event"
)

var ErrNotFound = errors.New("subscription not found")

type Registry struct {
	log utils.SimpleLogger

	mu            sync.Mutex // protects subscriptions
	subscriptions map[uint64]event.Subscription
}

func New(log utils.SimpleLogger) *Registry {
	return &Registry{
		subscriptions: make(map[uint64]event.Subscription),
		log:           log,
	}
}

func getRandomID() uint64 {
	var n uint64
	for err := binary.Read(rand.Reader, binary.LittleEndian, &n); err != nil; {
	}
	return n
}

func (r *Registry) Add(ctx context.Context, sub event.Subscription) uint64 {
	id := getRandomID()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subscriptions[id] = sub
	return id
}

func (r *Registry) Delete(id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sub, ok := r.subscriptions[id]
	if !ok {
		return ErrNotFound
	}
	sub.Unsubscribe()
	delete(r.subscriptions, id)
	return nil
}
