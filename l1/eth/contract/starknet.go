// Package contract hand-decodes the Starknet core L1 contract events
// juno consumes. It exists to remove the abigen + go-ethereum dependency
// from the L1 sync path. Only the event(s) juno actually subscribes to
// are implemented; calls into contract methods are not supported because
// juno doesn't make any.
package contract

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"

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

// LogStateUpdate is the decoded form of the Starknet core contract's
// LogStateUpdate event.
//
//	event LogStateUpdate(uint256 globalRoot, int256 blockNumber, uint256 blockHash)
//
// BlockNumber is declared as int256 in Solidity and decoded with sign
// extension; in practice the bridge always emits non-negative values.
type LogStateUpdate struct {
	GlobalRoot  *big.Int
	BlockNumber *big.Int
	BlockHash   *big.Int

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
		return nil, fmt.Errorf("LogStateUpdate: bad data length %d, want %d",
			len(log.Data), logStateUpdateDataLen)
	}
	return &LogStateUpdate{
		GlobalRoot:  new(big.Int).SetBytes(log.Data[0:32]),
		BlockNumber: decodeInt256(log.Data[32:64]),
		BlockHash:   new(big.Int).SetBytes(log.Data[64:96]),
		Raw:         *log,
	}, nil
}

// decodeInt256 reads a 32-byte big-endian two's-complement signed
// integer into *big.Int. If the high bit is set, subtract 2^256 to
// recover the negative value — matching go-ethereum's ABI decoder.
func decodeInt256(b []byte) *big.Int {
	n := new(big.Int).SetBytes(b)
	if b[0]&0x80 != 0 {
		twoTo256 := new(big.Int).Lsh(big.NewInt(1), 256)
		n.Sub(n, twoTo256)
	}
	return n
}

// LogClient is the slice of *client.Client the contract decoder needs.
// Declared as an interface so tests can swap in a fake without dialling
// a real endpoint.
type LogClient interface {
	FilterLogs(ctx context.Context, q client.FilterQuery) ([]eth.Log, error)
	SubscribeLogs(ctx context.Context, q client.FilterQuery, sink chan<- *eth.Log) (eth.Subscription, error)
}

// FilterLogStateUpdate returns every LogStateUpdate emitted by contract
// in the inclusive L1 block range [from, to].
func FilterLogStateUpdate(
	ctx context.Context, c LogClient, contract eth.Address, from, to uint64,
) ([]*LogStateUpdate, error) {
	logs, err := c.FilterLogs(ctx, client.FilterQuery{
		FromBlock: from,
		ToBlock:   to,
		Addresses: []eth.Address{contract},
		Topics:    [][]eth.Hash{{LogStateUpdateSigHash}},
	})
	if err != nil {
		return nil, fmt.Errorf("FilterLogStateUpdate: %w", err)
	}
	out := make([]*LogStateUpdate, 0, len(logs))
	for i := range logs {
		ev, derr := Decode(&logs[i])
		if derr != nil {
			return nil, fmt.Errorf("FilterLogStateUpdate: %w", derr)
		}
		out = append(out, ev)
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
	ctx context.Context, c LogClient, contract eth.Address, sink chan<- *LogStateUpdate,
) (eth.Subscription, error) {
	rawSink := make(chan *eth.Log, 64)
	inner, err := c.SubscribeLogs(ctx, client.FilterQuery{
		Addresses: []eth.Address{contract},
		Topics:    [][]eth.Hash{{LogStateUpdateSigHash}},
	}, rawSink)
	if err != nil {
		return nil, fmt.Errorf("WatchLogStateUpdate: %w", err)
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
				w.fail(fmt.Errorf("WatchLogStateUpdate decode: %w", err))
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

