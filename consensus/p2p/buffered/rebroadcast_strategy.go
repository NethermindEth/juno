package buffered

import (
	"iter"
	"maps"
	"time"

	"google.golang.org/protobuf/proto"
)

type rebroadcastMessages struct {
	trigger  <-chan time.Time
	messages iter.Seq[[]byte]
}

type RebroadcastStrategy[M proto.Message] interface {
	Receive(msg M, msgBytes []byte) rebroadcastMessages
}

type rebroadcastStrategy[M proto.Message, K comparable] struct {
	rebroadcastInterval time.Duration
	getKey              func(M) K
	cache               map[K][]byte
}

func NewRebroadcastStrategy[M proto.Message, K comparable](rebroadcastInterval time.Duration, getKey func(M) K) RebroadcastStrategy[M] {
	return &rebroadcastStrategy[M, K]{
		rebroadcastInterval: rebroadcastInterval,
		getKey:              getKey,
		cache:               make(map[K][]byte),
	}
}

func (s *rebroadcastStrategy[M, K]) Receive(msg M, msgBytes []byte) rebroadcastMessages {
	key := s.getKey(msg)
	s.cache[key] = msgBytes

	return rebroadcastMessages{
		trigger:  time.Tick(s.rebroadcastInterval),
		messages: maps.Values(s.cache),
	}
}
