package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultScriptDir        = "/bench/rpc"
	defaultResultsDir       = "/results"
	defaultWarmupIterations = uint64(200)
	maximumRate             = uint64(2147483647)
)

var (
	commitPattern      = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digitsPattern      = regexp.MustCompile(`^\d+$`)
	sha256Pattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	imageDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type lookupEnv func(string) (string, bool)

type config struct {
	scriptDir string
	loadError string

	nodeURL             string
	readyURL            string
	expectedChainID     string
	expectedBlockNumber string
	snapshotID          string
	snapshotSHA256      string
	junoImageDigest     string
	runnerImageDigest   string

	junoCommit string
	runID      string
	runIDFile  string
	resultsDir string
	corpusPath string

	readyTimeoutRaw      string
	readyPollIntervalRaw string
	readyTimeout         time.Duration
	readyPollInterval    time.Duration

	iterationsRaw          string
	vusRaw                 string
	concurrencyDurationRaw string
	throughputDurationRaw  string
	throughputVUsRaw       string
	ratesRaw               string

	iterations          uint64
	vus                 uint64
	throughputVUs       uint64
	rates               []uint64
	normalizedRates     string
	expectedBlock       uint64
	expectedBlockParsed bool
}

func loadConfig(
	getenv lookupEnv,
	readFile func(string) ([]byte, error),
	now time.Time,
) *config {
	scriptDir := defaultScriptDir
	commitBytes, err := readFile(scriptDir + "/juno-commit")
	junoCommit := strings.TrimSpace(string(commitBytes))
	loadError := ""
	if err != nil {
		loadError = "embedded Juno commit is unavailable"
	} else if !commitPattern.MatchString(junoCommit) {
		loadError = "embedded Juno commit is invalid"
	}

	required := []string{
		"NODE_URL",
		"READY_URL",
		"EXPECTED_CHAIN_ID",
		"EXPECTED_BLOCK_NUMBER",
		"SNAPSHOT_ID",
		"SNAPSHOT_SHA256",
		"JUNO_IMAGE_DIGEST",
		"RUNNER_IMAGE_DIGEST",
	}
	values := make(map[string]string, len(required))
	for _, name := range required {
		value, _ := getenv(name)
		values[name] = value
	}

	valueOr := func(name, fallback string) string {
		if value, ok := getenv(name); ok && value != "" {
			return value
		}
		return fallback
	}

	commitID := "unknown"
	if commitPattern.MatchString(junoCommit) {
		commitID = junoCommit[:12]
	}
	runID := valueOr("RUN_ID", now.Format("20060102T150405Z")+"-"+commitID)
	runIDFile, _ := getenv("RUN_ID_FILE")

	return &config{
		scriptDir: scriptDir,
		loadError: loadError,

		nodeURL:             values["NODE_URL"],
		readyURL:            values["READY_URL"],
		expectedChainID:     values["EXPECTED_CHAIN_ID"],
		expectedBlockNumber: values["EXPECTED_BLOCK_NUMBER"],
		snapshotID:          values["SNAPSHOT_ID"],
		snapshotSHA256:      values["SNAPSHOT_SHA256"],
		junoImageDigest:     values["JUNO_IMAGE_DIGEST"],
		runnerImageDigest:   values["RUNNER_IMAGE_DIGEST"],

		junoCommit: junoCommit,
		runID:      runID,
		runIDFile:  runIDFile,
		resultsDir: valueOr("RESULTS_DIR", defaultResultsDir),
		corpusPath: valueOr("CORPUS_PATH", scriptDir+"/corpus/v0_10/getTransactionByHash.json"),

		readyTimeoutRaw:      valueOr("READY_TIMEOUT", "30m"),
		readyPollIntervalRaw: valueOr("READY_POLL_INTERVAL", "5s"),

		iterationsRaw:          valueOr("ITERATIONS", "200"),
		vusRaw:                 valueOr("VUS", "50"),
		concurrencyDurationRaw: valueOr("CONCURRENCY_DURATION", "30s"),
		throughputDurationRaw:  valueOr("THROUGHPUT_DURATION", "5s"),
		throughputVUsRaw:       valueOr("THROUGHPUT_VUS", "50"),
		ratesRaw:               valueOr("RATES", "1000,2000,3000"),
	}
}

func (c *config) validate() error {
	if err := c.validateProvenance(); err != nil {
		return err
	}

	rates, err := parseRates(c.ratesRaw)
	if err != nil {
		c.rates = []uint64{}
		return err
	}
	c.rates = rates
	rateStrings := make([]string, len(rates))
	for i, rate := range rates {
		rateStrings[i] = strconv.FormatUint(rate, 10)
	}
	c.normalizedRates = strings.Join(rateStrings, ",")

	if c.iterations, err = parsePositiveInteger("ITERATIONS", c.iterationsRaw); err != nil {
		return err
	}
	if c.vus, err = parsePositiveInteger("VUS", c.vusRaw); err != nil {
		return err
	}
	if c.throughputVUs, err = parsePositiveInteger("THROUGHPUT_VUS", c.throughputVUsRaw); err != nil {
		return err
	}
	if _, err := parseDuration("CONCURRENCY_DURATION", c.concurrencyDurationRaw); err != nil {
		return err
	}
	if _, err := parseDuration("THROUGHPUT_DURATION", c.throughputDurationRaw); err != nil {
		return err
	}

	if !digitsPattern.MatchString(c.expectedBlockNumber) {
		return fmt.Errorf("EXPECTED_BLOCK_NUMBER must be a non-negative integer")
	}
	c.expectedBlock, err = strconv.ParseUint(c.expectedBlockNumber, 10, 64)
	if err != nil {
		return fmt.Errorf("EXPECTED_BLOCK_NUMBER must be a non-negative integer")
	}
	c.expectedBlockParsed = true

	if c.readyTimeout, err = parseDuration("READY_TIMEOUT", c.readyTimeoutRaw); err != nil {
		return err
	}
	c.readyPollInterval, err = parseDuration("READY_POLL_INTERVAL", c.readyPollIntervalRaw)
	if err != nil {
		return err
	}
	return nil
}

func (c *config) validateProvenance() error {
	if c.loadError != "" {
		return fmt.Errorf("%s", c.loadError)
	}
	required := []struct{ name, value string }{
		{name: "NODE_URL", value: c.nodeURL},
		{name: "READY_URL", value: c.readyURL},
		{name: "EXPECTED_CHAIN_ID", value: c.expectedChainID},
		{name: "EXPECTED_BLOCK_NUMBER", value: c.expectedBlockNumber},
		{name: "SNAPSHOT_ID", value: c.snapshotID},
		{name: "SNAPSHOT_SHA256", value: c.snapshotSHA256},
		{name: "JUNO_IMAGE_DIGEST", value: c.junoImageDigest},
		{name: "RUNNER_IMAGE_DIGEST", value: c.runnerImageDigest},
	}
	for _, item := range required {
		if item.value == "" {
			return fmt.Errorf("%s is required", item.name)
		}
	}
	if !sha256Pattern.MatchString(c.snapshotSHA256) {
		return fmt.Errorf("SNAPSHOT_SHA256 must be a 64-character hex digest")
	}
	if !imageDigestPattern.MatchString(c.junoImageDigest) {
		return fmt.Errorf("JUNO_IMAGE_DIGEST must be sha256:<64 hex>")
	}
	if !imageDigestPattern.MatchString(c.runnerImageDigest) {
		return fmt.Errorf("RUNNER_IMAGE_DIGEST must be sha256:<64 hex>")
	}
	return nil
}

func parseDuration(name, raw string) (time.Duration, error) {
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return duration, nil
}

func parseRates(raw string) ([]uint64, error) {
	parts := strings.Split(raw, ",")
	if len(parts) == 0 {
		return nil, fmt.Errorf("RATES must be a comma-separated list of integers")
	}
	rates := make([]uint64, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if !digitsPattern.MatchString(part) {
			return nil, fmt.Errorf("RATES must be a comma-separated list of integers")
		}
		rate, err := strconv.ParseUint(part, 10, 32)
		if err != nil || rate == 0 || rate > maximumRate {
			return nil, fmt.Errorf("RATES must contain positive 32-bit integers")
		}
		rates[i] = rate
	}
	return rates, nil
}

func parsePositiveInteger(name, raw string) (uint64, error) {
	if !digitsPattern.MatchString(raw) {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
