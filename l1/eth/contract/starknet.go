// Package contract hand-decodes the Starknet core L1 contract events
// juno consumes. It exists to remove the abigen + go-ethereum dependency
// from the L1 sync path. Only the event(s) juno actually subscribes to
// are implemented; calls into contract methods are not supported because
// juno doesn't make any.
package contract

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
)

// LogStateUpdateSigHash is keccak256("LogStateUpdate(uint256,int256,uint256)").
// Verified against the deployed Starknet core contract on Ethereum
// mainnet and against the go-ethereum abigen output that this package
// replaces.
var LogStateUpdateSigHash = eth.HashFromString(
	"0xd342ddf7a308dec111745b00315c14b7efb2bdae570a6856e088ed0c65a3576c",
)

// logStateUpdateDataLen is the byte length of the LogStateUpdate event's
// data section: three 32-byte words for (uint256 globalRoot,
// int256 blockNumber, uint256 blockHash). No indexed args, so all three
// fields are in data rather than topics.
const logStateUpdateDataLen = 3 * 32

// watchSinkBuffer is the per-subscription buffer between the raw log
// channel and the decoded-event sink. Sized so a slow consumer doesn't
// immediately stall the underlying transport reader.
const watchSinkBuffer = 64

// LogStateUpdate is the decoded form of the Starknet core contract's
// LogStateUpdate event.
//
//	event LogStateUpdate(uint256 globalRoot, int256 blockNumber, uint256 blockHash)
//
// BlockNumber is declared as int256 in Solidity but the bridge always
// emits non-negative values that fit in uint64, so we decode it as
// uint64 (low 8 bytes of the 32-byte word). globalRoot and blockHash
// are Starknet field elements packed into a uint256 slot, so they
// land in felt.Felt without going through *big.Int.
type LogStateUpdate struct {
	GlobalRoot  felt.Felt
	BlockNumber uint64
	BlockHash   felt.Felt

	// Raw is the underlying log envelope, preserving the L1 block
	// number where the event was emitted and the Removed flag set on
	// reorgs.
	Raw eth.Log
}

// ErrWrongTopic is returned when Decode is given a log whose first
// topic is not LogStateUpdateSigHash.
var ErrWrongTopic = errors.New("log topic is not LogStateUpdate")

// Decode parses a single eth.Log into a LogStateUpdate. The caller is
// expected to have prefiltered by topic and contract address; Decode
// re-checks the sig hash defensively.
func Decode(log *eth.Log) (*LogStateUpdate, error) {
	if len(log.Topics) == 0 || log.Topics[0] != LogStateUpdateSigHash {
		return nil, ErrWrongTopic
	}
	if len(log.Data) != logStateUpdateDataLen {
		return nil, fmt.Errorf("bad LogStateUpdate data length: got %d, want %d",
			len(log.Data), logStateUpdateDataLen)
	}
	ev := &LogStateUpdate{
		// Low 8 bytes of the 32-byte int256 slot. The upper 24 bytes
		// are silently dropped; the bridge always emits a block number
		// that fits in uint64.
		BlockNumber: binary.BigEndian.Uint64(log.Data[56:64]),
		Raw:         *log,
	}
	ev.GlobalRoot.SetBytes(log.Data[0:32])
	ev.BlockHash.SetBytes(log.Data[64:96])
	return ev, nil
}

// LogClient is the slice of *client.Client the contract decoder needs.
// Declared as an interface so tests can swap in a fake without dialling
// a real endpoint.
type LogClient interface {
	FilterLogs(ctx context.Context, q client.FilterQuery) ([]eth.Log, error)
	SubscribeLogs(
		ctx context.Context,
		q client.FilterQuery,
		sink chan<- *eth.Log,
	) (eth.Subscription, error)
}

// FilterLogStateUpdate returns every LogStateUpdate emitted by contract
// in the inclusive L1 block range [from, to].
func FilterLogStateUpdate(
	ctx context.Context,
	c LogClient,
	contract eth.Address,
	from uint64,
	to uint64,
) ([]*LogStateUpdate, error) {
	logs, err := c.FilterLogs(ctx, client.FilterQuery{
		FromBlock: &from,
		ToBlock:   &to,
		Addresses: []eth.Address{contract},
		Topics:    [][]eth.Hash{{LogStateUpdateSigHash}},
	})
	if err != nil {
		return nil, fmt.Errorf("filtering LogStateUpdate: %w", err)
	}
	out := make([]*LogStateUpdate, len(logs))
	for i := range logs {
		ev, derr := Decode(&logs[i])
		if derr != nil {
			return nil, fmt.Errorf("decoding LogStateUpdate: %w", derr)
		}
		out[i] = ev
	}
	return out, nil
}

// WatchLogStateUpdate subscribes to live LogStateUpdate events from
// contract. Decoded events are delivered on sink; the returned
// Subscription surfaces transport errors on Err() and releases the
// subscription on Unsubscribe.
//
// On a decode failure the watcher terminates with the decode error on
// Err() — a misformatted log can only mean we're talking to the wrong
// contract or to a buggy node.
func WatchLogStateUpdate(
	ctx context.Context,
	c LogClient,
	contract eth.Address,
	sink chan<- *LogStateUpdate,
) (eth.Subscription, error) {
	rawSink := make(chan *eth.Log, watchSinkBuffer)
	inner, err := c.SubscribeLogs(ctx, client.FilterQuery{
		Addresses: []eth.Address{contract},
		Topics:    [][]eth.Hash{{LogStateUpdateSigHash}},
	}, rawSink)
	if err != nil {
		return nil, fmt.Errorf("subscribing to LogStateUpdate: %w", err)
	}
	w := &stateUpdateWatcher{
		inner:  inner,
		errCh:  make(chan error, 1),
		closed: make(chan struct{}),
	}
	go w.run(rawSink, sink)
	return w, nil
}

// stateUpdateWatcher decorates an underlying log subscription with
// decoding into LogStateUpdate. Lifecycle is one-to-one with the inner
// subscription.
type stateUpdateWatcher struct {
	inner     eth.Subscription
	errCh     chan error
	closed    chan struct{}
	closeOnce sync.Once
}

func (w *stateUpdateWatcher) Err() <-chan error { return w.errCh }

func (w *stateUpdateWatcher) Unsubscribe() {
	w.fail(nil)
	w.inner.Unsubscribe()
}

func (w *stateUpdateWatcher) fail(cause error) {
	w.closeOnce.Do(func() {
		close(w.closed)
		if cause != nil {
			select {
			case w.errCh <- cause:
			default:
			}
		}
		close(w.errCh)
	})
}

func (w *stateUpdateWatcher) run(rawSink <-chan *eth.Log, sink chan<- *LogStateUpdate) {
	for {
		select {
		case <-w.closed:
			return
		case err, ok := <-w.inner.Err():
			if !ok {
				w.fail(nil)
				return
			}
			w.fail(err)
			return
		case raw := <-rawSink:
			ev, err := Decode(raw)
			if err != nil {
				w.fail(fmt.Errorf("decoding LogStateUpdate: %w", err))
				return
			}
			select {
			case sink <- ev:
			case <-w.closed:
				return
			}
		}
	}
}
