package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	resultDirectoryMode = os.FileMode(0o755)
	readyRequestTimeout = 5 * time.Second
	rpcRequestTimeout   = 10 * time.Second

	statusPending = "pending"
	statusRunning = "running"
	statusPassed  = "passed"
	statusFailed  = "failed"

	stageConfiguration = "configuration"
	stageSingle        = "single"
	stageConcurrency   = "concurrency"
	stageThroughput    = "throughput"

	k6QuietFlag = "--quiet"
)

type runner struct {
	config *config
	stdout io.Writer
	stderr io.Writer
	client *http.Client

	startedAt  time.Time
	finishedAt time.Time

	runStatus     string
	currentStage  string
	failureReason string
	failExitCode  int

	readyStatus       string
	warmupStatus      string
	singleStatus      string
	concurrencyStatus string
	throughputStatus  string

	actualChainID     string
	actualBlockNumber string
	actualJunoVersion string
	actualCorpusSHA   string
	corpusMeta        json.RawMessage

	warmupMetrics      *summaryMetrics
	singleMetrics      *summaryMetrics
	concurrencyMetrics *summaryMetrics
	throughputMetrics  *summaryMetrics

	context context.Context
	cancel  context.CancelFunc
	signals chan os.Signal
	mu      sync.Mutex
	signal  os.Signal
	active  *os.Process
}

func newRunner(config *config, stdout, stderr io.Writer) *runner {
	ctx, cancel := context.WithCancel(context.Background())
	transport := http.DefaultTransport.(*http.Transport).Clone()
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &runner{
		config: config, stdout: stdout, stderr: stderr, client: client,
		startedAt: nowUTC(), runStatus: statusRunning, currentStage: "preflight",
		failExitCode: 2, readyStatus: statusPending, warmupStatus: statusPending,
		singleStatus: statusPending, concurrencyStatus: statusPending, throughputStatus: statusPending,
		context: ctx, cancel: cancel, signals: make(chan os.Signal, 1),
	}
}

func (r *runner) run() int {
	if err := os.MkdirAll(r.config.resultsDir, resultDirectoryMode); err != nil {
		fmt.Fprintf(r.stderr, "create results directory: %v\n", err)
		return 2
	}
	if r.config.runIDFile != "" {
		if err := writeAtomic(r.config.runIDFile, []byte(r.config.runID+"\n")); err != nil {
			fmt.Fprintf(r.stderr, "write run ID: %v\n", err)
			return 2
		}
	}
	if err := r.cleanKnownOutputs(); err != nil {
		fmt.Fprintf(r.stderr, "clean result outputs: %v\n", err)
		return 2
	}

	r.startSignalHandler()
	defer r.stopSignalHandler()

	r.currentStage = "corpus-validation"
	if err := r.validateCorpus(); err != nil {
		return r.fail(err.Error())
	}

	r.currentStage = stageConfiguration
	if err := r.config.validate(); err != nil {
		return r.fail(err.Error())
	}
	if err := r.writeManifest(); err != nil {
		return r.fail(fmt.Sprintf("could not write manifest: %v", err))
	}

	r.failExitCode = 1
	if code := r.runReadiness(); code != 0 {
		return code
	}
	if code := r.validateTarget(); code != 0 {
		return code
	}
	if code := r.runWarmup(); code != 0 {
		return code
	}
	for _, scenario := range []string{stageSingle, stageConcurrency, stageThroughput} {
		if code := r.runScenario(scenario); code != 0 {
			return code
		}
	}

	if signal := r.receivedSignal(); signal != nil {
		return r.terminate(signal)
	}
	r.currentStage = "complete"
	r.runStatus = statusPassed
	r.finishedAt = nowUTC()
	if err := r.writeManifest(); err != nil {
		return r.fail(fmt.Sprintf("could not write manifest: %v", err))
	}
	fmt.Fprintf(r.stderr, "benchmark run %s completed successfully\n", r.config.runID)
	return 0
}

func (r *runner) startSignalHandler() {
	signal.Notify(r.signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		r.receiveSignal(<-r.signals)
	}()
}

func (r *runner) receiveSignal(received os.Signal) {
	r.mu.Lock()
	r.signal = received
	active := r.active
	r.mu.Unlock()
	r.cancel()
	if active != nil {
		_ = active.Signal(received)
	}
}

func (r *runner) stopSignalHandler() {
	signal.Stop(r.signals)
	r.cancel()
}

func (r *runner) receivedSignal() os.Signal {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.signal
}

func (r *runner) terminate(received os.Signal) int {
	name, code := "TERM", 143
	if received == os.Interrupt {
		name, code = "INT", 130
	}
	r.markCurrentStageFailed()
	r.runStatus = statusFailed
	r.failureReason = "runner terminated by " + name
	r.finishedAt = nowUTC()
	if err := r.writeManifest(); err != nil {
		fmt.Fprintf(r.stderr, "write termination manifest: %v\n", err)
	}
	fmt.Fprintf(r.stderr, "benchmark runner terminated by %s during %s\n", name, r.currentStage)
	return code
}

func (r *runner) fail(reason string) int {
	if received := r.receivedSignal(); received != nil {
		return r.terminate(received)
	}
	r.markCurrentStageFailed()
	r.failureReason = reason
	r.runStatus = statusFailed
	r.finishedAt = nowUTC()
	fmt.Fprintf(r.stderr, "benchmark runner failed during %s: %s\n", r.currentStage, reason)
	if err := r.writeManifest(); err != nil {
		fmt.Fprintf(r.stderr, "write failure manifest: %v\n", err)
	}
	return r.failExitCode
}

func (r *runner) markCurrentStageFailed() {
	switch r.currentStage {
	case "readiness":
		r.readyStatus = statusFailed
	case "warmup":
		r.warmupStatus = statusFailed
	case stageSingle:
		r.singleStatus = statusFailed
	case stageConcurrency:
		r.concurrencyStatus = statusFailed
	case stageThroughput:
		r.throughputStatus = statusFailed
	}
}

func (r *runner) cleanKnownOutputs() error {
	paths := []string{
		filepath.Join(r.config.resultsDir, "manifest.json"),
		filepath.Join(r.config.resultsDir, "single.json"),
		filepath.Join(r.config.resultsDir, "concurrency.json"),
		filepath.Join(r.config.resultsDir, "throughput.json"),
	}
	patterns := []string{
		".manifest.json.tmp.*",
		".rpc.json.tmp.*",
		".warmup.json.tmp.*",
		".single.json.tmp.*",
		".concurrency.json.tmp.*",
		".throughput.json.tmp.*",
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(r.config.resultsDir, pattern))
		if err != nil {
			return err
		}
		paths = append(paths, matches...)
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (r *runner) validateCorpus() error {
	checksumPath := r.config.corpusPath + ".sha256"
	if _, err := os.Stat(r.config.corpusPath); err != nil {
		return fmt.Errorf("corpus is not readable: %s", r.config.corpusPath)
	}
	checksumBytes, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("corpus checksum is not readable: %s", checksumPath)
	}
	fields := strings.Fields(string(checksumBytes))
	expected := ""
	if len(fields) > 0 {
		expected = fields[0]
	}
	corpus, err := os.Open(r.config.corpusPath)
	if err != nil {
		return fmt.Errorf("corpus is not readable: %s", r.config.corpusPath)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, corpus)
	closeErr := corpus.Close()
	if copyErr != nil || closeErr != nil {
		return fmt.Errorf("could not checksum corpus")
	}
	r.actualCorpusSHA = hex.EncodeToString(hash.Sum(nil))
	if expected == "" || expected != r.actualCorpusSHA {
		return fmt.Errorf("corpus checksum mismatch")
	}

	data, err := os.ReadFile(r.config.corpusPath)
	if err != nil {
		return fmt.Errorf("corpus metadata is invalid")
	}
	var document struct {
		Meta     json.RawMessage   `json:"meta"`
		Requests []json.RawMessage `json:"requests"`
	}
	if err := json.Unmarshal(data, &document); err != nil || !isJSONObject(document.Meta) {
		r.corpusMeta = json.RawMessage("null")
		return fmt.Errorf("corpus metadata is invalid")
	}
	r.corpusMeta = append(json.RawMessage(nil), document.Meta...)
	if len(document.Requests) == 0 {
		return fmt.Errorf("corpus requests are invalid or empty")
	}
	return nil
}

func isJSONObject(raw json.RawMessage) bool {
	var object map[string]json.RawMessage
	return len(raw) > 0 && json.Unmarshal(raw, &object) == nil && object != nil
}

func (r *runner) runReadiness() int {
	r.currentStage = "readiness"
	r.readyStatus = "waiting"
	if err := r.writeManifest(); err != nil {
		return r.fail(fmt.Sprintf("could not write manifest: %v", err))
	}
	started := time.Now()
	for {
		requestCtx, cancel := context.WithTimeout(r.context, readyRequestTimeout)
		request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, r.config.readyURL, http.NoBody)
		if err == nil {
			var response *http.Response
			response, err = r.client.Do(request)
			if response != nil {
				_, _ = io.Copy(io.Discard, response.Body)
				_ = response.Body.Close()
				if response.StatusCode >= http.StatusBadRequest {
					err = fmt.Errorf("HTTP status %d", response.StatusCode)
				}
			}
		}
		cancel()
		if received := r.receivedSignal(); received != nil {
			return r.terminate(received)
		}
		if err == nil {
			break
		}
		if time.Since(started) >= r.config.readyTimeout {
			return r.fail("RPC readiness timed out after " + r.config.readyTimeoutRaw)
		}
		fmt.Fprintf(r.stderr, "waiting for RPC readiness at %s\n", r.config.readyURL)
		if r.config.readyPollInterval <= 0 {
			return r.fail("READY_POLL_INTERVAL must be a positive duration using s, m, or h")
		}
		timer := time.NewTimer(r.config.readyPollInterval)
		select {
		case <-timer.C:
		case <-r.context.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return r.terminate(r.receivedSignal())
		}
	}
	r.readyStatus = statusPassed
	if err := r.writeManifest(); err != nil {
		return r.fail(fmt.Sprintf("could not write manifest: %v", err))
	}
	return 0
}

func (r *runner) rpcResult(method string) (json.RawMessage, error) {
	payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": []any{}, "id": 1})
	ctx, cancel := context.WithTimeout(r.context, rpcRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.config.nodeURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := r.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("HTTP status %d", response.StatusCode)
	}
	var result struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != nil || len(result.Result) == 0 {
		return nil, fmt.Errorf("JSON-RPC response has no result")
	}
	return result.Result, nil
}

func (r *runner) validateTarget() int {
	r.currentStage = "target-validation"
	versionResult, err := r.rpcResult("juno_version")
	if err != nil || json.Unmarshal(versionResult, &r.actualJunoVersion) != nil {
		return r.fail("could not read Juno version")
	}
	expectedVersion := "sha-" + r.config.junoCommit
	if r.actualJunoVersion != expectedVersion {
		return r.fail(fmt.Sprintf("Juno image mismatch: expected %s, got %s", expectedVersion, r.actualJunoVersion))
	}

	chainResult, err := r.rpcResult("starknet_chainId")
	if err != nil || json.Unmarshal(chainResult, &r.actualChainID) != nil {
		return r.fail("could not read chain ID")
	}
	if r.actualChainID != r.config.expectedChainID {
		return r.fail(fmt.Sprintf("chain ID mismatch: expected %s, got %s", r.config.expectedChainID, r.actualChainID))
	}

	blockResult, err := r.rpcResult("starknet_blockNumber")
	if err != nil {
		return r.fail("could not read block number")
	}
	r.actualBlockNumber = string(bytes.TrimSpace(blockResult))
	if !digitsPattern.MatchString(r.actualBlockNumber) {
		return r.fail("node returned an invalid block number: " + r.actualBlockNumber)
	}
	if r.actualBlockNumber != r.config.expectedBlockNumber {
		return r.fail(fmt.Sprintf(
			"block number mismatch: expected %s, got %s",
			r.config.expectedBlockNumber,
			r.actualBlockNumber,
		))
	}
	if err := r.writeManifest(); err != nil {
		return r.fail(fmt.Sprintf("could not write manifest: %v", err))
	}
	return 0
}

func (r *runner) runWarmup() int {
	r.currentStage = "warmup"
	r.warmupStatus = statusRunning
	if err := r.writeManifest(); err != nil {
		return r.fail(fmt.Sprintf("could not write manifest: %v", err))
	}
	temporary := filepath.Join(r.config.resultsDir, fmt.Sprintf(".warmup.json.tmp.%d", os.Getpid()))
	args := []string{
		"run", k6QuietFlag,
		"-e", "NODE_URL=" + r.config.nodeURL,
		"--vus", "1",
		"--iterations", strconv.FormatUint(defaultWarmupIterations, 10),
		"--summary-export", temporary,
		filepath.Join(r.config.scriptDir, "run.js"),
	}
	exitCode, commandErr := r.runK6(args, io.Discard)
	metrics, metricsErr := parseSummaryMetrics(temporary)
	_ = os.Remove(temporary)
	if metricsErr != nil {
		r.warmupMetrics = nil
		return r.fail("warmup did not produce a valid summary")
	}
	r.warmupMetrics = metrics
	if received := r.receivedSignal(); received != nil {
		return r.terminate(received)
	}
	if commandErr != nil || exitCode != 0 ||
		metrics.FailedChecks != 0 || metrics.RequestFailures != 0 || metrics.VUFailures != 0 {
		return r.fail("warmup recorded check, request, or VU failures")
	}
	r.warmupStatus = statusPassed
	if err := r.writeManifest(); err != nil {
		return r.fail(fmt.Sprintf("could not write manifest: %v", err))
	}
	return 0
}

func (r *runner) runScenario(name string) int {
	r.currentStage = name
	r.setScenarioStatus(name, statusRunning)
	if err := r.writeManifest(); err != nil {
		return r.fail(fmt.Sprintf("could not write manifest: %v", err))
	}
	temporary := filepath.Join(r.config.resultsDir, fmt.Sprintf(".%s.json.tmp.%d", name, os.Getpid()))
	result := filepath.Join(r.config.resultsDir, name+".json")
	args, err := r.scenarioArgs(name, temporary)
	if err != nil {
		return r.fail(err.Error())
	}

	exitCode, commandErr := r.runK6(args, r.stdout)
	metrics, err := promoteScenarioSummary(temporary, result)
	if err != nil {
		return r.fail(name + " did not produce a valid summary")
	}
	r.setScenarioMetrics(name, metrics)
	if received := r.receivedSignal(); received != nil {
		return r.terminate(received)
	}
	if commandErr != nil || exitCode != 0 {
		return r.fail(fmt.Sprintf("%s exited with status %d", name, exitCode))
	}
	if metrics.FailedChecks != 0 || metrics.RequestFailures != 0 || metrics.VUFailures != 0 {
		return r.fail(name + " recorded check, request, or VU failures")
	}
	if metrics.DroppedIterations != 0 {
		fmt.Fprintf(
			r.stderr,
			"%s dropped %s scheduled iterations: offered load exceeded the worker pool\n",
			name,
			formatMetric(metrics.DroppedIterations),
		)
	}
	r.setScenarioStatus(name, statusPassed)
	if err := r.writeManifest(); err != nil {
		return r.fail(fmt.Sprintf("could not write manifest: %v", err))
	}
	return 0
}

func (r *runner) scenarioArgs(name, summaryPath string) ([]string, error) {
	args := []string{"run", k6QuietFlag, "-e", "NODE_URL=" + r.config.nodeURL}
	switch name {
	case stageSingle:
		args = append(args,
			"--vus", "1",
			"--iterations", r.config.iterationsRaw,
		)
	case stageConcurrency:
		args = append(args,
			"--vus", r.config.vusRaw,
			"--duration", r.config.concurrencyDurationRaw,
		)
	case stageThroughput:
		args = append(args,
			"-e", "RATES="+r.config.normalizedRates,
			"-e", "DURATION="+r.config.throughputDurationRaw,
			"-e", "THROUGHPUT_VUS="+r.config.throughputVUsRaw,
		)
	default:
		return nil, fmt.Errorf("unknown scenario: %s", name)
	}
	args = append(args,
		"--summary-export", summaryPath,
		"--summary-trend-stats", "avg,min,med,p(90),p(99),max",
	)
	if name == stageThroughput {
		return append(args, filepath.Join(r.config.scriptDir, "throughput.js")), nil
	}
	return append(args, filepath.Join(r.config.scriptDir, "run.js")), nil
}

func promoteScenarioSummary(temporary, result string) (*summaryMetrics, error) {
	metrics, err := parseSummaryMetrics(temporary)
	if err != nil {
		if info, statErr := os.Stat(temporary); statErr == nil && info.Size() > 0 {
			_ = os.Rename(temporary, result)
		} else {
			_ = os.Remove(temporary)
		}
		return nil, err
	}
	if err := os.Rename(temporary, result); err != nil {
		return nil, err
	}
	return metrics, nil
}

func (r *runner) runK6(args []string, stdout io.Writer) (int, error) {
	input, err := os.Open(r.config.corpusPath)
	if err != nil {
		return -1, err
	}
	defer input.Close()
	command := exec.CommandContext(context.WithoutCancel(r.context), "k6", args...)
	command.Stdin = input
	command.Stdout = stdout
	command.Stderr = r.stderr

	r.mu.Lock()
	if r.signal != nil {
		r.mu.Unlock()
		return -1, context.Canceled
	}
	if err := command.Start(); err != nil {
		r.mu.Unlock()
		return -1, err
	}
	r.active = command.Process
	r.mu.Unlock()

	err = command.Wait()
	r.mu.Lock()
	r.active = nil
	r.mu.Unlock()
	if err == nil {
		return 0, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode(), err
	}
	return -1, err
}

func (r *runner) setScenarioStatus(name, status string) {
	switch name {
	case stageSingle:
		r.singleStatus = status
	case stageConcurrency:
		r.concurrencyStatus = status
	case stageThroughput:
		r.throughputStatus = status
	}
}

func (r *runner) setScenarioMetrics(name string, metrics *summaryMetrics) {
	switch name {
	case stageSingle:
		r.singleMetrics = metrics
	case stageConcurrency:
		r.concurrencyMetrics = metrics
	case stageThroughput:
		r.throughputMetrics = metrics
	}
}

func formatMetric(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
