package starknetrpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/clients/timeout"
	"github.com/NethermindEth/juno/utils/log"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"go.uber.org/zap"
)

const (
	clientName          = "rpcsync"
	maxIdleConnsPerHost = 64
)

type Client struct {
	url        string
	rpc        *gethrpc.Client
	timeouts   atomic.Pointer[timeout.Timeouts]
	maxRetries int
	maxWait    time.Duration
	minWait    time.Duration
	logger     log.StructuredLogger
	listener   EventListener
}

var _ timeout.Client = (*Client)(nil)

func New(
	ctx context.Context,
	clientURL *url.URL,
	initialTimeouts *timeout.Timeouts,
	logger log.StructuredLogger,
	opts ...Option,
) (*Client, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = maxIdleConnsPerHost
	dialOpts := []gethrpc.ClientOption{
		gethrpc.WithHTTPClient(&http.Client{Transport: transport}),
		// Classes can be 16 MB, so the WebSocket read limit is lifted.
		gethrpc.WithWebsocketMessageSizeLimit(0),
	}
	if o.userAgent != "" {
		dialOpts = append(dialOpts, gethrpc.WithHeader("User-Agent", o.userAgent))
	}
	rpcClient, err := gethrpc.DialOptions(ctx, clientURL.String(), dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("dialing %s: %w", clientURL, err)
	}

	client := &Client{
		url:        clientURL.String(),
		rpc:        rpcClient,
		maxRetries: o.maxRetries,
		maxWait:    o.maxWait,
		minWait:    o.minWait,
		logger:     logger,
		listener:   o.listener,
	}
	client.timeouts.Store(initialTimeouts)
	return client, nil
}

func (c *Client) Close() {
	c.rpc.Close()
}

func (c *Client) Name() string {
	return clientName
}

func (c *Client) Timeouts() string {
	return c.timeouts.Load().String()
}

func (c *Client) SetTimeouts(timeouts []time.Duration, fixed bool) {
	c.timeouts.Store(timeout.New(timeouts, fixed))
}

func (c *Client) Call[P params, R any](ctx context.Context, m method[P, R], p P) (*R, error) {
	var result R
	retries, wait := 0, c.minWait
	for {
		err := c.send(ctx, m, p, &result)
		if err == nil {
			return &result, nil
		}
		if !retryable(err) {
			return nil, err
		}
		if retries == c.maxRetries {
			return nil, fmt.Errorf("giving up after %d retries: %w", c.maxRetries, err)
		}
		retries++
		c.logRetry(m.name, wait, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		wait = min(wait*2, c.maxWait)
	}
}

func (c *Client) send[P params, R any](
	ctx context.Context,
	m method[P, R],
	p P,
	result *R,
) error {
	timeouts := c.timeouts.Load()
	ctx, cancel := context.WithTimeout(ctx, timeouts.GetCurrentTimeout())
	start := time.Now()
	err := c.rpc.CallContext(ctx, result, m.name, p.args()...)
	took := time.Since(start)
	cancel()

	switch {
	case err == nil:
		timeouts.DecreaseTimeout()
	case errors.Is(err, context.DeadlineExceeded):
		timeouts.IncreaseTimeout()
	}
	c.listener.OnResponse(m.name, outcome(err), took)
	return err
}

func (c *Client) logRetry(method string, wait time.Duration, err error) {
	currentTimeout := c.timeouts.Load().GetCurrentTimeout()
	logf, hint := c.logger.Debug, zap.Skip()
	if currentTimeout >= timeout.MediumGrowThreshold {
		logf, hint = c.logger.Warn, zap.String("hint",
			`Set --http-update-port and --http-update-host flags and `+
				`make a PUT request to "/feeder/timeouts" with the specified timeouts`,
		)
	}
	logf("Failed request to RPC sync node, retrying...",
		zap.String("url", log.SanitizeString(c.url)),
		zap.String("method", method),
		zap.String("retryAfter", wait.String()),
		zap.Error(err),
		zap.String("timeout", currentTimeout.String()),
		hint,
	)
}
