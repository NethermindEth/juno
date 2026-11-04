package starknetrpc_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/clients/starknetrpc"
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

func newClient(
	t *testing.T,
	handler http.Handler,
	opts ...starknetrpc.Option,
) *starknetrpc.Client {
	t.Helper()
	return newClientWithTimeouts(t, handler, timeout.New([]time.Duration{time.Second}, true), opts...)
}

func newClientWithTimeouts(
	t *testing.T,
	handler http.Handler,
	timeouts *timeout.Timeouts,
	opts ...starknetrpc.Option,
) *starknetrpc.Client {
	t.Helper()
	serverURL := testutils.Listen(t, handler)
	opts = append(
		[]starknetrpc.Option{
			starknetrpc.WithMinWait(time.Millisecond),
			starknetrpc.WithMaxWait(time.Millisecond),
		},
		opts...,
	)
	client, err := starknetrpc.New(t.Context(), serverURL, timeouts, log.NewNopZapLogger(), opts...)
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

// runCall checks that call answers like the handler call want does. want may answer by
// value or by pointer; normalize runs on both sides before they are compared.
func runCall[R, W any](
	t *testing.T,
	name string,
	call func(ctx context.Context) (*R, error),
	want func() (W, *jsonrpc.Error),
	normalize ...func(*R),
) {
	t.Run(name, func(t *testing.T) {
		answer, rpcErr := want()
		requireNoRPCError(t, rpcErr)

		got, err := call(t.Context())

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

			client, err := starknetrpc.New(t.Context(), clientURL, timeouts, log.NewNopZapLogger())

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Nil(t, client)
				return
			}
			require.NoError(t, err)
			t.Cleanup(client.Close)
			assert.Equal(t, "rpcsync", client.Name())
			assert.Equal(t, timeouts.String(), client.Timeouts())
		})
	}
}

func TestWithUserAgent(t *testing.T) {
	remote := testutils.Serve(t, testutils.NewChain(t))

	tests := []struct {
		name string
		opts []starknetrpc.Option
		want string
	}{
		{name: "default", want: "Go-http-client/1.1"},
		{
			name: "custom",
			opts: []starknetrpc.Option{starknetrpc.WithUserAgent("juno-test/1")},
			want: "juno-test/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userAgent string
			handler := newGate(remote, func(_ int64, r *http.Request) bool {
				userAgent = r.UserAgent()
				return true
			})
			client := newClient(t, handler, tt.opts...)

			_, err := client.Call(t.Context(), starknetrpc.ChainID, starknetrpc.NoParams{})

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

	runCall(
		t, "chain id",
		func(ctx context.Context) (*felt.Felt, error) {
			return client.Call(ctx, starknetrpc.ChainID, starknetrpc.NoParams{})
		},
		handler.ChainID,
	)
	runCall(
		t, "spec version",
		func(ctx context.Context) (*string, error) {
			return client.Call(ctx, starknetrpc.SpecVersion, starknetrpc.NoParams{})
		},
		handler.SpecVersion,
	)
	runCall(
		t, "block hash and number",
		func(ctx context.Context) (*rpcv10.BlockHashAndNumber, error) {
			return client.Call(ctx, starknetrpc.BlockHashAndNumber, starknetrpc.NoParams{})
		},
		handler.BlockHashAndNumber,
	)
	runCall(
		t, "block with receipts",
		func(ctx context.Context) (*rpcv10.BlockWithReceipts, error) {
			params := starknetrpc.BlockWithReceiptsParams{BlockNumber: blockNumber}
			return client.Call(ctx, starknetrpc.BlockWithReceipts, params)
		},
		func() (*rpcv10.BlockWithReceipts, *jsonrpc.Error) {
			return handler.BlockWithReceipts(&blockID, rpcv10.ResponseFlags{IncludeProofFacts: true})
		},
	)
	runCall(
		t, "state update",
		func(ctx context.Context) (*rpcv10.StateUpdate, error) {
			params := starknetrpc.BlockParams{BlockNumber: blockNumber}
			return client.Call(ctx, starknetrpc.StateUpdate, params)
		},
		func() (rpcv10.StateUpdate, *jsonrpc.Error) { return handler.StateUpdate(&blockID, nil) },
		func(update *rpcv10.StateUpdate) { sortStateDiff(update.StateDiff) },
	)
	runCall(
		t, "deprecated class",
		func(ctx context.Context) (*rpcv10.Class, error) {
			params := starknetrpc.ClassParams{BlockNumber: blockNumber, ClassHash: deprecatedClass}
			return client.Call(ctx, starknetrpc.Class, params)
		},
		func() (*rpcv10.Class, *jsonrpc.Error) { return handler.Class(&blockID, deprecatedClass) },
	)
	runCall(
		t, "sierra class",
		func(ctx context.Context) (*rpcv10.Class, error) {
			params := starknetrpc.ClassParams{BlockNumber: blockNumber, ClassHash: sierraClass}
			return client.Call(ctx, starknetrpc.Class, params)
		},
		func() (*rpcv10.Class, *jsonrpc.Error) { return handler.Class(&blockID, sierraClass) },
	)
	runCall(
		t, "compiled casm",
		func(ctx context.Context) (*rpcv10.CompiledCasmResponse, error) {
			params := starknetrpc.ClassHashParams{ClassHash: sierraClass}
			return client.Call(ctx, starknetrpc.CompiledCasm, params)
		},
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
		opts         []starknetrpc.Option
		call         func(ctx context.Context, client *starknetrpc.Client) error
		wantRequests int64
		wantErr      func(t *testing.T, err error)
	}{
		{
			name:  "block not found is not retried",
			serve: func(context.CancelFunc) func(int64, *http.Request) bool { return serveAll },
			call: func(ctx context.Context, client *starknetrpc.Client) error {
				params := starknetrpc.BlockWithReceiptsParams{BlockNumber: testutils.MissingBlockNumber}
				_, err := client.Call(ctx, starknetrpc.BlockWithReceipts, params)
				return err
			},
			wantRequests: 1,
			wantErr: func(t *testing.T, err error) {
				var rpcErr gethrpc.Error
				require.ErrorAs(t, err, &rpcErr)
				assert.Equal(t, rpccore.ErrBlockNotFound.Code, rpcErr.ErrorCode())
				assert.NotContains(t, err.Error(), "giving up")
			},
		},
		{
			name: "transient failures are retried",
			serve: func(context.CancelFunc) func(int64, *http.Request) bool {
				return func(request int64, _ *http.Request) bool { return request > 2 }
			},
			opts: []starknetrpc.Option{starknetrpc.WithMaxRetries(2)},
			call: func(ctx context.Context, client *starknetrpc.Client) error {
				_, err := client.Call(ctx, starknetrpc.ChainID, starknetrpc.NoParams{})
				return err
			},
			wantRequests: 3,
			wantErr:      func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "timed out requests are retried",
			serve: func(context.CancelFunc) func(int64, *http.Request) bool {
				return func(request int64, r *http.Request) bool {
					if request > 1 {
						return true
					}
					// The server notices the client hanging up only once it has read the body.
					if _, err := io.Copy(io.Discard, r.Body); err == nil {
						<-r.Context().Done()
					}
					return false
				}
			},
			opts: []starknetrpc.Option{starknetrpc.WithMaxRetries(1)},
			call: func(ctx context.Context, client *starknetrpc.Client) error {
				_, err := client.Call(ctx, starknetrpc.ChainID, starknetrpc.NoParams{})
				return err
			},
			wantRequests: 2,
			wantErr:      func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:  "other rpc errors are retried",
			serve: func(context.CancelFunc) func(int64, *http.Request) bool { return serveAll },
			opts:  []starknetrpc.Option{starknetrpc.WithMaxRetries(2)},
			call: func(ctx context.Context, client *starknetrpc.Client) error {
				params := starknetrpc.ClassParams{
					BlockNumber: testutils.ChainHeight - 1,
					ClassHash:   felt.NewFromUint64[felt.Felt](1),
				}
				_, err := client.Call(ctx, starknetrpc.Class, params)
				return err
			},
			wantRequests: 3,
			wantErr: func(t *testing.T, err error) {
				var rpcErr gethrpc.Error
				require.ErrorAs(t, err, &rpcErr)
				assert.Equal(t, rpccore.ErrClassHashNotFound.Code, rpcErr.ErrorCode())
				assert.Contains(t, err.Error(), "giving up after 2 retries")
			},
		},
		{
			name:  "gives up after max retries",
			serve: func(context.CancelFunc) func(int64, *http.Request) bool { return serveNone },
			opts:  []starknetrpc.Option{starknetrpc.WithMaxRetries(2)},
			call: func(ctx context.Context, client *starknetrpc.Client) error {
				_, err := client.Call(ctx, starknetrpc.ChainID, starknetrpc.NoParams{})
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
			opts: []starknetrpc.Option{
				starknetrpc.WithMinWait(time.Hour),
				starknetrpc.WithMaxWait(time.Hour),
			},
			call: func(ctx context.Context, client *starknetrpc.Client) error {
				_, err := client.Call(ctx, starknetrpc.ChainID, starknetrpc.NoParams{})
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

func TestCallDefaultBackoff(t *testing.T) {
	const wantBackoff = 500*time.Millisecond + time.Second + 8*2*time.Second
	handler := newGate(http.NotFoundHandler(), serveNone)
	// synctest advances time only once every goroutine in the bubble is durably blocked: the
	// server lives outside it, and closed connections keep the transport out of network reads.
	serverURL := testutils.Listen(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "close")
		handler.ServeHTTP(w, r)
	}))

	synctest.Test(t, func(t *testing.T) {
		timeouts := timeout.New([]time.Duration{time.Second}, true)
		client, err := starknetrpc.New(t.Context(), serverURL, timeouts, log.NewNopZapLogger())
		require.NoError(t, err)
		t.Cleanup(client.Close)
		start := time.Now()

		_, err = client.Call(t.Context(), starknetrpc.ChainID, starknetrpc.NoParams{})

		require.ErrorContains(t, err, "giving up after 10 retries")
		assert.Equal(t, wantBackoff, time.Since(start))
		assert.Equal(t, int64(11), handler.requests.Load())
	})
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
	client := newClientWithTimeouts(t, handler, timeouts, starknetrpc.WithMaxRetries(0))
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

			_, err := client.Call(t.Context(), starknetrpc.ChainID, starknetrpc.NoParams{})

			if step.wantErr != nil {
				require.ErrorIs(t, err, step.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, step.wantTimeout, timeouts.GetCurrentTimeout())
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
	listener := &starknetrpc.SelectiveListener{
		OnResponseCb: func(method, outcome string, took time.Duration) {
			responses = append(responses, response{method: method, outcome: outcome})
			assert.Positive(t, took)
		},
	}
	client := newClient(
		t, handler, starknetrpc.WithListener(listener), starknetrpc.WithMaxRetries(0),
	)

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
				_, err := client.Call(ctx, starknetrpc.ChainID, starknetrpc.NoParams{})
				return err
			},
			want: response{method: "starknet_chainId", outcome: "ok"},
		},
		{
			name:  "rpc error",
			serve: true,
			call: func(ctx context.Context) error {
				params := starknetrpc.BlockParams{BlockNumber: testutils.MissingBlockNumber}
				_, err := client.Call(ctx, starknetrpc.StateUpdate, params)
				return err
			},
			want: response{method: "starknet_getStateUpdate", outcome: "24"},
		},
		{
			name: "transport error",
			call: func(ctx context.Context) error {
				_, err := client.Call(ctx, starknetrpc.ChainID, starknetrpc.NoParams{})
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
