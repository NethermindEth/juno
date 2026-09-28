package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

// Sepolia blocks 56377..56379 are the only contiguous range in the feeder fixtures.
// The classes they reference are not in the fixtures, so real class bodies are stored
// under the referenced hashes; the simulator never verifies a class against its hash.
const (
	fixtureFrom uint64 = 56377
	fixtureTo   uint64 = 56379

	deprecatedHash = "0x29927c8af6bccf3f6fda035981e765a7bdbf18a2dc0d630494f8758aa908e2b"
	sierraHash     = "0x4106c8a9ad636d002880eaa19da8fbf37cabfd8ca933ce87c06915b1e6ddf8e"

	deprecatedFixture = "0x28d1671fb74ecb54d848d463cefccffaef6df3ae40db52130e19fe8299a7b43"
	sierraFixture     = "0x3cc90db763e736ca9b6c581ea4008408842b1a125947ab087438676a7e40b7b"

	contractAddressesJSON = `{"GpsStatementVerifier": "0x07ec0D28e50322Eb0C159B9090ecF3aeA8346DFe",` +
		` "Starknet": "0xE2Bb56ee936fd6433DC0F6e7e3b8365C906AA057"}`
)

var fixtureRoot = filepath.Join("..", "..", "..", "..", "clients", "feeder", "testdata", "sepolia")

type fixture struct {
	resource resource
	body     []byte
}

func readFixture(t *testing.T, elements ...string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(append([]string{fixtureRoot}, elements...)...))
	require.NoError(t, err)
	return body
}

func mustResource[K, F comparable](t *testing.T, endpoint *endpoint[K, F], key K) resource {
	t.Helper()
	resource, err := endpoint.resource(key)
	require.NoError(t, err)
	return resource
}

func fixtures(t *testing.T) []fixture {
	t.Helper()
	deprecated := classKey{ClassHash: deprecatedHash}
	sierra := classKey{ClassHash: sierraHash}
	all := []fixture{
		{mustResource(t, contractAddresses, struct{}{}), []byte(contractAddressesJSON)},
		{mustResource(t, classByHash, deprecated), readFixture(t, "class", deprecatedFixture+".json")},
		{mustResource(t, classByHash, sierra), readFixture(t, "class", sierraFixture+".json")},
		{mustResource(t, compiledClass, sierra), readFixture(t, "compiled_class", sierraFixture+".json")},
	}
	for number := fixtureFrom; number <= fixtureTo; number++ {
		key := blockKey{BlockNumber: number}
		file := strconv.FormatUint(number, 10) + ".json"
		all = append(all,
			fixture{mustResource(t, block, key), readFixture(t, "block", file)},
			fixture{mustResource(t, stateUpdate, key), readFixture(t, "state_update_with_block", file)},
		)
	}
	return all
}

func fixtureBody(t *testing.T, all []fixture, file string) []byte {
	t.Helper()
	for _, fixture := range all {
		if fixture.resource.file == file {
			return fixture.body
		}
	}
	require.Failf(t, "fixture not found", "%s", file)
	return nil
}

func writeDataset(t *testing.T, all []fixture) dataset {
	t.Helper()
	dataset := dataset{root: t.TempDir()}
	for _, fixture := range all {
		gzipped, err := gzipBytes(fixture.body)
		require.NoError(t, err)
		require.NoError(t, dataset.write(fixture.resource.file, gzipped))
	}
	return dataset
}

func fixtureDataset(t *testing.T) dataset {
	t.Helper()
	return writeDataset(t, fixtures(t))
}

func testConfig(from, to, tip uint64) *config {
	return &config{
		from:           from,
		to:             to,
		tip:            tip,
		concurrency:    defaultConcurrency,
		captureTimeout: defaultCaptureTimeout,
		interval:       time.Millisecond,
	}
}

func fixtureConfig() *config {
	return testConfig(fixtureFrom, fixtureTo, fixtureFrom)
}

// upstream stands in for the real feeder gateway during capture tests. It serves the
// fixture bodies uncompressed and answers anything else with a gateway 400.
type upstream struct {
	*httptest.Server
	requests atomic.Int64
	bodies   map[string][]byte
}

func newUpstream(t *testing.T, all []fixture) *upstream {
	t.Helper()
	upstream := &upstream{bodies: make(map[string][]byte, len(all))}
	base := &url.URL{Path: feederPrefix}
	for _, fixture := range all {
		upstream.bodies[fixture.resource.url(base).RequestURI()] = fixture.body
	}
	upstream.Server = httptest.NewServer(http.HandlerFunc(upstream.handle))
	t.Cleanup(upstream.Close)
	return upstream
}

func (upstream *upstream) handle(writer http.ResponseWriter, request *http.Request) {
	upstream.requests.Add(1)
	body, ok := upstream.bodies[request.URL.RequestURI()]
	if !ok {
		writer.WriteHeader(http.StatusBadRequest)
		body, _ = json.Marshal(gateway.Error{Code: blockNotFound, Message: request.URL.RequestURI()})
	}
	writer.Write(body) //nolint:errcheck // test server
}

func (upstream *upstream) feederURL(t *testing.T) *url.URL {
	t.Helper()
	feederURL, err := url.Parse(upstream.URL + feederPrefix)
	require.NoError(t, err)
	return feederURL
}

func (upstream *upstream) network(t *testing.T) *networks.Network {
	t.Helper()
	network := networks.Sepolia
	network.FeederURL = upstream.feederURL(t)
	return &network
}

// rpcStateDiffJSON is a starknet_traceBlockTransactions state diff with every field kind set.
const rpcStateDiffJSON = `{
	"storage_diffs": [{
		"address": "0xa1",
		"storage_entries": [{"key": "0x1", "value": "0x10"}, {"key": "0x2", "value": "0x20"}]
	}],
	"nonces": [{"contract_address": "0xa1", "nonce": "0x5"}],
	"deployed_contracts": [{"address": "0xa2", "class_hash": "0xc1"}],
	"deprecated_declared_classes": ["0xc0"],
	"declared_classes": [{"class_hash": "0xc1", "compiled_class_hash": "0xd1"}],
	"replaced_classes": [{"contract_address": "0xa3", "class_hash": "0xc2"}],
	"migrated_compiled_classes": [{"class_hash": "0xc3", "compiled_class_hash": "0xd3"}]
}`

const rpcBlockNotFound = `{"jsonrpc": "2.0", "id": 1, ` +
	`"error": {"code": 24, "message": "Block not found"}}`

// traceFixture is a real starknet_traceBlockTransactions result for Sepolia block 3, whose
// pre-0.13.1 header lacks the gas prices a pre-confirmed block needs.
var traceFixture = filepath.Join(
	"..", "..", "..", "..", "rpc", "v10", "testdata", "traces", "sepolia_block_3.json",
)

func rpcResult(result []byte) []byte {
	return []byte(`{"jsonrpc": "2.0", "id": 1, "result": ` + string(result) + `}`)
}

func fixtureBlock(t *testing.T, number uint64) *confirmedBlock {
	t.Helper()
	var response rawBlock
	file := strconv.FormatUint(number, 10) + ".json"
	require.NoError(t, json.Unmarshal(readFixture(t, "state_update_with_block", file), &response))
	require.NotNil(t, response.Block)
	return response.Block
}

// syntheticTraces is the starknet_traceBlockTransactions result for a fixture block: the full
// diff on the first transaction, an empty one on the second and null on the rest.
func syntheticTraces(t *testing.T, block *confirmedBlock) []byte {
	t.Helper()
	traces := make([]json.RawMessage, 0, len(block.Transactions))
	for index, transaction := range block.Transactions {
		var fields struct {
			Hash string `json:"transaction_hash"`
		}
		require.NoError(t, json.Unmarshal(transaction, &fields))

		diff := "null"
		switch index {
		case 0:
			diff = rpcStateDiffJSON
		case 1:
			diff = "{}"
		}
		trace := fmt.Sprintf(
			`{"transaction_hash": %q, "trace_root": {"state_diff": %s}}`,
			fields.Hash,
			diff,
		)
		traces = append(traces, json.RawMessage(trace))
	}

	result, err := json.Marshal(traces)
	require.NoError(t, err)
	return result
}

func decodeTraceResult(t *testing.T, result []byte) []tracedTransaction {
	t.Helper()
	var traces []tracedTransaction
	require.NoError(t, json.Unmarshal(result, &traces))
	return traces
}

func fixtureTraces(t *testing.T) map[uint64][]byte {
	t.Helper()
	traces := make(map[uint64][]byte, fixtureTo-fixtureFrom+1)
	for number := fixtureFrom; number <= fixtureTo; number++ {
		traces[number] = syntheticTraces(t, fixtureBlock(t, number))
	}
	return traces
}

// preConfirmedFixtures are the rounds --preconfirmed capture builds for the fixture blocks.
func preConfirmedFixtures(t *testing.T) []fixture {
	t.Helper()
	all := make([]fixture, 0, fixtureTo-fixtureFrom+1)
	for number := fixtureFrom; number <= fixtureTo; number++ {
		block := fixtureBlock(t, number)
		round, err := buildRound(number, block, decodeTraceResult(t, syntheticTraces(t, block)))
		require.NoError(t, err)
		plain, err := gunzip(round)
		require.NoError(t, err)
		resource := mustResource(t, preConfirmedBlock, blockKey{BlockNumber: number})
		all = append(all, fixture{resource, plain})
	}
	return all
}

// rpcUpstream stands in for the JSON-RPC node during --preconfirmed capture. It answers
// starknet_traceBlockTransactions from traces keyed by block number, uncompressed, and
// everything else with a JSON-RPC "Block not found" error.
type rpcUpstream struct {
	*httptest.Server
	requests atomic.Int64
	traces   map[uint64][]byte
}

func newRPCUpstream(t *testing.T, traces map[uint64][]byte) *rpcUpstream {
	t.Helper()
	upstream := &rpcUpstream{traces: traces}
	upstream.Server = httptest.NewServer(http.HandlerFunc(upstream.handle))
	t.Cleanup(upstream.Close)
	return upstream
}

func (upstream *rpcUpstream) handle(writer http.ResponseWriter, request *http.Request) {
	upstream.requests.Add(1)
	var call traceRequest
	if err := json.NewDecoder(request.Body).Decode(&call); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	body := []byte(rpcBlockNotFound)
	result, ok := upstream.traces[call.Params.BlockID.BlockNumber]
	if ok && call.Method == "starknet_traceBlockTransactions" {
		body = rpcResult(result)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Write(body) //nolint:errcheck // test server
}

func (upstream *rpcUpstream) url(t *testing.T) *url.URL {
	t.Helper()
	rpcURL, err := url.Parse(upstream.URL)
	require.NoError(t, err)
	return rpcURL
}

// preConfirmedConfig serves a window of [tip-1, tip+2] over the fixture range. Its 1h interval
// splits into four 15m phases, so a test picks a phase by sleeping inside a synctest bubble.
func preConfirmedConfig(tip uint64) *config {
	config := testConfig(fixtureFrom, fixtureTo, tip)
	config.preconfirmed = true
	config.lead = 2
	config.keep = 1
	config.stages = 3
	config.interval = time.Hour
	return config
}

// preConfirmedStore loads the fixtures and their rounds; overrides replace files before loading.
func preConfirmedStore(t *testing.T, overrides ...fixture) *store {
	t.Helper()
	dataset := writeDataset(t, slices.Concat(fixtures(t), preConfirmedFixtures(t), overrides))
	config := preConfirmedConfig(fixtureFrom)
	store, err := loadStore(t.Context(), dataset, config, log.NewNopZapLogger())
	require.NoError(t, err)
	return store
}

// decodedRounds are the fixture rounds as Juno decodes them, by block number.
func decodedRounds(t *testing.T) map[uint64]*starknet.PreConfirmedBlock {
	t.Helper()
	rounds := make(map[uint64]*starknet.PreConfirmedBlock, fixtureTo-fixtureFrom+1)
	for index, fixture := range preConfirmedFixtures(t) {
		envelope, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(fixture.body))
		require.NoError(t, err)
		round, ok := envelope.Update.(starknet.PreConfirmedBlock)
		require.True(t, ok, "got %T", envelope.Update)
		rounds[fixtureFrom+uint64(index)] = &round
	}
	return rounds
}

type replyKind int

const (
	wantFull replyKind = iota
	wantDelta
	wantNoChange
	wantNotFound
)

// wantUpdate is what Juno decodes for the transactions [from, to) of a decoded round.
func wantUpdate(
	round *starknet.PreConfirmedBlock,
	kind replyKind,
	from, to uint64,
) starknet.PreConfirmedUpdate {
	switch kind {
	case wantFull:
		full := *round
		full.Transactions = round.Transactions[from:to]
		full.Receipts = round.Receipts[from:to]
		full.TransactionStateDiffs = round.TransactionStateDiffs[from:to]
		return full
	case wantDelta:
		return starknet.PreConfirmedDeltaUpdate{
			BlockIdentifier:       round.BlockIdentifier,
			Transactions:          round.Transactions[from:to],
			Receipts:              round.Receipts[from:to],
			TransactionStateDiffs: round.TransactionStateDiffs[from:to],
		}
	default:
		return starknet.PreConfirmedNoChange{}
	}
}

func newTestServer(t *testing.T, store *store, config *config) *server {
	t.Helper()
	logger := log.NewNopZapLogger()
	server := &server{
		store:  store,
		clock:  newClock(store.blocks, config, logger),
		config: config,
		logger: logger,
	}

	if config.preconfirmed {
		window, err := newWindow(store, config, logger, server.clock.position())
		require.NoError(t, err)
		server.window = window
	}

	return server
}

func newFeederClient(t *testing.T, baseURL string, httpClient *http.Client) *feeder.Client {
	t.Helper()
	feederURL, err := url.Parse(baseURL + feederPrefix)
	require.NoError(t, err)
	return feeder.NewClient(
		feederURL,
		feeder.WithHTTPClient(httpClient),
		feeder.WithMaxRetries(0),
		feeder.WithBackoff(feeder.NopBackoff),
	)
}

// pipeSimulator serves the routes to Juno's feeder client over net.Pipe, for synctest bubbles.
// A goroutine waiting on a loopback socket is not durably blocked, so an httptest.Server
// would keep a bubble from ever advancing its fake clock.
type pipeSimulator struct {
	*server
	client *feeder.Client
}

func newPipeSimulator(t *testing.T, store *store, config *config) *pipeSimulator {
	t.Helper()
	server := newTestServer(t, store, config)
	listener := newPipeListener()
	httpServer := &http.Server{Handler: server.routes(), ReadHeaderTimeout: readHeaderTimeout}
	served := make(chan error, 1)
	go func() { served <- httpServer.Serve(listener) }()

	transport := &http.Transport{DialContext: listener.dial}
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		require.NoError(t, httpServer.Close())
		require.ErrorIs(t, <-served, http.ErrServerClosed)
	})

	client := newFeederClient(t, "http://feeder-sim", &http.Client{Transport: transport})
	return &pipeSimulator{server: server, client: client}
}

type pipeListener struct {
	conns  chan net.Conn
	closed chan struct{}
	close  sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{conns: make(chan net.Conn), closed: make(chan struct{})}
}

func (listener *pipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-listener.conns:
		return conn, nil
	case <-listener.closed:
		return nil, net.ErrClosed
	}
}

func (listener *pipeListener) Close() error {
	listener.close.Do(func() { close(listener.closed) })
	return nil
}

func (listener *pipeListener) Addr() net.Addr {
	return pipeAddr{}
}

func (listener *pipeListener) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	server, client := net.Pipe()
	select {
	case listener.conns <- server:
		return client, nil
	case <-listener.closed:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }

func (pipeAddr) String() string { return "pipe" }
