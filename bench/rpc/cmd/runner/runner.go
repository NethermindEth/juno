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
	maxCapturedStderr   = 4 * 1024

	statusPending = "pending"
	statusWaiting = "waiting"
	statusRunning = "running"
	statusPassed  = "passed"
	statusFailed  = "failed"

	stageConfiguration = "configuration"
	stageSingle        = "single"
	stageConcurrency   = "concurrency"
	stageThroughput    = "throughput"

	k6QuietFlag         = "--quiet"
	k6SummaryExportFlag = "--summary-export"
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

	readyStatus string
	scenarios   map[string]*scenarioState

	actualChainID     string
	actualBlockNumber string
	actualJunoVersion string
	actualCorpusSHA   string
	corpusMeta        json.RawMessage

	context context.Context
	cancel  context.CancelFunc
	signals chan os.Signal
	mu      sync.Mutex
	signal  os.Signal
	active  *os.Process
}

type scenarioState struct {
	status  string
	metrics *summaryMetrics
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
		failExitCode: 2, readyStatus: statusPending,
		scenarios: map[string]*scenarioState{
			"warmup":         {status: statusPending},
			stageSingle:      {status: statusPending},
			stageConcurrency: {status: statusPending},
			stageThroughput:  {status: statusPending},
		},
		context: ctx, cancel: cancel, signals: make(chan os.Signal, 1),
	}
}

func (r *runner) run() int {
	if err := os.MkdirAll(r.config.resultsDir, resultDirectoryMode); err != nil {
		fmt.Fprintf(r.stderr, "create results directory: %v\n", err)
		return 2
	}
	if r.config.runIDFile != "" {
		if err := writeFileViaRename(r.config.runIDFile, []byte(r.config.runID+"\n")); err != nil {
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
	code := r.execute()
	if err := r.writeManifest(); err != nil {
		fmt.Fprintf(r.stderr, "write terminal manifest: %v\n", err)
		if code == 0 {
			return 1
		}
	}
	return code
}

func (r *runner) execute() int {
	r.currentStage = stageConfiguration
	if err := r.config.validate(); err != nil {
		return r.fail(err.Error())
	}

	r.currentStage = "corpus-validation"
	if err := r.validateCorpus(); err != nil {
		return r.fail(err.Error())
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
	fmt.Fprintf(r.stderr, "benchmark run %s completed successfully\n", r.config.runID)
	return 0
}

func (r *runner) startSignalHandler() {
	signal.Notify(r.signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		for received := range r.signals {
			r.receiveSignal(received)
		}
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
	close(r.signals)
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
	return r.failExitCode
}

func (r *runner) markCurrentStageFailed() {
	switch r.currentStage {
	case "readiness":
		r.readyStatus = statusFailed
	default:
		if scenario := r.scenarios[r.currentStage]; scenario != nil {
			scenario.status = statusFailed
		}
	}
}

func (r *runner) cleanKnownOutputs() error {
	outputs := []string{"manifest", "warmup", stageSingle, stageConcurrency, stageThroughput}
	for _, name := range outputs {
		if name != "warmup" {
			path := filepath.Join(r.config.resultsDir, name+".json")
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		matches, err := filepath.Glob(filepath.Join(r.config.resultsDir, "."+name+".json.tmp.*"))
		if err != nil {
			return err
		}
		for _, path := range matches {
			if err := os.Remove(path); err != nil {
				return err
			}
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
	r.readyStatus = statusWaiting
	started := time.Now()
	for {
		requestCtx, cancel := context.WithTimeout(r.context, readyRequestTimeout)
		request, err := http.NewRequestWithContext(
			requestCtx,
			http.MethodGet,
			r.config.readyURL,
			http.NoBody,
		)
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
		select {
		case <-time.After(r.config.readyPollInterval):
		case <-r.context.Done():
			return r.terminate(r.receivedSignal())
		}
	}
	r.readyStatus = statusPassed
	return 0
}

func (r *runner) rpcResult(method string) (json.RawMessage, error) {
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  []any{},
		"id":      1,
	})
	ctx, cancel := context.WithTimeout(r.context, rpcRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		r.config.nodeURL,
		bytes.NewReader(payload),
	)
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
	if result.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: %s", result.Error.Message)
	}
	if len(result.Result) == 0 {
		return nil, fmt.Errorf("JSON-RPC response has no result")
	}
	return result.Result, nil
}

func (r *runner) validateTarget() int {
	r.currentStage = "target-validation"
	versionResult, err := r.rpcResult("juno_version")
	if err != nil {
		return r.fail(fmt.Sprintf("could not read Juno version: %v", err))
	}
	if err := json.Unmarshal(versionResult, &r.actualJunoVersion); err != nil {
		return r.fail(fmt.Sprintf("could not decode Juno version: %v", err))
	}
	expectedVersion := "sha-" + r.config.junoCommit
	if r.actualJunoVersion != expectedVersion {
		return r.fail(fmt.Sprintf(
			"Juno image mismatch: expected %s, got %s",
			expectedVersion,
			r.actualJunoVersion,
		))
	}

	chainResult, err := r.rpcResult("starknet_chainId")
	if err != nil {
		return r.fail(fmt.Sprintf("could not read chain ID: %v", err))
	}
	if err := json.Unmarshal(chainResult, &r.actualChainID); err != nil {
		return r.fail(fmt.Sprintf("could not decode chain ID: %v", err))
	}
	if r.actualChainID != r.config.expectedChainID {
		return r.fail(fmt.Sprintf(
			"chain ID mismatch: expected %s, got %s",
			r.config.expectedChainID,
			r.actualChainID,
		))
	}

	blockResult, err := r.rpcResult("starknet_blockNumber")
	if err != nil {
		return r.fail(fmt.Sprintf("could not read block number: %v", err))
	}
	r.actualBlockNumber = string(bytes.TrimSpace(blockResult))
	actualBlock, err := strconv.ParseUint(r.actualBlockNumber, 10, 64)
	if err != nil {
		return r.fail("node returned an invalid block number: " + r.actualBlockNumber)
	}
	if actualBlock != r.config.expectedBlock {
		return r.fail(fmt.Sprintf(
			"block number mismatch: expected %s, got %s",
			r.config.expectedBlockNumber,
			r.actualBlockNumber,
		))
	}
	return 0
}

func (r *runner) runWarmup() int {
	temporary := filepath.Join(r.config.resultsDir, fmt.Sprintf(".warmup.json.tmp.%d", os.Getpid()))
	args := []string{
		"run", k6QuietFlag,
		"-e", "NODE_URL=" + r.config.nodeURL,
		"--vus", "1",
		"--iterations", strconv.FormatUint(defaultWarmupIterations, 10),
		k6SummaryExportFlag, temporary,
		filepath.Join(r.config.scriptDir, "run.js"),
	}
	return r.executeScenario("warmup", temporary, "", args, io.Discard)
}

func (r *runner) runScenario(name string) int {
	r.currentStage = name
	temporary := filepath.Join(r.config.resultsDir, fmt.Sprintf(".%s.json.tmp.%d", name, os.Getpid()))
	result := filepath.Join(r.config.resultsDir, name+".json")
	args, err := r.scenarioArgs(name, temporary)
	if err != nil {
		return r.fail(err.Error())
	}
	return r.executeScenario(name, temporary, result, args, r.stdout)
}

func (r *runner) executeScenario(
	name, temporary, result string,
	args []string,
	stdout io.Writer,
) int {
	r.currentStage = name
	scenario := r.scenarios[name]
	scenario.status = statusRunning
	exitCode, commandErr := r.runK6(args, stdout)
	var metrics *summaryMetrics
	var err error
	if result == "" {
		metrics, err = parseSummaryMetrics(temporary)
		_ = os.Remove(temporary)
	} else {
		metrics, err = promoteScenarioSummary(temporary, result)
	}
	if err != nil {
		reason := fmt.Sprintf("%s did not produce a valid summary: %v", name, err)
		return r.fail(commandReason(reason, commandErr))
	}
	scenario.metrics = metrics
	if received := r.receivedSignal(); received != nil {
		return r.terminate(received)
	}
	if commandErr != nil || exitCode != 0 {
		return r.fail(commandReason(fmt.Sprintf("%s exited with status %d", name, exitCode), commandErr))
	}
	if metrics.FailedChecks != 0 || metrics.RequestFailures != 0 ||
		metrics.HTTPRequestFailures != 0 || metrics.VUFailures != 0 {
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
	scenario.status = statusPassed
	return 0
}

func commandReason(reason string, err error) string {
	if err != nil {
		return fmt.Sprintf("%s: %v", reason, err)
	}
	return reason
}

type tailWriter struct {
	data []byte
}

func (w *tailWriter) Write(p []byte) (int, error) {
	written := len(p)
	if written >= maxCapturedStderr {
		w.data = append(w.data[:0], p[written-maxCapturedStderr:]...)
		return written, nil
	}
	if overflow := len(w.data) + written - maxCapturedStderr; overflow > 0 {
		copy(w.data, w.data[overflow:])
		w.data = w.data[:len(w.data)-overflow]
	}
	w.data = append(w.data, p...)
	return written, nil
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
		k6SummaryExportFlag, summaryPath,
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
		_ = os.Remove(temporary)
		return nil, err
	}
	if err := os.Rename(temporary, result); err != nil {
		_ = os.Remove(temporary)
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
	var commandStderr tailWriter
	command.Stderr = io.MultiWriter(r.stderr, &commandStderr)

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
	if detail := strings.TrimSpace(string(commandStderr.data)); detail != "" {
		err = fmt.Errorf("%w: %s", err, detail)
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode(), err
	}
	return -1, err
}

func formatMetric(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
