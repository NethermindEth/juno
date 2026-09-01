package feeder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

const (
	blockNumberArg = "blockNumber"
	classHashArg   = "classHash"
	trueStr        = "true"

	PreConfirmedBlankIdentifier = "0x0"
)

var ErrDeprecatedCompiledClass = errors.New("deprecated compiled class")

// ErrPreConfirmedBlockNotFound is returned by the pre-confirmed queries when
// the gateway answers 400, meaning the requested block is not (or no longer)
// in the pre-confirmed window.
var ErrPreConfirmedBlockNotFound = errors.New("pre-confirmed block not found")

// StatusError reports a non-OK HTTP status from the feeder gateway.
type StatusError struct {
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%d %s", e.Code, http.StatusText(e.Code))
}

type Backoff func(wait time.Duration) time.Duration

type Client struct {
	url        *url.URL
	client     *http.Client
	backoff    Backoff
	maxRetries int
	maxWait    time.Duration
	minWait    time.Duration
	logger     log.StructuredLogger
	userAgent  string
	apiKey     string
	listener   EventListener
	timeouts   atomic.Pointer[Timeouts]
}

//go:generate mockgen -destination=../../mocks/mock_feeder.go -mock_names Reader=MockFeederReader -package=mocks github.com/NethermindEth/juno/clients/feeder Reader
//nolint:staticcheck // Transaction() returns the deprecated DeprecatedTransactionStatus type.
type Reader interface {
	Block(ctx context.Context, blockID string) (starknet.Block, error)
	BlockHeader(ctx context.Context, blockID string) (starknet.BlockHeader, error)
	BlockTrace(ctx context.Context, blockHash string) (starknet.BlockTrace, error)
	CasmClassDefinition(ctx context.Context, classHash *felt.Felt) (starknet.CasmClass, error)
	ClassDefinition(ctx context.Context, classHash *felt.Felt) (starknet.ClassDefinition, error)
	FeeTokenAddresses(ctx context.Context) (starknet.FeeTokenAddresses, error)
	PreConfirmedBlockWithIdentifier(
		ctx context.Context,
		blockNumber string,
		blockIdentifier string,
		knownTransactionCount uint64,
	) (starknet.PreConfirmedUpdate, error)
	PreConfirmedBlockLatest(
		ctx context.Context,
		blockIdentifier string,
		knownTransactionCount uint64,
	) (starknet.PreConfirmedUpdate, uint64, error)
	PublicKey(ctx context.Context) (felt.Felt, error)
	Signature(ctx context.Context, blockID string) (starknet.Signature, error)
	StateUpdate(ctx context.Context, blockID string) (starknet.StateUpdate, error)
	StateUpdateWithBlockAndSignature(
		ctx context.Context,
		blockID string,
	) (starknet.StateUpdateWithBlockAndSignature, error)
	// Deprecated: Use TransactionStatus() instead.
	Transaction(
		ctx context.Context,
		transactionHash *felt.Felt,
	) (starknet.DeprecatedTransactionStatus, error)
	TransactionStatus(
		ctx context.Context,
		transactionHash *felt.Felt,
	) (starknet.TransactionStatus, error)
}

var _ Reader = (*Client)(nil)

func ExponentialBackoff(wait time.Duration) time.Duration {
	return wait * 2
}

func NopBackoff(d time.Duration) time.Duration {
	return 0
}

func NewClient(clientURL *url.URL, opts ...Option) *Client {
	defaultTimeouts := getDefaultFixedTimeouts()
	o := options{
		httpClient: http.DefaultClient,
		backoff:    ExponentialBackoff,
		maxRetries: 10, // ~20s with default backoff and maxWait (block time on mainnet is 2s on average)
		maxWait:    2 * time.Second,
		minWait:    500 * time.Millisecond,
		logger:     log.NewNopZapLogger(),
		listener:   &SelectiveListener{},
		timeouts:   &defaultTimeouts,
	}
	for _, opt := range opts {
		opt(&o)
	}

	client := &Client{
		url:        clientURL,
		client:     o.httpClient,
		backoff:    o.backoff,
		maxRetries: o.maxRetries,
		maxWait:    o.maxWait,
		minWait:    o.minWait,
		logger:     o.logger,
		userAgent:  o.userAgent,
		apiKey:     o.apiKey,
		listener:   o.listener,
	}
	client.timeouts.Store(o.timeouts)
	return client
}

// SetTimeouts atomically replaces the timeouts of a live client.
// It is used by the PUT /feeder/timeouts endpoint.
func (c *Client) SetTimeouts(timeouts []time.Duration, fixed bool) {
	c.timeouts.Store(makeTimeouts(timeouts, fixed))
}

// get performs a "GET" http request with the given URL and returns the response body
func (c *Client) get(
	ctx context.Context,
	queryURL *url.URL,
	opts ...requestOption,
) (io.ReadCloser, error) {
	var cfg requestConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var res *http.Response
	var err error
	wait := time.Duration(0)
	for range c.maxRetries + 1 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
			var req *http.Request
			req, err = http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), http.NoBody)
			if err != nil {
				return nil, err
			}
			if c.userAgent != "" {
				req.Header.Set("User-Agent", c.userAgent)
			}
			if c.apiKey != "" {
				req.Header.Set("X-Throttling-Bypass", c.apiKey)
			}

			timeouts := c.timeouts.Load()
			c.client.Timeout = timeouts.GetCurrentTimeout()
			reqTimer := time.Now()
			res, err = c.client.Do(req)
			tooManyRequests, badRequest := false, false
			if err == nil {
				c.listener.OnResponse(req.URL.Path, res.StatusCode, time.Since(reqTimer))
				tooManyRequests = res.StatusCode == http.StatusTooManyRequests
				badRequest = res.StatusCode == http.StatusBadRequest
				if res.StatusCode == http.StatusOK {
					timeouts.DecreaseTimeout()
					return res.Body, nil
				} else {
					err = &StatusError{Code: res.StatusCode}
				}

				res.Body.Close()
			}

			if cfg.failFastOnBadRequest && badRequest {
				return nil, err
			}

			if !tooManyRequests && !badRequest {
				timeouts.IncreaseTimeout()
			}

			if wait < c.minWait {
				wait = c.minWait
			} else {
				wait = min(c.backoff(wait), c.maxWait)
			}

			currentTimeout := timeouts.GetCurrentTimeout()
			if currentTimeout >= mediumGrowThreshold {
				c.logger.Warn("Failed query to feeder, retrying...",
					zap.String("req", log.SanitizeString(req.URL.String())),
					zap.String("retryAfter", wait.String()),
					zap.Error(err),
					zap.String("newHTTPTimeout", currentTimeout.String()),
				)
				c.logger.Warn("Timeouts can be updated via HTTP PUT request",
					zap.String("timeout", currentTimeout.String()),
					zap.String("hint",
						`Set --http-update-port and --http-update-host flags and `+
							`make a PUT request to "/feeder/timeouts" with the specified timeouts`,
					),
				)
			} else {
				c.logger.Debug("Failed query to feeder, retrying...",
					zap.String("req", log.SanitizeString(req.URL.String())),
					zap.String("retryAfter", wait.String()),
					zap.Error(err),
					zap.String("newHTTPTimeout", currentTimeout.String()),
				)
			}
		}
	}
	return nil, err
}

func (c *Client) Block(ctx context.Context, blockID string) (starknet.Block, error) {
	queryURL := buildQueryString(c.url, "get_block", map[string]string{
		blockNumberArg: blockID,
	})

	return c.doRequest[starknet.Block](ctx, queryURL)
}

func (c *Client) BlockHeader(
	ctx context.Context, blockID string,
) (starknet.BlockHeader, error) {
	queryURL := buildQueryString(c.url, "get_block", map[string]string{
		blockNumberArg: blockID,
		"headerOnly":   trueStr,
	})

	return c.doRequest[starknet.BlockHeader](ctx, queryURL)
}

func (c *Client) BlockTrace(ctx context.Context, blockHash string) (starknet.BlockTrace, error) {
	queryURL := buildQueryString(c.url, "get_block_traces", map[string]string{
		"blockHash": blockHash,
	})

	return c.doRequest[starknet.BlockTrace](ctx, queryURL)
}

func (c *Client) CasmClassDefinition(
	ctx context.Context,
	classHash *felt.Felt,
) (starknet.CasmClass, error) {
	queryURL := buildQueryString(c.url, "get_compiled_class_by_class_hash", map[string]string{
		classHashArg:   classHash.String(),
		blockNumberArg: "latest",
	})

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return starknet.CasmClass{}, err
	}
	defer body.Close()

	definition, err := io.ReadAll(body)
	if err != nil {
		return starknet.CasmClass{}, err
	}

	if deprecated, _ := starknet.IsDeprecatedCompiledClassDefinition(definition); deprecated {
		return starknet.CasmClass{}, ErrDeprecatedCompiledClass
	}

	var class starknet.CasmClass
	if err = json.Unmarshal(definition, &class); err != nil {
		return starknet.CasmClass{}, err
	}
	return class, nil
}

func (c *Client) ClassDefinition(
	ctx context.Context, classHash *felt.Felt,
) (starknet.ClassDefinition, error) {
	queryURL := buildQueryString(c.url, "get_class_by_hash", map[string]string{
		classHashArg:   classHash.String(),
		blockNumberArg: "latest",
	})

	return c.doRequest[starknet.ClassDefinition](ctx, queryURL)
}

func (c *Client) FeeTokenAddresses(ctx context.Context) (starknet.FeeTokenAddresses, error) {
	queryURL := buildQueryString(c.url, "get_contract_addresses", nil)

	return c.doRequest[starknet.FeeTokenAddresses](ctx, queryURL)
}

func (c *Client) PublicKey(ctx context.Context) (felt.Felt, error) {
	queryURL := buildQueryString(c.url, "get_public_key", nil)

	// public key is a hex string
	publicKey, err := c.doRequest[starknet.PublicKey](ctx, queryURL)
	if err != nil {
		return felt.Felt{}, err
	}
	return felt.FromString[felt.Felt](string(publicKey))
}

func (c *Client) Signature(ctx context.Context, blockID string) (starknet.Signature, error) {
	queryURL := buildQueryString(c.url, "get_signature", map[string]string{
		blockNumberArg: blockID,
	})

	return c.doRequest[starknet.Signature](ctx, queryURL)
}

func (c *Client) StateUpdate(ctx context.Context, blockID string) (starknet.StateUpdate, error) {
	queryURL := buildQueryString(c.url, "get_state_update", map[string]string{
		blockNumberArg: blockID,
	})

	return c.doRequest[starknet.StateUpdate](ctx, queryURL)
}

func (c *Client) StateUpdateWithBlockAndSignature(
	ctx context.Context,
	blockID string,
) (starknet.StateUpdateWithBlockAndSignature, error) {
	queryURL := buildQueryString(c.url, "get_state_update", map[string]string{
		blockNumberArg:     blockID,
		"includeBlock":     trueStr,
		"includeSignature": trueStr,
	})

	return c.doRequest[starknet.StateUpdateWithBlockAndSignature](ctx, queryURL)
}

// PreConfirmedBlockWithIdentifier fetches the pre_confirmed block at the given height,
// using the given block identifier and known transaction count to tell the server what
// the caller already has.
//
// blockIdentifier and knownTransactionCount enable delta sync: the server
// uses them to decide whether to return a no-change marker, only the
// transactions appended since knownTransactionCount, or the full block when
// the round identifier no longer matches. Set both to zero values to get a full block.
func (c *Client) PreConfirmedBlockWithIdentifier(
	ctx context.Context,
	blockNumber string,
	blockIdentifier string,
	knownTransactionCount uint64,
) (starknet.PreConfirmedUpdate, error) {
	preConfirmedEnvelope, err := c.fetchPreConfirmedUpdate(
		ctx,
		blockNumber,
		blockIdentifier,
		knownTransactionCount,
	)
	if err != nil {
		return nil, err
	}
	return preConfirmedEnvelope.Update, nil
}

// PreConfirmedBlockLatest fetches the highest pre_confirmed block the server
// currently exposes. The response carries its block_number so the caller can
// discover the pre_confirmed tip without tracking  the height itself.
// Pass an empty identifier and zero txCount for a full reply.
func (c *Client) PreConfirmedBlockLatest(
	ctx context.Context,
	blockIdentifier string,
	knownTransactionCount uint64,
) (starknet.PreConfirmedUpdate, uint64, error) {
	preConfirmedEnvelope, err := c.fetchPreConfirmedUpdate(
		ctx,
		"latest",
		blockIdentifier,
		knownTransactionCount,
	)
	if err != nil {
		return nil, 0, err
	}
	// A full block on the latest query must carry its block_number; absent (or genesis 0)
	// is invalid since the caller relies on it to discover the pre_confirmed tip.
	if _, ok := preConfirmedEnvelope.Update.(starknet.PreConfirmedBlock); ok &&
		preConfirmedEnvelope.BlockNumber == 0 {
		return nil, 0, errors.New(
			"pre_confirmed latest: full block response is missing block_number",
		)
	}
	return preConfirmedEnvelope.Update, preConfirmedEnvelope.BlockNumber, nil
}

func (c *Client) fetchPreConfirmedUpdate(
	ctx context.Context,
	blockNumber string,
	blockIdentifier string,
	knownTransactionCount uint64,
) (*starknet.PreConfirmedUpdateEnvelope, error) {
	if blockIdentifier == "" {
		blockIdentifier = PreConfirmedBlankIdentifier
	}
	queryURL := buildQueryString(c.url, "get_preconfirmed_block", map[string]string{
		blockNumberArg:          blockNumber,
		"blockIdentifier":       blockIdentifier,
		"knownTransactionCount": strconv.FormatUint(knownTransactionCount, 10),
	})

	// PreConfirmedUpdateEnvelope intentionally has no UnmarshalJSON (see its doc),
	// so it cannot ride the generic doRequest. Decode in a single scan, then run
	// the same Validate + error-wrap that doRequest applies.
	//
	// The gateway answers 400 (never 404) when the queried block is not in the
	// pre-confirmed window. That is deterministic, so the request fails fast
	// instead of burning the retry budget, and the 400 surfaces as
	// ErrPreConfirmedBlockNotFound for callers to match on.
	body, err := c.get(ctx, queryURL, failFastOnBadRequest())
	if err != nil {
		var statusErr *StatusError
		if errors.As(err, &statusErr) && statusErr.Code == http.StatusBadRequest {
			return nil, fmt.Errorf("%w: %w", ErrPreConfirmedBlockNotFound, err)
		}
		return nil, err
	}
	defer body.Close()

	env, err := starknet.DecodePreConfirmedUpdate(body)
	if err != nil {
		return nil, err
	}
	if err := env.Validate(); err != nil {
		return nil, errors.Join(
			ErrInvalidFeederResponse,
			fmt.Errorf("querying %s: %w", queryURL, err),
		)
	}
	return &env, nil
}

// Deprecated: Transaction calls the get_transaction endpoint which returns
// the full transaction body. Use TransactionStatus() instead.
func (c *Client) Transaction(
	ctx context.Context, transactionHash *felt.Felt,
) (starknet.DeprecatedTransactionStatus, error) {
	queryURL := buildQueryString(c.url, "get_transaction", map[string]string{
		"transactionHash": transactionHash.String(),
	})

	return c.doRequest[starknet.DeprecatedTransactionStatus](ctx, queryURL)
}

// TransactionStatus calls the get_transaction_status endpoint which returns only status fields
// (finality, execution status, block hash) without the full transaction body.
func (c *Client) TransactionStatus(
	ctx context.Context,
	transactionHash *felt.Felt,
) (starknet.TransactionStatus, error) {
	queryURL := buildQueryString(c.url, "get_transaction_status", map[string]string{
		"transactionHash": transactionHash.String(),
	})

	return c.doRequest[starknet.TransactionStatus](ctx, queryURL)
}
