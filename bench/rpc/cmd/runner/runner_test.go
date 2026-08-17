package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadAndValidateConfig(t *testing.T) {
	t.Parallel()
	environment := validEnvironment()
	config := loadConfig(
		func(name string) (string, bool) {
			value, ok := environment[name]
			return value, ok
		},
		func(string) ([]byte, error) {
			return []byte("0123456789abcdef0123456789abcdef01234567\n"), nil
		},
		time.Date(2026, time.August, 13, 3, 4, 5, 0, time.UTC),
	)
	if err := config.validate(); err != nil {
		t.Fatalf("validate config: %v", err)
	}

	if config.runID != "20260813T030405Z-0123456789ab" {
		t.Fatalf("unexpected run ID %q", config.runID)
	}
	if !reflect.DeepEqual(config.rates, []uint64{10, 20}) {
		t.Fatalf("unexpected rates %#v", config.rates)
	}
	if config.normalizedRates != "10,20" {
		t.Fatalf("unexpected normalized rates %q", config.normalizedRates)
	}
	if config.expectedBlock != 800000 || config.readyTimeout != 10*time.Second {
		t.Fatalf("unexpected parsed config: %#v", config)
	}
}

func TestConfigValidationFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*config)
		want   string
	}{
		{
			name: "rates syntax", mutate: func(c *config) { c.ratesRaw = "1e3" },
			want: "RATES must be a comma-separated list of integers",
		},
		{
			name: "rates range", mutate: func(c *config) { c.ratesRaw = "0" },
			want: "RATES must contain positive 32-bit integers",
		},
		{
			name: "iterations", mutate: func(c *config) { c.iterationsRaw = "invalid" },
			want: "ITERATIONS must be a positive integer",
		},
		{
			name: "block", mutate: func(c *config) { c.expectedBlockNumber = "invalid" },
			want: "EXPECTED_BLOCK_NUMBER must be a non-negative integer",
		},
		{
			name: "timeout", mutate: func(c *config) { c.readyTimeoutRaw = "30x" },
			want: "READY_TIMEOUT must be a positive duration",
		},
		{
			name: "poll interval", mutate: func(c *config) { c.readyPollIntervalRaw = "30x" },
			want: "READY_POLL_INTERVAL must be a positive duration",
		},
		{
			name: "duration", mutate: func(c *config) { c.concurrencyDurationRaw = "30x" },
			want: "CONCURRENCY_DURATION must be a positive duration",
		},
		{
			name: "missing variable", mutate: func(c *config) { c.snapshotID = "" },
			want: "SNAPSHOT_ID is required",
		},
		{
			name: "image digest", mutate: func(c *config) { c.junoImageDigest = "sha256:juno" },
			want: "JUNO_IMAGE_DIGEST must be sha256:<64 hex>",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config := validConfig()
			test.mutate(config)
			err := config.validate()
			if err == nil || err.Error() != test.want {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseSummaryMetricsSupportsBothK6Shapes(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "summary.json")
	data := `{
  "metrics": {
    "checks": {"fails": 2},
    "http_req_failed": {"passes": 4},
    "vu_failures": {"values": {"count": 5}},
    "iterations": {"count": 6}
  }
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	metrics, err := parseSummaryMetrics(path)
	if err != nil {
		t.Fatalf("parse metrics: %v", err)
	}
	want := &summaryMetrics{
		FailedChecks: 2, RequestFailures: 2, HTTPRequestFailures: 4,
		VUFailures: 5, DroppedIterations: 0, CompletedIterations: 6,
	}
	if !reflect.DeepEqual(metrics, want) {
		t.Fatalf("got %#v, want %#v", metrics, want)
	}

	if err := os.WriteFile(path, []byte(`{"metrics":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = parseSummaryMetrics(path)
	if err == nil || err.Error() != "missing metric checks.fails" {
		t.Fatalf("unexpected missing metric error: %v", err)
	}
}

func TestPromoteScenarioSummaryRemovesInvalidArtifact(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	temporary := filepath.Join(directory, ".single.json.tmp")
	result := filepath.Join(directory, "single.json")
	if err := os.WriteFile(temporary, []byte("not JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := promoteScenarioSummary(temporary, result); err == nil {
		t.Fatal("invalid summary was accepted")
	}
	for _, path := range []string{temporary, result} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("invalid artifact remains at %s", path)
		}
	}
}

func TestValidateCorpus(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	corpusPath := filepath.Join(directory, "corpus.json")
	corpus := []byte(`{"meta":{"method":"test"},"requests":[{"id":1}]}`)
	if err := os.WriteFile(corpusPath, corpus, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(corpus)
	checksum := hex.EncodeToString(digest[:]) + "  corpus.json\n"
	if err := os.WriteFile(corpusPath+".sha256", []byte(checksum), 0o644); err != nil {
		t.Fatal(err)
	}
	r := newRunner(validConfig(), io.Discard, io.Discard)
	r.config.corpusPath = corpusPath
	if err := r.validateCorpus(); err != nil {
		t.Fatalf("validate corpus: %v", err)
	}
	if r.actualCorpusSHA != hex.EncodeToString(digest[:]) ||
		string(r.corpusMeta) != `{"method":"test"}` {
		t.Fatalf("unexpected corpus state: sha=%q meta=%s", r.actualCorpusSHA, r.corpusMeta)
	}

	badChecksum := strings.Repeat("0", 64) + "  corpus.json\n"
	if err := os.WriteFile(corpusPath+".sha256", []byte(badChecksum), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.validateCorpus(); err == nil || err.Error() != "corpus checksum mismatch" {
		t.Fatalf("unexpected checksum error: %v", err)
	}
}

func TestScenarioArgs(t *testing.T) {
	config := validConfig()
	if err := config.validate(); err != nil {
		t.Fatal(err)
	}
	r := newRunner(config, io.Discard, io.Discard)
	tests := []struct {
		name string
		want []string
	}{
		{stageSingle, []string{
			"run", "--quiet", "-e", "NODE_URL=http://127.0.0.1:6060/v0_10",
			"--vus", "1", "--iterations", "2", k6SummaryExportFlag, "/tmp/summary.json",
			"--summary-trend-stats", "avg,min,med,p(90),p(99),max", "/bench/rpc/run.js",
		}},
		{stageConcurrency, []string{
			"run", "--quiet", "-e", "NODE_URL=http://127.0.0.1:6060/v0_10",
			"--vus", "1", "--duration", "1s", k6SummaryExportFlag, "/tmp/summary.json",
			"--summary-trend-stats", "avg,min,med,p(90),p(99),max", "/bench/rpc/run.js",
		}},
		{stageThroughput, []string{
			"run", "--quiet", "-e", "NODE_URL=http://127.0.0.1:6060/v0_10",
			"-e", "RATES=10,20", "-e", "DURATION=1s", "-e", "THROUGHPUT_VUS=3",
			k6SummaryExportFlag, "/tmp/summary.json", "--summary-trend-stats",
			"avg,min,med,p(90),p(99),max", "/bench/rpc/throughput.js",
		}},
	}
	for _, test := range tests {
		args, err := r.scenarioArgs(test.name, "/tmp/summary.json")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(args, test.want) {
			t.Fatalf("%s args = %#v, want %#v", test.name, args, test.want)
		}
	}
}

func TestTailWriter(t *testing.T) {
	input := strings.Repeat("x", maxCapturedStderr) + "tail"
	var writer tailWriter
	for _, chunk := range []string{input[:maxCapturedStderr], input[maxCapturedStderr:]} {
		written, err := writer.Write([]byte(chunk))
		if err != nil || written != len(chunk) {
			t.Fatalf("Write() = (%d, %v), want (%d, nil)", written, err, len(chunk))
		}
	}
	if got, want := string(writer.data), input[len(input)-maxCapturedStderr:]; got != want {
		t.Fatalf("captured stderr length %d, want tail length %d", len(got), len(want))
	}
}

func TestRunnerClassifiesConfigurationAndTargetFailures(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*config)
		wantStatus int
		wantStage  string
		wantReason string
	}{
		{
			name: "configuration", mutate: func(config *config) { config.iterationsRaw = "invalid" },
			wantStatus: 2, wantStage: "configuration", wantReason: "ITERATIONS must be a positive integer",
		},
		{
			name: "target", mutate: func(config *config) { config.expectedChainID = "0xBAD" },
			wantStatus: 1, wantStage: "target-validation", wantReason: "chain ID mismatch",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			config, server := runnableConfig(t, directory)
			defer server.Close()
			test.mutate(config)

			var stderr strings.Builder
			r := newRunner(config, io.Discard, &stderr)
			if status := r.run(); status != test.wantStatus {
				t.Fatalf("runner exited %d, want %d: %s", status, test.wantStatus, stderr.String())
			}
			data, err := os.ReadFile(filepath.Join(config.resultsDir, "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			var result manifest
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "failed" ||
				result.Failure == nil ||
				result.Failure.Stage != test.wantStage ||
				!strings.Contains(result.Failure.Reason, test.wantReason) {
				t.Fatalf("unexpected failure manifest: %#v", result.Failure)
			}
			if test.name == "configuration" && result.Scenarios.Single.Iterations != nil {
				t.Fatal("invalid iteration count should be null")
			}
		})
	}
}

func runnableConfig(t *testing.T, directory string) (*config, *httptest.Server) {
	t.Helper()
	const commit = "0123456789abcdef0123456789abcdef01234567"
	handler := func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodGet {
			_, _ = io.WriteString(writer, `{"ready":true}`)
			return
		}
		defer request.Body.Close()
		var payload struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		var result any
		switch payload.Method {
		case "juno_version":
			result = "sha-" + commit
		case "starknet_chainId":
			result = "0x1"
		case "starknet_blockNumber":
			result = 800000
		default:
			t.Errorf("unexpected RPC method %q", payload.Method)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}
	server := httptest.NewServer(http.HandlerFunc(handler))

	corpusPath := filepath.Join(directory, "corpus.json")
	corpus := []byte(`{"meta":{"method":"test"},"requests":[{"id":1}]}`)
	if err := os.WriteFile(corpusPath, corpus, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(corpus)
	checksum := hex.EncodeToString(digest[:]) + "  corpus.json\n"
	if err := os.WriteFile(corpusPath+".sha256", []byte(checksum), 0o644); err != nil {
		t.Fatal(err)
	}
	resultsDir := filepath.Join(directory, "results")
	if err := os.Mkdir(resultsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	unrelatedPath := filepath.Join(resultsDir, "unrelated.txt")
	if err := os.WriteFile(unrelatedPath, []byte("preserve\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	config := validConfig()
	config.nodeURL = server.URL
	config.readyURL = server.URL + "/ready"
	config.junoCommit = commit
	config.corpusPath = corpusPath
	config.resultsDir = resultsDir
	config.runIDFile = filepath.Join(directory, "run-id")
	return config, server
}

func validEnvironment() map[string]string {
	return map[string]string{
		"NODE_URL": "http://127.0.0.1:6060/v0_10", "READY_URL": "http://127.0.0.1:6060/ready/rpc",
		"EXPECTED_CHAIN_ID": "0x1", "EXPECTED_BLOCK_NUMBER": "800000", "SNAPSHOT_ID": "snapshot",
		"SNAPSHOT_SHA256":     strings.Repeat("a", 64),
		"JUNO_IMAGE_DIGEST":   "sha256:" + strings.Repeat("b", 64),
		"RUNNER_IMAGE_DIGEST": "sha256:" + strings.Repeat("c", 64),
		"READY_TIMEOUT":       "10s", "READY_POLL_INTERVAL": "1s",
		"ITERATIONS": "2", "VUS": "1", "CONCURRENCY_DURATION": "1s", "THROUGHPUT_DURATION": "1s",
		"THROUGHPUT_VUS": "3", "RATES": " 10,20 ",
	}
}

func validConfig() *config {
	return &config{
		scriptDir: defaultScriptDir, nodeURL: "http://127.0.0.1:6060/v0_10",
		readyURL: "http://127.0.0.1:6060/ready/rpc", expectedChainID: "0x1",
		expectedBlockNumber: "800000", snapshotID: "snapshot", snapshotSHA256: strings.Repeat("a", 64),
		junoImageDigest:   "sha256:" + strings.Repeat("b", 64),
		runnerImageDigest: "sha256:" + strings.Repeat("c", 64),
		junoCommit:        "0123456789abcdef0123456789abcdef01234567", runID: "test-run",
		resultsDir: "/results", corpusPath: "/corpus.json",
		readyTimeoutRaw: "10s", readyPollIntervalRaw: "1s", iterationsRaw: "2", vusRaw: "1",
		concurrencyDurationRaw: "1s", throughputDurationRaw: "1s",
		throughputVUsRaw: "3", ratesRaw: "10,20",
	}
}
