package starknetrpc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/clients/starknetrpc/testutils"
	"github.com/NethermindEth/juno/clients/timeout"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	junosync "github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gate sits in front of the RPC handler and counts requests. serve decides per
// request whether it reaches the handler; declined requests get a 503.
type gate struct {
	upstream http.Handler
	requests atomic.Int64
	serve    func(request int64, r *http.Request) bool
}

func newGate(upstream http.Handler, serve func(request int64, r *http.Request) bool) *gate {
	return &gate{upstream: upstream, serve: serve}
}

func (g *gate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request := g.requests.Add(1)
	if g.serve != nil && !g.serve(request, r) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	g.upstream.ServeHTTP(w, r)
}

func serveAll(int64, *http.Request) bool { return true }

func serveNone(int64, *http.Request) bool { return false }

func newClient(t *testing.T, handler http.Handler, opts ...Option) *Client {
	t.Helper()
	return newClientWithTimeouts(t, handler, timeout.New([]time.Duration{time.Second}, true), opts...)
}

func newClientWithTimeouts(
	t *testing.T,
	handler http.Handler,
	timeouts *timeout.Timeouts,
	opts ...Option,
) *Client {
	t.Helper()
	serverURL := testutils.Listen(t, handler)
	opts = append([]Option{WithMinWait(time.Millisecond), WithMaxWait(time.Millisecond)}, opts...)
	client, err := New(t.Context(), serverURL, timeouts, log.NewNopZapLogger(), opts...)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func requireNoRPCError(t *testing.T, rpcErr *jsonrpc.Error) {
	t.Helper()
	if rpcErr != nil {
		t.Fatalf("rpc error %d: %s: %v", rpcErr.Code, rpcErr.Message, rpcErr.Data)
	}
}

// runCall checks that client answers m like the handler call want does. want may answer by
// value or by pointer; normalize runs on both sides before they are compared.
func runCall[P params, R, W any](
	t *testing.T,
	client *Client,
	name string,
	m method[P, R],
	p P,
	want func() (W, *jsonrpc.Error),
	normalize ...func(*R),
) {
	t.Run(name, func(t *testing.T) {
		answer, rpcErr := want()
		requireNoRPCError(t, rpcErr)

		got, err := client.Call(t.Context(), m, p)

		require.NoError(t, err)
		expected := decodeInto[R](t, answer)
		for _, fn := range normalize {
			fn(expected)
			fn(got)
		}
		assert.Equal(t, expected, got)
	})
}

// decodeInto round-trips value through JSON into the client's answer type.
func decodeInto[R, W any](t *testing.T, value W) *R {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	var decoded R
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	return &decoded
}

func sortStateDiff(diff *rpcv10.StateDiff) {
	slices.SortFunc(diff.StorageDiffs, func(a, b rpcv10.StorageDiff) int {
		return a.Address.Cmp(&b.Address)
	})
	for _, storageDiff := range diff.StorageDiffs {
		slices.SortFunc(storageDiff.StorageEntries, func(a, b rpcv10.Entry) int {
			return a.Key.Cmp(&b.Key)
		})
	}
	slices.SortFunc(diff.Nonces, func(a, b rpcv10.Nonce) int {
		return a.ContractAddress.Cmp(&b.ContractAddress)
	})
	slices.SortFunc(diff.DeployedContracts, func(a, b rpcv10.DeployedContract) int {
		return a.Address.Cmp(&b.Address)
	})
	slices.SortFunc(diff.DeclaredClasses, func(a, b rpcv10.DeclaredClass) int {
		return a.ClassHash.Cmp(&b.ClassHash)
	})
	slices.SortFunc(diff.ReplacedClasses, func(a, b rpcv10.ReplacedClass) int {
		return a.ContractAddress.Cmp(&b.ContractAddress)
	})
	slices.SortFunc(diff.MigratedCompiledClasses, func(a, b rpcv10.MigratedCompiledClass) int {
		classHashA, classHashB := felt.Felt(a.ClassHash), felt.Felt(b.ClassHash)
		return classHashA.Cmp(&classHashB)
	})
}

func TestNew(t *testing.T) {
	serverURL := testutils.Listen(t, testutils.Serve(t, testutils.NewChain(t)))

	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{name: "http endpoint", url: serverURL.String()},
		{
			name:    "unknown scheme",
			url:     "ftp://127.0.0.1",
			wantErr: `dialing ftp://127.0.0.1: no known transport for URL scheme "ftp"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientURL, err := url.Parse(tt.url)
			require.NoError(t, err)
			timeouts := timeout.New([]time.Duration{time.Second}, true)

			client, err := New(t.Context(), clientURL, timeouts, log.NewNopZapLogger())

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Nil(t, client)
				return
			}
			require.NoError(t, err)
			t.Cleanup(client.Close)
			assert.Equal(t, "rpcsync", client.Name())
			assert.Equal(t, timeouts.String(), client.Timeouts())
			assert.Equal(t, defaultMaxRetries, client.maxRetries)
			assert.Equal(t, defaultMinWait, client.minWait)
			assert.Equal(t, defaultMaxWait, client.maxWait)
		})
	}
}

func TestWithUserAgent(t *testing.T) {
	remote := testutils.Serve(t, testutils.NewChain(t))

	tests := []struct {
		name string
		opts []Option
		want string
	}{
		{name: "default", want: "Go-http-client/1.1"},
		{name: "custom", opts: []Option{WithUserAgent("juno-test/1")}, want: "juno-test/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userAgent string
			handler := newGate(remote, func(_ int64, r *http.Request) bool {
				userAgent = r.UserAgent()
				return true
			})
			client := newClient(t, handler, tt.opts...)

			_, err := client.Call(t.Context(), ChainID, NoParams{})

			require.NoError(t, err)
			assert.Equal(t, tt.want, userAgent)
		})
	}
}

func TestCall(t *testing.T) {
	chain := testutils.NewChain(t)
	handler := rpcv10.New(chain, new(junosync.NoopSynchronizer), nil, log.NewNopZapLogger())
	client := newClient(t, testutils.Serve(t, chain))
	blockNumber := uint64(testutils.ChainHeight - 1)
	blockID := rpcv10.BlockIDFromNumber(blockNumber)
	deprecatedClass := felt.NewUnsafeFromString[felt.Felt](testutils.DeprecatedClassHash)
	sierraClass := felt.NewUnsafeFromString[felt.Felt](testutils.SierraClassHash)

	runCall(t, client, "chain id", ChainID, NoParams{}, handler.ChainID)
	runCall(t, client, "spec version", SpecVersion, NoParams{}, handler.SpecVersion)
	runCall(
		t, client, "block hash and number",
		BlockHashAndNumber, NoParams{}, handler.BlockHashAndNumber,
	)
	runCall(
		t, client, "block with receipts",
		BlockWithReceipts, BlockWithReceiptsParams{BlockNumber: blockNumber},
		func() (*rpcv10.BlockWithReceipts, *jsonrpc.Error) {
			return handler.BlockWithReceipts(&blockID, rpcv10.ResponseFlags{IncludeProofFacts: true})
		},
	)
	runCall(
		t, client, "state update",
		StateUpdate, BlockParams{BlockNumber: blockNumber},
		func() (rpcv10.StateUpdate, *jsonrpc.Error) { return handler.StateUpdate(&blockID, nil) },
		func(update *rpcv10.StateUpdate) { sortStateDiff(update.StateDiff) },
	)
	runCall(
		t, client, "deprecated class",
		Class, ClassParams{BlockNumber: blockNumber, ClassHash: deprecatedClass},
		func() (*rpcv10.Class, *jsonrpc.Error) { return handler.Class(&blockID, deprecatedClass) },
	)
	runCall(
		t, client, "sierra class",
		Class, ClassParams{BlockNumber: blockNumber, ClassHash: sierraClass},
		func() (*rpcv10.Class, *jsonrpc.Error) { return handler.Class(&blockID, sierraClass) },
	)
	runCall(
		t, client, "compiled casm",
		CompiledCasm, ClassHashParams{ClassHash: sierraClass},
		func() (rpcv10.CompiledCasmResponse, *jsonrpc.Error) {
			return handler.CompiledCasm(sierraClass)
		},
	)
}

func TestCallRetries(t *testing.T) {
	remote := testutils.Serve(t, testutils.NewChain(t))

	tests := []struct {
		name         string
		serve        func(cancel context.CancelFunc) func(request int64, r *http.Request) bool
		opts         []Option
		call         func(ctx context.Context, client *Client) error
		wantRequests int64
		wantErr      func(t *testing.T, err error)
	}{
		{
			name:  "block not found is not retried",
			serve: func(context.CancelFunc) func(int64, *http.Request) bool { return serveAll },
			call: func(ctx context.Context, client *Client) error {
				params := BlockWithReceiptsParams{BlockNumber: testutils.MissingBlockNumber}
				_, err := client.Call(ctx, BlockWithReceipts, params)
				return err
			},
			wantRequests: 1,
			wantErr: func(t *testing.T, err error) {
				code, isRPCError := errorCode(err)
				require.True(t, isRPCError)
				assert.Equal(t, rpccore.ErrBlockNotFound.Code, code)
				assert.NotContains(t, err.Error(), "giving up")
			},
		},
		{
			name: "transient failures are retried",
			serve: func(context.CancelFunc) func(int64, *http.Request) bool {
				return func(request int64, _ *http.Request) bool { return request > 2 }
			},
			opts: []Option{WithMaxRetries(2)},
			call: func(ctx context.Context, client *Client) error {
				_, err := client.Call(ctx, ChainID, NoParams{})
				return err
			},
			wantRequests: 3,
			wantErr:      func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:  "gives up after max retries",
			serve: func(context.CancelFunc) func(int64, *http.Request) bool { return serveNone },
			opts:  []Option{WithMaxRetries(2)},
			call: func(ctx context.Context, client *Client) error {
				_, err := client.Call(ctx, ChainID, NoParams{})
				return err
			},
			wantRequests: 3,
			wantErr: func(t *testing.T, err error) {
				var httpErr gethrpc.HTTPError
				require.ErrorAs(t, err, &httpErr)
				assert.Equal(t, http.StatusServiceUnavailable, httpErr.StatusCode)
				assert.Contains(t, err.Error(), "giving up after 2 retries")
			},
		},
		{
			name: "cancelled context stops the backoff",
			serve: func(cancel context.CancelFunc) func(int64, *http.Request) bool {
				return func(int64, *http.Request) bool {
					cancel()
					return false
				}
			},
			opts: []Option{WithMinWait(time.Hour), WithMaxWait(time.Hour)},
			call: func(ctx context.Context, client *Client) error {
				_, err := client.Call(ctx, ChainID, NoParams{})
				return err
			},
			wantRequests: 1,
			wantErr:      func(t *testing.T, err error) { require.ErrorIs(t, err, context.Canceled) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			handler := newGate(remote, tt.serve(cancel))
			client := newClient(t, handler, tt.opts...)

			err := tt.call(ctx, client)

			tt.wantErr(t, err)
			assert.Equal(t, tt.wantRequests, handler.requests.Load())
		})
	}
}

func TestCallAdaptsTimeouts(t *testing.T) {
	const (
		initialTimeout = 50 * time.Millisecond
		serverDelay    = 4 * initialTimeout
	)
	remote := testutils.Serve(t, testutils.NewChain(t))
	var slow atomic.Bool
	handler := newGate(remote, func(_ int64, r *http.Request) bool {
		if slow.Load() {
			select {
			case <-time.After(serverDelay):
			case <-r.Context().Done():
			}
		}
		return true
	})
	timeouts := timeout.New([]time.Duration{initialTimeout}, false)
	client := newClientWithTimeouts(t, handler, timeouts, WithMaxRetries(0))
	grownTimeout := timeout.New([]time.Duration{initialTimeout}, false)
	grownTimeout.IncreaseTimeout()

	steps := []struct {
		name        string
		slow        bool
		wantErr     error
		wantTimeout time.Duration
	}{
		{
			name:        "deadline exceeded grows the timeout",
			slow:        true,
			wantErr:     context.DeadlineExceeded,
			wantTimeout: grownTimeout.GetCurrentTimeout(),
		},
		{name: "success shrinks the timeout", wantTimeout: initialTimeout},
		{name: "success at the floor keeps the timeout", wantTimeout: initialTimeout},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			slow.Store(step.slow)

			_, err := client.Call(t.Context(), ChainID, NoParams{})

			if step.wantErr != nil {
				require.ErrorIs(t, err, step.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, step.wantTimeout, client.timeouts.Load().GetCurrentTimeout())
		})
	}
}

func TestListener(t *testing.T) {
	type response struct {
		method  string
		outcome string
	}
	remote := testutils.Serve(t, testutils.NewChain(t))
	var serve atomic.Bool
	handler := newGate(remote, func(int64, *http.Request) bool { return serve.Load() })
	var responses []response
	listener := &SelectiveListener{OnResponseCb: func(method, outcome string, took time.Duration) {
		responses = append(responses, response{method: method, outcome: outcome})
		assert.Positive(t, took)
	}}
	client := newClient(t, handler, WithListener(listener), WithMaxRetries(0))

	tests := []struct {
		name  string
		serve bool
		call  func(ctx context.Context) error
		want  response
	}{
		{
			name:  "success",
			serve: true,
			call: func(ctx context.Context) error {
				_, err := client.Call(ctx, ChainID, NoParams{})
				return err
			},
			want: response{method: "starknet_chainId", outcome: "ok"},
		},
		{
			name:  "rpc error",
			serve: true,
			call: func(ctx context.Context) error {
				_, err := client.Call(ctx, StateUpdate, BlockParams{BlockNumber: testutils.MissingBlockNumber})
				return err
			},
			want: response{method: "starknet_getStateUpdate", outcome: "24"},
		},
		{
			name: "transport error",
			call: func(ctx context.Context) error {
				_, err := client.Call(ctx, ChainID, NoParams{})
				return err
			},
			want: response{method: "starknet_chainId", outcome: "error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serve.Store(tt.serve)
			responses = nil

			err := tt.call(t.Context())

			if tt.want.outcome == "ok" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			assert.Equal(t, []response{tt.want}, responses)
		})
	}
}

func TestSetTimeouts(t *testing.T) {
	client := newClient(t, testutils.Serve(t, testutils.NewChain(t)))

	tests := []struct {
		name     string
		timeouts []time.Duration
		fixed    bool
	}{
		{name: "fixed single", timeouts: []time.Duration{5 * time.Second}, fixed: true},
		{
			name:     "fixed list",
			timeouts: []time.Duration{time.Second, 2 * time.Second, 3 * time.Second},
			fixed:    true,
		},
		{name: "dynamic", timeouts: []time.Duration{5 * time.Second}, fixed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client.SetTimeouts(tt.timeouts, tt.fixed)

			assert.Equal(t, timeout.New(tt.timeouts, tt.fixed).String(), client.Timeouts())
		})
	}
}

func TestErrorClassification(t *testing.T) {
	remote := testutils.Serve(t, testutils.NewChain(t))
	client := newClient(t, remote, WithMaxRetries(0))
	ctx := t.Context()
	_, blockNotFound := client.Call(
		ctx, StateUpdate, BlockParams{BlockNumber: testutils.MissingBlockNumber},
	)
	require.Error(t, blockNotFound)
	var ignored json.RawMessage
	invalidParams := client.rpc.CallContext(ctx, &ignored, Class.name)
	require.Error(t, invalidParams)

	tests := []struct {
		name          string
		err           error
		wantRetryable bool
		wantOutcome   string
	}{
		{
			name:          "plain error",
			err:           errors.New("boom"),
			wantRetryable: true,
			wantOutcome:   "error",
		},
		{
			name:          "http error",
			err:           gethrpc.HTTPError{StatusCode: http.StatusBadGateway},
			wantRetryable: true,
			wantOutcome:   "error",
		},
		{
			name:          "deadline exceeded",
			err:           context.DeadlineExceeded,
			wantRetryable: true,
			wantOutcome:   "error",
		},
		{
			name:          "block not found",
			err:           blockNotFound,
			wantRetryable: false,
			wantOutcome:   "24",
		},
		{
			name:          "invalid params",
			err:           invalidParams,
			wantRetryable: true,
			wantOutcome:   "-32602",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantRetryable, retryable(tt.err))
			assert.Equal(t, tt.wantOutcome, outcome(tt.err))
		})
	}
}
