package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

type simulator struct {
	*server
	*httptest.Server
	fixtures []fixture
}

func newSimulator(t *testing.T, config *config) *simulator {
	t.Helper()
	all := fixtures(t)
	logger := log.NewNopZapLogger()
	store, err := loadStore(t.Context(), writeDataset(t, all), config, logger)
	require.NoError(t, err)

	server := &server{
		store:  store,
		clock:  newClock(store.blocks, config, logger),
		config: config,
		logger: logger,
	}

	httpServer := httptest.NewServer(server.routes())
	t.Cleanup(httpServer.Close)
	return &simulator{server: server, Server: httpServer, fixtures: all}
}

type reply struct {
	status int
	header http.Header
	body   []byte
}

func (simulator *simulator) get(t *testing.T, path, query, acceptEncoding string) reply {
	t.Helper()
	requestURL := simulator.URL + path + "?" + query
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, requestURL, http.NoBody)
	require.NoError(t, err)
	// Setting the header explicitly stops the transport from decompressing gzip on its own.
	request.Header.Set("Accept-Encoding", acceptEncoding)

	response, err := simulator.Client().Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return reply{status: response.StatusCode, header: response.Header, body: body}
}

func (simulator *simulator) feederClient(t *testing.T) *feeder.Client {
	t.Helper()
	feederURL, err := url.Parse(simulator.URL + feederPrefix)
	require.NoError(t, err)
	return feeder.NewClient(feederURL, feeder.WithMaxRetries(0), feeder.WithBackoff(feeder.NopBackoff))
}

// routeCase expects the fixture body wantFile on success, a gateway error when
// wantCode is set, and a plain HTTP 500 otherwise.
type routeCase struct {
	name        string
	query       string
	gzip        bool
	wantFile    string
	wantCode    gateway.ErrorCode
	wantMessage string
}

func blockRouteCases() []routeCase {
	return []routeCase{
		{
			name:     "at tip",
			query:    "blockNumber=56377&headerOnly=true",
			wantFile: "get_block/56377.json.gz",
		},
		{
			name:     "at tip gzipped",
			query:    "blockNumber=56377&headerOnly=true",
			gzip:     true,
			wantFile: "get_block/56377.json.gz",
		},
		{
			name:     "latest",
			query:    "blockNumber=latest&headerOnly=true",
			wantFile: "get_block/56377.json.gz",
		},
		{
			name:        "above tip",
			query:       "blockNumber=56378&headerOnly=true",
			wantCode:    blockNotFound,
			wantMessage: "Block number 56378 was not found.",
		},
		{
			name:        "above tip gzipped",
			query:       "blockNumber=56378&headerOnly=true",
			gzip:        true,
			wantCode:    blockNotFound,
			wantMessage: "Block number 56378 was not found.",
		},
		{
			name:        "above to",
			query:       "blockNumber=56380&headerOnly=true",
			wantCode:    blockNotFound,
			wantMessage: "Block number 56380 was not found.",
		},
		{
			name:        "below from",
			query:       "blockNumber=56376&headerOnly=true",
			wantMessage: "block 56376 is below --from 56377; Juno's DB is probably not at 56376",
		},
		{
			name:        "without headerOnly",
			query:       "blockNumber=56377",
			wantCode:    malformedRequest,
			wantMessage: `get_block: unsupported query "blockNumber=56377"`,
		},
		{
			name:        "full body",
			query:       "blockNumber=56377&headerOnly=false",
			wantCode:    malformedRequest,
			wantMessage: "get_block: unsupported query",
		},
		{
			name:        "bad number",
			query:       "blockNumber=abc&headerOnly=true",
			wantCode:    malformedRequest,
			wantMessage: "get_block:",
		},
	}
}

func stateUpdateRouteCases() []routeCase {
	return []routeCase{
		{
			name:     "at tip",
			query:    "blockNumber=56377&includeBlock=true&includeSignature=true",
			wantFile: "get_state_update/56377.json.gz",
		},
		{
			name:     "latest gzipped",
			query:    "blockNumber=latest&includeBlock=true&includeSignature=true",
			gzip:     true,
			wantFile: "get_state_update/56377.json.gz",
		},
		{
			name:        "above tip",
			query:       "blockNumber=56378&includeBlock=true&includeSignature=true",
			wantCode:    blockNotFound,
			wantMessage: "Block number 56378 was not found.",
		},
		{
			name:        "without signature",
			query:       "blockNumber=56377&includeBlock=true",
			wantCode:    malformedRequest,
			wantMessage: "get_state_update: unsupported query",
		},
	}
}

func classRouteCases() []routeCase {
	return []routeCase{
		{
			name:     "declared above tip",
			query:    "classHash=" + sierraHash + "&blockNumber=latest",
			wantFile: "get_class_by_hash/" + sierraHash + ".json.gz",
		},
		{
			name:     "deprecated gzipped",
			query:    "classHash=" + deprecatedHash + "&blockNumber=latest",
			gzip:     true,
			wantFile: "get_class_by_hash/" + deprecatedHash + ".json.gz",
		},
		{
			name:        "at number",
			query:       "classHash=" + sierraHash + "&blockNumber=56377",
			wantCode:    malformedRequest,
			wantMessage: "get_class_by_hash: unsupported query",
		},
		{
			name:        "without block",
			query:       "classHash=" + sierraHash,
			wantCode:    malformedRequest,
			wantMessage: "get_class_by_hash: unsupported query",
		},
		{
			name:        "unknown",
			query:       "classHash=0x1&blockNumber=latest",
			wantMessage: "get_class_by_hash/0x1.json.gz: not in dataset",
		},
	}
}

func compiledClassRouteCases() []routeCase {
	return []routeCase{
		{
			name:     "sierra",
			query:    "classHash=" + sierraHash + "&blockNumber=latest",
			wantFile: "get_compiled_class_by_class_hash/" + sierraHash + ".json.gz",
		},
		{
			name:        "deprecated",
			query:       "classHash=" + deprecatedHash + "&blockNumber=latest",
			wantMessage: "not in dataset",
		},
	}
}

func TestServerRoutes(t *testing.T) {
	simulator := newSimulator(t, fixtureConfig())
	groups := []struct {
		path  string
		cases []routeCase
	}{
		{feederPrefix + "get_block", blockRouteCases()},
		{feederPrefix + "get_state_update", stateUpdateRouteCases()},
		{feederPrefix + "get_class_by_hash", classRouteCases()},
		{feederPrefix + "get_compiled_class_by_class_hash", compiledClassRouteCases()},
		{feederPrefix + "get_contract_addresses", []routeCase{
			{name: "served", wantFile: "get_contract_addresses.json.gz"},
		}},
		{feederPrefix + "get_preconfirmed_block", []routeCase{
			{
				name:        "never in window",
				query:       "blockNumber=56378",
				wantCode:    blockNotFound,
				wantMessage: "No pre-confirmed block.",
			},
		}},
		{feederPrefix + "get_signature", []routeCase{
			{
				name:        "unknown feeder endpoint",
				query:       "blockNumber=56377",
				wantCode:    malformedRequest,
				wantMessage: "unknown endpoint /feeder_gateway/get_signature",
			},
		}},
		{"/gateway/add_transaction", []routeCase{
			{
				name:        "gateway",
				wantCode:    malformedRequest,
				wantMessage: "unknown endpoint /gateway/add_transaction",
			},
		}},
		{"/", []routeCase{
			{name: "root", wantCode: malformedRequest, wantMessage: "unknown endpoint /"},
		}},
	}
	for _, group := range groups {
		t.Run(group.path, func(t *testing.T) {
			for _, test := range group.cases {
				t.Run(test.name, func(t *testing.T) {
					simulator.check(t, group.path, &test)
				})
			}
		})
	}
}

func (simulator *simulator) check(t *testing.T, path string, test *routeCase) {
	t.Helper()
	acceptEncoding := "identity"
	if test.gzip {
		acceptEncoding = "gzip"
	}
	reply := simulator.get(t, path, test.query, acceptEncoding)

	switch {
	case test.wantFile == "" && test.wantCode == "":
		require.Equal(t, http.StatusInternalServerError, reply.status)
		require.Empty(t, reply.header.Get("Content-Encoding"), "errors are never compressed")
		require.Contains(t, string(reply.body), test.wantMessage)
		return
	case test.wantFile == "":
		require.Equal(t, http.StatusBadRequest, reply.status)
		require.Equal(t, "application/json", reply.header.Get("Content-Type"))
		require.Empty(t, reply.header.Get("Content-Encoding"), "errors are never compressed")
		var failure gateway.Error
		require.NoError(t, json.Unmarshal(reply.body, &failure))
		require.Equal(t, test.wantCode, failure.Code)
		require.Contains(t, failure.Message, test.wantMessage)
		return
	}

	require.Equal(t, http.StatusOK, reply.status)
	require.Equal(t, "application/json", reply.header.Get("Content-Type"))
	body := reply.body
	if test.gzip {
		require.Equal(t, "gzip", reply.header.Get("Content-Encoding"))
		var err error
		body, err = gunzip(body)
		require.NoError(t, err)
	} else {
		require.Empty(t, reply.header.Get("Content-Encoding"))
	}
	require.Equal(t, fixtureBody(t, simulator.fixtures, test.wantFile), body)
}

func TestServerFollowsTip(t *testing.T) {
	simulator := newSimulator(t, fixtureConfig())
	query := "blockNumber=56378&headerOnly=true"

	hidden := simulator.get(t, feederPrefix+"get_block", query, "identity")
	require.Equal(t, http.StatusBadRequest, hidden.status)

	simulator.clock.current.Store(56378)

	served := simulator.get(t, feederPrefix+"get_block", query, "identity")
	require.Equal(t, http.StatusOK, served.status)
	require.Equal(t, fixtureBody(t, simulator.fixtures, "get_block/56378.json.gz"), served.body)

	latestQuery := "blockNumber=latest&headerOnly=true"
	latest := simulator.get(t, feederPrefix+"get_block", latestQuery, "identity")
	require.Equal(t, served.body, latest.body)
}

func TestServerLatency(t *testing.T) {
	config := fixtureConfig()
	config.latency = 50 * time.Millisecond
	simulator := newSimulator(t, config)

	started := time.Now()
	reply := simulator.get(t, feederPrefix+"get_contract_addresses", "", "identity")
	require.Equal(t, http.StatusOK, reply.status)
	require.GreaterOrEqual(t, time.Since(started), config.latency)
}

func requireBadRequest(t *testing.T, err error) {
	t.Helper()
	var status *feeder.StatusError
	require.ErrorAs(t, err, &status)
	require.Equal(t, http.StatusBadRequest, status.Code)
}

func mustFelt(t *testing.T, hex string) *felt.Felt {
	t.Helper()
	value, err := new(felt.Felt).SetString(hex)
	require.NoError(t, err)
	return value
}

func TestServerWithFeederClient(t *testing.T) {
	simulator := newSimulator(t, testConfig(fixtureFrom, fixtureTo, 56378))
	client := simulator.feederClient(t)
	ctx := t.Context()

	t.Run("block header", func(t *testing.T) {
		header, err := client.BlockHeader(ctx, "56377")
		require.NoError(t, err)
		require.Equal(t, uint64(56377), header.Number)
		require.NotNil(t, header.Hash)
	})
	t.Run("latest block header", func(t *testing.T) {
		header, err := client.BlockHeader(ctx, "latest")
		require.NoError(t, err)
		require.Equal(t, uint64(56378), header.Number)
	})
	t.Run("block header above tip", func(t *testing.T) {
		_, err := client.BlockHeader(ctx, "56379")
		requireBadRequest(t, err)
	})
	t.Run("state update with block and signature", func(t *testing.T) {
		update, err := client.StateUpdateWithBlockAndSignature(ctx, "56378")
		require.NoError(t, err)
		require.Equal(t, uint64(56378), update.Block.Number)
		require.Equal(t, uint64(1712214058), update.Block.Timestamp)
		require.NotNil(t, update.StateUpdate)
		require.NotEmpty(t, update.Signature)
	})
	t.Run("state update without block is unsupported", func(t *testing.T) {
		_, err := client.StateUpdate(ctx, "56377")
		requireBadRequest(t, err)
	})
	t.Run("sierra class", func(t *testing.T) {
		class, err := client.ClassDefinition(ctx, mustFelt(t, sierraHash))
		require.NoError(t, err)
		require.NotNil(t, class.Sierra)
		require.Nil(t, class.DeprecatedCairo)
	})
	t.Run("deprecated class", func(t *testing.T) {
		class, err := client.ClassDefinition(ctx, mustFelt(t, deprecatedHash))
		require.NoError(t, err)
		require.Nil(t, class.Sierra)
		require.NotNil(t, class.DeprecatedCairo)
	})
	t.Run("compiled class", func(t *testing.T) {
		_, err := client.CasmClassDefinition(ctx, mustFelt(t, sierraHash))
		require.NoError(t, err)
	})
	t.Run("contract addresses", func(t *testing.T) {
		_, err := client.FeeTokenAddresses(ctx)
		require.NoError(t, err)
	})
}
