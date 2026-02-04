package feeder

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

var ErrDeprecatedCompiledClass = errors.New("deprecated compiled class")

type Backoff func(wait time.Duration) time.Duration

type Client struct {
	url        string
	client     *http.Client
	backoff    Backoff
	maxRetries int
	maxWait    time.Duration
	minWait    time.Duration
	log        utils.StructuredLogger
	userAgent  string
	apiKey     string
	listener   EventListener
	timeouts   atomic.Pointer[Timeouts]
}

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

func (c *Client) WithLogger(log utils.StructuredLogger) *Client {
	c.log = log
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
	client := &Client{
		url:        clientURL,
		client:     http.DefaultClient,
		backoff:    ExponentialBackoff,
		maxRetries: 10, // ~20s with default backoff and maxWait (block time on mainnet is 2s on average)
		maxWait:    2 * time.Second,
		minWait:    500 * time.Millisecond,
		log:        utils.NewNopZapLogger(),
		listener:   &SelectiveListener{},
	}
	client.timeouts.Store(&defaultTimeouts)
	return client
}

// get performs a "GET" http request with the given URL and returns the response body
func (c *Client) get(ctx context.Context, queryURL string) (io.ReadCloser, error) {
	var res *http.Response
	var err error
	wait := time.Duration(0)
	for range c.maxRetries + 1 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
			var req *http.Request
			req, err = http.NewRequestWithContext(ctx, http.MethodGet, queryURL, http.NoBody)
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
				c.log.Warn("Failed query to feeder, retrying...",
					zap.String("req", req.URL.String()),
					zap.String("retryAfter", wait.String()),
					zap.Error(err),
					zap.String("newHTTPTimeout", currentTimeout.String()),
				)
				c.log.Warn("Timeouts can be updated via HTTP PUT request",
					zap.String("timeout", currentTimeout.String()),
					zap.String("hint", `Set --http-update-port and --http-update-host flags and make a PUT request to "/feeder/timeouts" with the specified timeouts`),
				)
			} else {
				c.log.Debug("Failed query to feeder, retrying...",
					zap.String("req", req.URL.String()),
					zap.String("retryAfter", wait.String()),
					zap.Error(err),
					zap.String("newHTTPTimeout", currentTimeout.String()),
				)
			}
		}
	}
	return nil, err
}

func (c *Client) StateUpdate(ctx context.Context, blockID string) (*starknet.StateUpdate, error) {
	queryURL := c.buildQueryString("get_state_update", map[string]string{
		"blockNumber": blockID,
	})

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	update := new(starknet.StateUpdate)
	if err = json.NewDecoder(body).Decode(update); err != nil {
		return nil, err
	}
	return update, nil
}

func (c *Client) Transaction(ctx context.Context, transactionHash *felt.Felt) (*starknet.TransactionStatus, error) {
	queryURL := c.buildQueryString("get_transaction", map[string]string{
		"transactionHash": transactionHash.String(),
	})

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	txStatus := new(starknet.TransactionStatus)
	if err = json.NewDecoder(body).Decode(txStatus); err != nil {
		return nil, err
	}
	return txStatus, nil
}

func (c *Client) Block(ctx context.Context, blockID string) (*starknet.Block, error) {
	queryURL := c.buildQueryString("get_block", map[string]string{
		"blockNumber": blockID,
	})

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	block := new(starknet.Block)
	if err = json.NewDecoder(body).Decode(block); err != nil {
		return nil, err
	}
	return block, nil
}

func (c *Client) ClassDefinition(ctx context.Context, classHash *felt.Felt) (*starknet.ClassDefinition, error) {
	queryURL := c.buildQueryString("get_class_by_hash", map[string]string{
		"classHash":   classHash.String(),
		"blockNumber": "pending",
	})

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	class := new(starknet.ClassDefinition)
	if err = json.NewDecoder(body).Decode(class); err != nil {
		return nil, err
	}
	return class, nil
}

func (c *Client) CasmClassDefinition(
	ctx context.Context,
	classHash *felt.Felt,
) (*starknet.CasmClass, error) {
	queryURL := c.buildQueryString("get_compiled_class_by_class_hash", map[string]string{
		"classHash":   classHash.String(),
		"blockNumber": "pending",
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

func (c *Client) PublicKey(ctx context.Context) (*felt.Felt, error) {
	queryURL := c.buildQueryString("get_public_key", nil)

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var publicKey string // public key hex string
	if err = json.NewDecoder(body).Decode(&publicKey); err != nil {
		return nil, err
	}

	return new(felt.Felt).SetString(publicKey)
}

func (c *Client) Signature(ctx context.Context, blockID string) (*starknet.Signature, error) {
	queryURL := c.buildQueryString("get_signature", map[string]string{
		"blockNumber": blockID,
	})

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	signature := new(starknet.Signature)
	if err := json.NewDecoder(body).Decode(signature); err != nil {
		return nil, err
	}

	return signature, nil
}

func (c *Client) StateUpdateWithBlock(ctx context.Context, blockID string) (*starknet.StateUpdateWithBlock, error) {
	queryURL := c.buildQueryString("get_state_update", map[string]string{
		"blockNumber":  blockID,
		"includeBlock": "true",
	})

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	stateUpdate := new(starknet.StateUpdateWithBlock)
	if err := json.NewDecoder(body).Decode(stateUpdate); err != nil {
		return nil, err
	}

	return stateUpdate, nil
}

func (c *Client) BlockTrace(ctx context.Context, blockHash string) (*starknet.BlockTrace, error) {
	queryURL := c.buildQueryString("get_block_traces", map[string]string{
		"blockHash": blockHash,
	})

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	traces := new(starknet.BlockTrace)
	if err = json.NewDecoder(body).Decode(traces); err != nil {
		return nil, err
	}
	return traces, nil
}

func (c *Client) PreConfirmedBlock(ctx context.Context, blockNumber string) (*starknet.PreConfirmedBlock, error) {
	queryURL := c.buildQueryString("get_preconfirmed_block", map[string]string{
		"blockNumber": blockNumber,
	})

	body, err := c.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	preConfirmedBlock := new(starknet.PreConfirmedBlock)
	if err = json.NewDecoder(body).Decode(preConfirmedBlock); err != nil {
		return nil, err
	}
	return preConfirmedBlock, nil
}

func (c *Client) FeeTokenAddresses(ctx context.Context) (starknet.FeeTokenAddresses, error) {
	queryURL := c.buildQueryString("get_contract_addresses", nil)
	body, err := c.get(ctx, queryURL)
	if err != nil {
		return starknet.FeeTokenAddresses{}, err
	}
	defer body.Close()

	contractAddresses := new(starknet.FeeTokenAddresses)
	if err = json.NewDecoder(body).Decode(contractAddresses); err != nil {
		return starknet.FeeTokenAddresses{}, err
	}
	return *contractAddresses, nil
}
