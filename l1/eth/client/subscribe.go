package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/l1/eth"
)

// SubscribeLogs subscribes to live log events matching q. Incoming
// logs are delivered to sink as they arrive. The returned subscription
// surfaces transport errors on Err() and tears down the server-side
// subscription on Unsubscribe.
func (c *Client) SubscribeLogs(
	ctx context.Context,
	q FilterQuery,
	sink chan<- *eth.Log,
) (eth.Subscription, error) {
	return c.tr.(*wsTransport).subscribeLogs(ctx, q, sink)
}

// wsLogSub is the concrete subscription returned by SubscribeLogs.
// It satisfies eth.Subscription.
type wsLogSub struct {
	id        string // server-assigned subscription id; set during subscribe handshake
	transport *wsTransport
	sink      chan<- *eth.Log

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
	if s.transport.subs != nil && s.id != "" {
		delete(s.transport.subs, s.id)
	}
	s.transport.mu.Unlock()

	if s.id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), wsUnsubscribeTimeout)
	defer cancel()
	_, _ = s.transport.call(ctx, "eth_unsubscribe", s.id)
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

// dispatch decodes log payloads and forwards them to sink. Runs until
// closed signals termination or a payload fails to decode.
func (s *wsLogSub) dispatch() {
	for {
		select {
		case raw := <-s.logCh:
			var log eth.Log
			if err := json.Unmarshal(raw, &log); err != nil {
				s.fail(fmt.Errorf("decode log: %w", err))
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
	ctx context.Context, q FilterQuery, sink chan<- *eth.Log,
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
		return nil, fmt.Errorf("eth_subscribe: %w", err)
	}

	go sub.dispatch()
	return sub, nil
}
