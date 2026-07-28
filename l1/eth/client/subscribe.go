package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/l1/eth"
	"go.uber.org/zap"
)

// Subscription mirrors go-ethereum's event.Subscription, as a drop-in for
// callers migrating off that package.
type Subscription interface {
	Err() <-chan error
	Unsubscribe()
}

// SubscribeLogs uses ctx only for the subscribe call itself — afterwards the
// sub lives until Unsubscribe or a transport failure (go-ethereum semantics).
func (c *Client) SubscribeLogs(
	ctx context.Context,
	q FilterQuery,
	sink chan<- *eth.Log,
) (Subscription, error) {
	return c.tr.subscribeLogs(ctx, q, sink)
}

type wsLogSub struct {
	id        string // server-assigned; set during the subscribe handshake
	transport *wsTransport
	sink      chan<- *eth.Log

	// cancelled is set under transport.mu by cancelPending when the caller's ctx fires;
	// registerSub checks it to avoid orphaning a server-side sub when the reply races
	// the cancellation.
	cancelled bool

	// logCh decouples the shared reader goroutine from this sub's decode+deliver work.
	logCh chan json.RawMessage

	// errCh is closed when the subscription terminates; a non-nil cause is sent before close.
	errCh chan error

	closed    chan struct{}
	closeOnce sync.Once
}

func (s *wsLogSub) Err() <-chan error { return s.errCh }

func (s *wsLogSub) Unsubscribe() {
	s.fail(nil)
	s.transport.mu.Lock()
	id := s.id
	s.id = ""
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
		s.transport.logger.Trace(
			"eth_unsubscribe failed",
			zap.String("subscription", id),
			zap.Error(err),
		)
	}
}

// fail(nil) is a clean shutdown (Unsubscribe); a non-nil cause is surfaced on Err().
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
				s.transport.removeSub(s)
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
		sub.closeOnce.Do(func() { close(sub.closed); close(sub.errCh) })
		return nil, fmt.Errorf("subscribing to logs: %w", err)
	}

	go sub.dispatch()
	return sub, nil
}
