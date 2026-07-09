package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/l1/eth"
	"go.uber.org/zap"
)

// Mirrors go-ethereum's event.Subscription; drop-in for callers migrating off that package.
type Subscription interface {
	Err() <-chan error
	Unsubscribe()
}

// SubscribeLogs subscribes to live log events matching q. Incoming
// logs are delivered to sink as they arrive. The returned subscription
// surfaces transport errors on Err() and tears down the server-side
// subscription on Unsubscribe.
//
// ctx governs only the subscribe call. Once SubscribeLogs returns, the
// subscription's lifetime is controlled solely by Unsubscribe (or a
// transport failure) — cancelling ctx afterwards has no effect. This
// matches go-ethereum's ethclient semantics.
func (c *Client) SubscribeLogs(
	ctx context.Context,
	q FilterQuery,
	sink chan<- *eth.Log,
) (Subscription, error) {
	return c.tr.subscribeLogs(ctx, q, sink)
}

type wsLogSub struct {
	id        string // server-assigned subscription id; set during subscribe handshake
	transport *wsTransport
	sink      chan<- *eth.Log

	// cancelled is set (under transport.mu) by cancelPending when the
	// subscribing caller's ctx fires. dispatchResponse checks it before
	// registering the sub, closing the window where a subscribe reply
	// races the ctx cancellation and orphans the server-side sub.
	cancelled bool

	// logCh carries raw eth_subscription "result" payloads from the
	// reader goroutine to the per-sub dispatch goroutine.
	logCh chan json.RawMessage

	// errCh is the user-facing Err() channel. It is closed when the
	// subscription terminates; a non-nil cause is sent before close.
	errCh chan error

	closed    chan struct{}
	closeOnce sync.Once
}

func (s *wsLogSub) Err() <-chan error { return s.errCh }

func (s *wsLogSub) Unsubscribe() {
	// Two-step teardown: stop the per-sub dispatch goroutine first,
	// then best-effort tell the server to release its side. The order
	// matters because if the server fails to ack, we still want our
	// local resources gone.
	s.fail(nil)
	s.transport.mu.Lock()
	id := s.id
	if s.transport.subs != nil && id != "" {
		delete(s.transport.subs, id)
	}
	s.transport.mu.Unlock()

	if id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), wsUnsubscribeTimeout)
	defer cancel()
	if _, err := s.transport.call(ctx, "eth_unsubscribe", id); err != nil {
		// Server-side cleanup may have already happened (transport
		// closed) or the server may simply have dropped the call.
		// Either way local state is already torn down; debug-log so
		// it's visible without being noisy.
		s.transport.logger.Trace("ws: eth_unsubscribe failed",
			zap.String("subscription", id),
			zap.Error(err),
		)
	}
}

// fail terminates the subscription. cause may be nil for a clean
// shutdown (Unsubscribe); otherwise it is the error surfaced to
// Err() before errCh is closed.
func (s *wsLogSub) fail(cause error) {
	s.closeOnce.Do(func() {
		close(s.closed)
		if cause != nil {
			select {
			case s.errCh <- cause:
			default:
			}
		}
		close(s.errCh)
	})
}

func (s *wsLogSub) dispatch() {
	for {
		select {
		case raw := <-s.logCh:
			var log eth.Log
			if err := json.Unmarshal(raw, &log); err != nil {
				s.fail(fmt.Errorf("decoding log: %w", err))
				return
			}
			select {
			case s.sink <- &log:
			case <-s.closed:
				return
			}
		case <-s.closed:
			return
		}
	}
}

func (t *wsTransport) subscribeLogs(
	ctx context.Context,
	q FilterQuery,
	sink chan<- *eth.Log,
) (*wsLogSub, error) {
	sub := &wsLogSub{
		transport: t,
		sink:      sink,
		logCh:     make(chan json.RawMessage, wsLogSubBuffer),
		errCh:     make(chan error, 1),
		closed:    make(chan struct{}),
	}

	if _, err := t.callWithSubReg(ctx, "eth_subscribe", sub, "logs", q); err != nil {
		// Subscribe call failed; drop the pre-registered sub so the
		// dispatcher never runs.
		sub.closeOnce.Do(func() { close(sub.closed); close(sub.errCh) })
		return nil, fmt.Errorf("subscribing to logs: %w", err)
	}

	go sub.dispatch()
	return sub, nil
}
