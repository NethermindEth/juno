package feeder

import (
	"context"
	"encoding/json"
	"errors"
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
//nolint:staticcheck // We need to mention the deprecated type in the interface
type Reader interface {
	Block(ctx context.Context, blockID string) (*starknet.Block, error)
	BlockHeader(ctx context.Context, blockID string) (starknet.BlockHeader, error)
	BlockTrace(ctx context.Context, blockHash string) (*starknet.BlockTrace, error)
	CasmClassDefinition(ctx context.Context, classHash *felt.Felt) (*starknet.CasmClass, error)
	ClassDefinition(ctx context.Context, classHash *felt.Felt) (*starknet.ClassDefinition, error)
	FeeTokenAddresses(ctx context.Context) (starknet.FeeTokenAddresses, error)
	DeprecatedPreConfirmedBlock(
		ctx context.Context,
		blockNumber string,
	) (*starknet.DeprecatedPreConfirmedBlock, error)
	PreConfirmedBlockWithIdentifier(
		ctx context.Context,
		blockNumber string,
		blockIdentifier string,
		knownTransactionCount uint64,
	) (starknet.PreConfirmedUpdate, error)
	PublicKey(ctx context.Context) (*felt.Felt, error)
	Signature(ctx context.Context, blockID string) (*starknet.Signature, error)
	StateUpdate(ctx context.Context, blockID string) (*starknet.StateUpdate, error)
	StateUpdateWithBlock(ctx context.Context, blockID string) (*starknet.StateUpdateWithBlock, error)
	StateUpdateWithBlockAndSignature(
		ctx context.Context,
		blockID string,
	) (*starknet.StateUpdateWithBlockAndSignature, error)
	// Deprecated: Use TransactionStatus() instead.
	Transaction(
		ctx context.Context,
		transactionHash *felt.Felt,
	) (*starknet.DeprecatedTransactionStatus, error)
	TransactionStatus(
		ctx context.Context,
		transactionHash *felt.Felt,
	) (*starknet.TransactionStatus, error)
}

var _ Reader = (*Client)(nil)

func (c *Client) WithListener(l EventListener) *Client {
	c.listener = l
	return c
}

func (c *Client) WithBackoff(b Backoff) *Client {
	c.backoff = b
	return c
}

func (c *Client) WithMaxRetries(num int) *Client {
	c.maxRetries = num
	return c
}

func (c *Client) WithMaxWait(d time.Duration) *Client {
	c.maxWait = d
	return c
}

func (c *Client) WithMinWait(d time.Duration) *Client {
	c.minWait = d
	return c
}

func (c *Client) WithLogger(logger log.StructuredLogger) *Client {
	c.logger = logger
	return c
}

func (c *Client) WithUserAgent(ua string) *Client {
	c.userAgent = ua
	return c
}

func (c *Client) WithTimeouts(timeouts []time.Duration, fixed bool) *Client {
	var newTimeouts *Timeouts
	if len(timeouts) > 1 || fixed {
		t := getFixedTimeouts(timeouts)
		newTimeouts = &t
	} else {
		t := getDynamicTimeouts(timeouts[0])
		newTimeouts = &t
	}
	c.timeouts.Store(newTimeouts)
	return c
}

func (c *Client) WithAPIKey(key string) *Client {
	c.apiKey = key
	return c
}

func ExponentialBackoff(wait time.Duration) time.Duration {
	return wait * 2
}

func NopBackoff(d time.Duration) time.Duration {
	return 0
}

func NewClient(clientURL string) *Client {
	defaultTimeouts := getDefaultFixedTimeouts()
	base, err := url.Parse(clientURL)
	if err != nil {
		panic("malformed feeder base URL")
	}

	client := &Client{
		url:        base,
		client:     http.DefaultClient,
		backoff:    ExponentialBackoff,
		maxRetries: 10, // ~20s with default backoff and maxWait (block time on mainnet is 2s on average)
		maxWait:    2 * time.Second,
		minWait:    500 * time.Millisecond,
		logger:     log.NewNopZapLogger(),
		listener:   &SelectiveListener{},
	}
	client.timeouts.Store(&defaultTimeouts)
	return client
}

// get performs a "GET" http request with the given URL and returns the response body
func (c *Client) get(ctx context.Context, queryURL *url.URL) (io.ReadCloser, error) {
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
					err = errors.New(res.Status)
				}

				res.Body.Close()
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

func (c *Client) Block(ctx context.Context, blockID string) (*starknet.Block, error) {
	queryURL := buildQueryString(c.url, "get_block", map[string]string{
		blockNumberArg: blockID,
	})

	return doRequest[starknet.Block](ctx, c, queryURL)
}

func (c *Client) BlockHeader(
	ctx context.Context, blockID string,
) (starknet.BlockHeader, error) {
	queryURL := buildQueryString(c.url, "get_block", map[string]string{
		blockNumberArg: blockID,
		"headerOnly":   trueStr,
	})

	header, err := doRequest[starknet.BlockHeader](ctx, c, queryURL)
	if err != nil {
		return starknet.BlockHeader{}, err
	}
	return *header, err
}

func (c *Client) BlockTrace(ctx context.Context, blockHash string) (*starknet.BlockTrace, error) {
	queryURL := buildQueryString(c.url, "get_block_traces", map[string]string{
		"blockHash": blockHash,
	})

	return doRequest[starknet.BlockTrace](ctx, c, queryURL)
}

func (c *Client) CasmClassDefinition(
	ctx context.Context,
	classHash *felt.Felt,
) (*starknet.CasmClass, error) {
	queryURL := buildQueryString(c.url, "get_compiled_class_by_class_hash", map[string]string{
		classHashArg:   classHash.String(),
		blockNumberArg: "latest",
	})

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	definition, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	if deprecated, _ := starknet.IsDeprecatedCompiledClassDefinition(definition); deprecated {
		return nil, ErrDeprecatedCompiledClass
	}

	class := new(starknet.CasmClass)
	if err = json.Unmarshal(definition, class); err != nil {
		return nil, err
	}
	return class, nil
}

func (c *Client) ClassDefinition(
	ctx context.Context, classHash *felt.Felt,
) (*starknet.ClassDefinition, error) {
	queryURL := buildQueryString(c.url, "get_class_by_hash", map[string]string{
		classHashArg:   classHash.String(),
		blockNumberArg: "latest",
	})

	return doRequest[starknet.ClassDefinition](ctx, c, queryURL)
}

func (c *Client) FeeTokenAddresses(ctx context.Context) (starknet.FeeTokenAddresses, error) {
	queryURL := buildQueryString(c.url, "get_contract_addresses", nil)

	addresses, err := doRequest[starknet.FeeTokenAddresses](ctx, c, queryURL)
	if err != nil {
		return starknet.FeeTokenAddresses{}, err
	}
	return *addresses, err
}

func (c *Client) PublicKey(ctx context.Context) (*felt.Felt, error) {
	queryURL := buildQueryString(c.url, "get_public_key", nil)

	// public key is a hex string
	publicKey, err := doRequest[starknet.PublicKey](ctx, c, queryURL)
	if err != nil {
		return nil, err
	}
	return felt.NewFromString[felt.Felt](string(*publicKey))
}

func (c *Client) Signature(ctx context.Context, blockID string) (*starknet.Signature, error) {
	queryURL := buildQueryString(c.url, "get_signature", map[string]string{
		blockNumberArg: blockID,
	})

	return doRequest[starknet.Signature](ctx, c, queryURL)
}

func (c *Client) StateUpdate(ctx context.Context, blockID string) (*starknet.StateUpdate, error) {
	queryURL := buildQueryString(c.url, "get_state_update", map[string]string{
		blockNumberArg: blockID,
	})

	return doRequest[starknet.StateUpdate](ctx, c, queryURL)
}

func (c *Client) StateUpdateWithBlock(ctx context.Context, blockID string) (*starknet.StateUpdateWithBlock, error) {
	queryURL := buildQueryString(c.url, "get_state_update", map[string]string{
		blockNumberArg: blockID,
		"includeBlock": trueStr,
	})

	return doRequest[starknet.StateUpdateWithBlock](ctx, c, queryURL)
}

func (c *Client) StateUpdateWithBlockAndSignature(
	ctx context.Context,
	blockID string,
) (*starknet.StateUpdateWithBlockAndSignature, error) {
	queryURL := buildQueryString(c.url, "get_state_update", map[string]string{
		blockNumberArg:     blockID,
		"includeBlock":     trueStr,
		"includeSignature": trueStr,
	})

	return doRequest[starknet.StateUpdateWithBlockAndSignature](ctx, c, queryURL)
}

// DeprecatedPreConfirmedBlock fetches the pre_confirmed block at the given
// height from the legacy "get_preconfirmed_block" endpoint. Prefer
// [Client.PreConfirmedBlockWithIdentifier].
//
//nolint:staticcheck // wraps the deprecated DeprecatedPreConfirmedBlock type.
func (c *Client) DeprecatedPreConfirmedBlock(
	ctx context.Context,
	blockNumber string,
) (*starknet.DeprecatedPreConfirmedBlock, error) {
	queryURL := buildQueryString(c.url, "get_preconfirmed_block", map[string]string{
		blockNumberArg: blockNumber,
	})

	return doRequest[starknet.DeprecatedPreConfirmedBlock](ctx, c, queryURL)
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
	if blockIdentifier == "" {
		blockIdentifier = PreConfirmedBlankIdentifier
	}
	queryURL := buildQueryString(c.url, "get_preconfirmed_block", map[string]string{
		blockNumberArg:          blockNumber,
		"blockIdentifier":       blockIdentifier,
		"knownTransactionCount": strconv.FormatUint(knownTransactionCount, 10),
	})

	env, err := doRequest[starknet.PreConfirmedUpdateEnvelope](ctx, c, queryURL)
	if err != nil {
		return nil, err
	}
	return env.Update, nil
}

// Deprecated: Transaction calls the get_transaction endpoint which returns
// the full transaction body. Use TransactionStatus() instead.
func (c *Client) Transaction(
	ctx context.Context, transactionHash *felt.Felt,
) (*starknet.DeprecatedTransactionStatus, error) {
	queryURL := buildQueryString(c.url, "get_transaction", map[string]string{
		"transactionHash": transactionHash.String(),
	})

	return doRequest[starknet.DeprecatedTransactionStatus](ctx, c, queryURL)
}

// TransactionStatus calls the get_transaction_status endpoint which returns only status fields
// (finality, execution status, block hash) without the full transaction body.
func (c *Client) TransactionStatus(
	ctx context.Context,
	transactionHash *felt.Felt,
) (*starknet.TransactionStatus, error) {
	queryURL := buildQueryString(c.url, "get_transaction_status", map[string]string{
		"transactionHash": transactionHash.String(),
	})

	return doRequest[starknet.TransactionStatus](ctx, c, queryURL)
}
