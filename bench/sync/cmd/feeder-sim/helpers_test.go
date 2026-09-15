package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/gateway"
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
