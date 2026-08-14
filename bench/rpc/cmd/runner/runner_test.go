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
	"syscall"
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

func TestLoadFailureProducesManifest(t *testing.T) {
	environment := validEnvironment()
	delete(environment, "SNAPSHOT_ID")
	environment["RESULTS_DIR"] = t.TempDir()
	config := loadConfig(
		func(name string) (string, bool) {
			value, ok := environment[name]
			return value, ok
		},
		func(string) ([]byte, error) {
			return []byte("0123456789abcdef0123456789abcdef01234567\n"), nil
		},
		time.Now(),
	)
	r := newRunner(config, io.Discard, io.Discard)
	if status := r.run(); status != 2 {
		t.Fatalf("runner exited %d, want 2", status)
	}
	data, err := os.ReadFile(filepath.Join(config.resultsDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result manifest
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Failure == nil || result.Failure.Stage != stageConfiguration ||
		result.Failure.Reason != "SNAPSHOT_ID is required" {
		t.Fatalf("unexpected failure: %#v", result.Failure)
	}
}

func TestParseSummaryMetricsSupportsBothK6Shapes(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "summary.json")
	data := `{
  "metrics": {
    "checks": {"fails": 2},
    "rpc_request_failures": {"values": {"count": 3}},
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
		FailedChecks: 2, RequestFailures: 3, HTTPRequestFailures: 4,
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

func TestRPCResult(t *testing.T) {
	t.Parallel()
	handler := func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"jsonrpc":"2.0","id":1,"result":"`+payload.Method+`"}`)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	config := validConfig()
	config.nodeURL = server.URL
	r := newRunner(config, io.Discard, io.Discard)
	result, err := r.rpcResult("juno_version")
	if err != nil {
		t.Fatalf("RPC result: %v", err)
	}
	if string(result) != `"juno_version"` {
		t.Fatalf("unexpected result %s", result)
	}
}

func TestManifestUsesNullForInvalidConfiguration(t *testing.T) {
	t.Parallel()
	config := validConfig()
	config.iterationsRaw = "invalid"
	r := newRunner(config, io.Discard, io.Discard)
	r.runStatus = "failed"
	r.currentStage = "configuration"
	r.failureReason = "ITERATIONS must be a positive integer"
	data, err := json.Marshal(r.currentManifest())
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Scenarios struct {
			Single struct {
				Iterations *uint64 `json:"iterations"`
			} `json:"single"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Scenarios.Single.Iterations != nil {
		t.Fatalf("invalid iteration count should be null")
	}
}

func TestAtomicTemporaryPathMatchesCleanupPattern(t *testing.T) {
	t.Parallel()
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	temporaryPath := atomicTemporaryPath(manifestPath, 1234)
	matched, err := filepath.Match(atomicTemporaryPattern(manifestPath), temporaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Fatalf("temporary path %q does not match cleanup pattern", temporaryPath)
	}
	if filepath.Base(temporaryPath) != ".manifest.json.tmp.1234" {
		t.Fatalf("unexpected temporary filename %q", filepath.Base(temporaryPath))
	}
}

func TestRunK6ReturnsExitStatusAndForwardsTerm(t *testing.T) {
	directory := t.TempDir()
	corpusPath := filepath.Join(directory, "corpus.json")
	readyPath := filepath.Join(directory, "ready")
	signalPath := filepath.Join(directory, "signal")
	if err := os.WriteFile(corpusPath, []byte(`{"meta":{},"requests":[{}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	k6Path := filepath.Join(directory, "k6")
	k6 := `#!/bin/sh
count=0
on_term() {
  count=$((count + 1))
  [ "$count" -lt 2 ] || exit 42
  : > "$K6_SIGNAL_FILE"
}
trap on_term TERM
: > "$K6_READY_FILE"
while :; do sleep 1; done
`
	if err := os.WriteFile(k6Path, []byte(k6), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("K6_READY_FILE", readyPath)
	t.Setenv("K6_SIGNAL_FILE", signalPath)

	config := validConfig()
	config.corpusPath = corpusPath
	r := newRunner(config, io.Discard, io.Discard)
	r.startSignalHandler()
	defer r.stopSignalHandler()
	result := make(chan int, 1)
	go func() {
		status, _ := r.runK6([]string{"run"}, io.Discard)
		result <- status
	}()

	waitForFile(t, readyPath)
	r.signals <- syscall.SIGTERM
	waitForFile(t, signalPath)
	r.signals <- syscall.SIGTERM
	select {
	case status := <-result:
		if status != 42 {
			t.Fatalf("got exit status %d, want 42", status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("fake k6 did not stop after TERM")
	}
}

func TestRunnerCompletesAllScenarios(t *testing.T) {
	directory := t.TempDir()
	installFakeK6(t, directory)
	config, server := runnableConfig(t, directory)
	defer server.Close()
	config.expectedBlockNumber = "0800000"

	var stderr strings.Builder
	r := newRunner(config, io.Discard, &stderr)
	if status := r.run(); status != 0 {
		t.Fatalf("runner exited %d: %s", status, stderr.String())
	}

	manifestData, err := os.ReadFile(filepath.Join(config.resultsDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result manifest
	if err := json.Unmarshal(manifestData, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "passed" || result.Node.Readiness != "passed" {
		t.Fatalf("unexpected manifest status: %#v", result)
	}
	if result.Scenarios.Single.Status != "passed" ||
		result.Scenarios.Concurrency.Status != "passed" ||
		result.Scenarios.Throughput.Status != "passed" {
		t.Fatalf("scenarios did not pass: %#v", result.Scenarios)
	}
	for _, name := range []string{"single.json", "concurrency.json", "throughput.json"} {
		if _, err := os.Stat(filepath.Join(config.resultsDir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	if !strings.Contains(stderr.String(), "completed successfully") {
		t.Fatalf("missing completion log: %s", stderr.String())
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
			installFakeK6(t, directory)
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

func installFakeK6(t *testing.T, directory string) {
	t.Helper()
	k6 := `#!/bin/sh
summary=
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--summary-export" ]; then
    shift
    summary=$1
  fi
  shift
done
summary_json='{"metrics":{'\
'"checks":{"fails":0},'\
'"rpc_request_failures":{"count":0},'\
'"http_req_failed":{"passes":0},'\
'"vu_failures":{"count":0},'\
'"dropped_iterations":{"count":0},'\
'"iterations":{"count":2}}}'
printf '%s\n' "$summary_json" > "$summary"
`
	if err := os.WriteFile(filepath.Join(directory, "k6"), []byte(k6), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
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

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
