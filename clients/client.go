package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NethermindEth/juno/clients/sequencertypes"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type Backoff func(wait time.Duration) time.Duration

type Client struct {
	url        string
	client     *http.Client
	backoff    Backoff
	maxRetries int
	maxWait    time.Duration
	minWait    time.Duration
	log        utils.SimpleLogger
	userAgent  string
	timeout    time.Duration
}

func NewClient(urlStr string) *Client {
	urlStr = strings.TrimSuffix(urlStr, "/")
	return &Client{
		url:        urlStr,
		client:     http.DefaultClient,
		backoff:    ExponentialBackoff,
		maxRetries: 35, // ~3.5 minutes with default backoff and maxWait (block time on mainnet is 1-2 minutes)
		maxWait:    10 * time.Second,
		minWait:    time.Second,
		log:        utils.NewNopZapLogger(),
		timeout:    10 * time.Second,
	}
}

func ExponentialBackoff(wait time.Duration) time.Duration {
	return wait * 2
}

func NopBackoff(d time.Duration) time.Duration {
	return 0
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

func (c *Client) WithLogger(log utils.SimpleLogger) *Client {
	c.log = log
	return c
}

func (c *Client) WithUserAgent(ua string) *Client {
	c.userAgent = ua
	return c
}

// BuildQueryString builds the query url with encoded parameters
func (c *Client) buildQueryString(endpoint string, args map[string]string) string {
	base, err := url.Parse(c.url)
	if err != nil {
		panic("Malformed feeder base URL")
	}

	base.Path += endpoint

	params := url.Values{}
	for k, v := range args {
		params.Add(k, v)
	}
	base.RawQuery = params.Encode()

	return base.String()
}

// NewTestClient returns a client and a function to close a test server.
func NewTestClient(t *testing.T, network utils.Network) *Client {
	srv := newTestServer(network)
	t.Cleanup(srv.Close)
	ua := "Juno/v0.0.1-test Starknet Implementation"

	c := NewClient(srv.URL).WithBackoff(NopBackoff).WithMaxRetries(0).WithUserAgent(ua)
	c.client = &http.Client{
		Transport: &http.Transport{
			// On macOS tests often fail with the following error:
			//
			// "Get "http://127.0.0.1:xxxx/get_{feeder gateway method}?{arg}={value}": dial tcp 127.0.0.1:xxxx:
			//    connect: can't assign requested address"
			//
			// This error makes running local tests, in quick succession, difficult because we have to wait for the OS to release ports.
			// Sometimes the sync tests will hang because sync process will keep making requests if there was some error.
			// This problem is further exacerbated by having parallel tests.
			//
			// Increasing test client's idle conns allows for large concurrent requests to be made from a single test client.
			MaxIdleConnsPerHost: 1000,
		},
	}
	return c
}

func newTestServer(network utils.Network) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.Contains(r.URL.Path, "add_transaction") {
		case true:
			testServerGateway(w, r)
		default:
			testServerFeeder(network, w, r)
		}
	}))
}

func testServerFeeder(network utils.Network, w http.ResponseWriter, r *http.Request) {
	queryMap, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	base := wd[:strings.LastIndex(wd, "juno")+4]
	queryArg := ""
	dir := ""
	const blockNumberArg = "blockNumber"
	switch {
	case strings.HasSuffix(r.URL.Path, "get_block"):
		dir = "block"
		queryArg = blockNumberArg
	case strings.HasSuffix(r.URL.Path, "get_state_update"):
		queryArg = blockNumberArg
		if includeBlock, ok := queryMap["includeBlock"]; !ok || len(includeBlock) == 0 {
			dir = "state_update"
		} else {
			dir = "state_update_with_block"
		}
	case strings.HasSuffix(r.URL.Path, "get_transaction"):
		dir = "transaction"
		queryArg = "transactionHash"
	case strings.HasSuffix(r.URL.Path, "get_class_by_hash"):
		dir = "class"
		queryArg = "classHash"
	case strings.HasSuffix(r.URL.Path, "get_compiled_class_by_class_hash"):
		dir = "compiled_class"
		queryArg = "classHash"
	case strings.HasSuffix(r.URL.Path, "get_public_key"):
		dir = "public_key"
		queryArg = "pk"
		queryMap[queryArg] = []string{queryArg}
	case strings.HasSuffix(r.URL.Path, "get_signature"):
		dir = "signature"
		queryArg = blockNumberArg
	}

	fileName, found := queryMap[queryArg]
	if !found {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	path := filepath.Join(base, "clients", "testdata", network.String(), dir, fileName[0]+".json")
	read, err := os.ReadFile(path)
	if err != nil {
		handleNotFound(dir, queryArg, w)
		return
	}
	w.Write(read) //nolint:errcheck
}

func testServerGateway(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error())) //nolint:errcheck
		return
	}

	// empty request: "{}"
	emptyReqLen := 4
	if string(b) == "null" {
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if len(b) <= emptyReqLen {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"code": "Malformed Request", "message": "empty request"}`)) //nolint:errcheck
		return
	}

	hash := new(felt.Felt).SetBytes([]byte("random"))
	resp := fmt.Sprintf("{\"code\": \"TRANSACTION_RECEIVED\", \"transaction_hash\": %q, \"address\": %q}", hash.String(), hash.String())
	w.Write([]byte(resp)) //nolint:errcheck
}

func handleNotFound(dir, queryArg string, w http.ResponseWriter) {
	// If a transaction data is missing, respond with
	// {"finality_status": "NOT_RECEIVED", "status": "NOT_RECEIVED"}
	// instead of 404 as per real test server behaviour.
	if dir == "transaction" && queryArg == "transactionHash" {
		w.Write([]byte("{\"finality_status\": \"NOT_RECEIVED\", \"status\": \"NOT_RECEIVED\"}")) //nolint:errcheck
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// get performs a "GET" http request with the given URL and returns the response body
func (c *Client) get(ctx context.Context, queryURL string) (io.ReadCloser, error) {
	var res *http.Response
	var err error
	wait := time.Duration(0)
	for i := 0; i <= c.maxRetries; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
			var req *http.Request
			req, err = http.NewRequestWithContext(ctx, "GET", queryURL, http.NoBody)
			if err != nil {
				return nil, err
			}
			if c.userAgent != "" {
				req.Header.Set("User-Agent", c.userAgent)
			}

			res, err = c.client.Do(req)
			if err == nil {
				if res.StatusCode == http.StatusOK {
					return res.Body, nil
				} else {
					err = errors.New(res.Status)
				}

				res.Body.Close()
			}

			if wait < c.minWait {
				wait = c.minWait
			}
			wait = c.backoff(wait)
			if wait > c.maxWait {
				wait = c.maxWait
			}
			c.log.Warnw("failed query to feeder, retrying...", "retryAfter", wait.String())
		}
	}
	return nil, err
}

// post performs additional utility function over doPost method
func (c *Client) post(urlStr string, data any) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resp, err := c.doPost(ctx, urlStr, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var gatewayError sequencertypes.Error
		body, readErr := io.ReadAll(resp.Body)
		if readErr == nil && len(body) > 0 {
			if err := json.Unmarshal(body, &gatewayError); err == nil {
				if len(gatewayError.Code) != 0 {
					return nil, &gatewayError
				}
			}
			return nil, errors.New(string(body))
		}
		return nil, errors.New(resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// doPost performs a "POST" http request with the given URL and a JSON payload derived from the provided data
// it returns response without additional error handling
func (c *Client) doPost(ctx context.Context, urlStr string, data any) (*http.Response, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return c.client.Do(req)
}
