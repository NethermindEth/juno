package contracts

import (
	"context"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

type IteratorFilterer interface {
	FilterLogs(ctx context.Context, query ethereum.FilterQuery) (LogIterator, error)
	SubscribeFilterLogs(ctx context.Context, query ethereum.FilterQuery) (LogIterator, error)
}

type iteratorFilterer struct {
	filterer ethereum.LogFilterer
}

func newIteratorFilterer(filterer ethereum.LogFilterer) *iteratorFilterer {
	return &iteratorFilterer{
		filterer: filterer,
	}
}

func (f *iteratorFilterer) FilterLogs(ctx context.Context, query ethereum.FilterQuery) (LogIterator, error) {
	buff, err := f.filterer.FilterLogs(ctx, query)
	if err != nil {
		return nil, err
	}
	logs := make(chan types.Log, 128)
	sub := event.NewSubscription(func(quit <-chan struct{}) error {
		for _, log := range buff {
			select {
			case logs <- log:
			case <-quit:
				return nil
			}
		}
		return nil
	})
	return &iterator{logs: logs, sub: sub}, nil
}

func (f *iteratorFilterer) SubscribeFilterLogs(ctx context.Context, query ethereum.FilterQuery) (LogIterator, error) {
	sink := make(chan types.Log)
	sub, err := f.filterer.SubscribeFilterLogs(ctx, query, sink)
	if err != nil {
		return nil, err
	}
	return &iterator{logs: sink, sub: sub}, nil
}
