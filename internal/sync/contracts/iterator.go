package contracts

import (
	"github.com/ethereum/go-ethereum"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type LogIterator interface {
	Next() bool
	Log() ethtypes.Log
	Error() error
	Close() error
}

type iterator struct {
	log  ethtypes.Log
	logs chan ethtypes.Log     // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

func (it *iterator) Error() error {
	return it.fail
}

func (it *iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

func (it *iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.log = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.log = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

func (it *iterator) Log() ethtypes.Log {
	return it.log
}
