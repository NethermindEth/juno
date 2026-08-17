package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const resultFileMode os.FileMode = 0o644

type failureManifest struct {
	Stage  string `json:"stage"`
	Reason string `json:"reason"`
}

type nodeManifest struct {
	URL                 string  `json:"url"`
	Readiness           string  `json:"readiness"`
	ExpectedChainID     string  `json:"expectedChainId"`
	ChainID             *string `json:"chainId"`
	ExpectedBlockNumber *uint64 `json:"expectedBlockNumber"`
	BlockNumber         *uint64 `json:"blockNumber"`
}

type snapshotManifest struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

type corpusManifest struct {
	SHA256 *string         `json:"sha256"`
	Meta   json.RawMessage `json:"meta"`
}

type junoManifest struct {
	Commit      string  `json:"commit"`
	Version     *string `json:"version"`
	ImageDigest string  `json:"imageDigest"`
}

type imageManifest struct {
	ImageDigest string `json:"imageDigest"`
}

type summaryMetrics struct {
	FailedChecks        float64 `json:"failedChecks"`
	RequestFailures     float64 `json:"requestFailures"`
	HTTPRequestFailures float64 `json:"httpRequestFailures"`
	VUFailures          float64 `json:"vuFailures"`
	DroppedIterations   float64 `json:"droppedIterations"`
	CompletedIterations float64 `json:"completedIterations"`
}

type warmupManifest struct {
	Status     string          `json:"status"`
	Measured   bool            `json:"measured"`
	Iterations uint64          `json:"iterations"`
	Metrics    *summaryMetrics `json:"metrics"`
}

type singleManifest struct {
	Status     string          `json:"status"`
	Measured   bool            `json:"measured"`
	Iterations *uint64         `json:"iterations"`
	Metrics    *summaryMetrics `json:"metrics"`
	Result     string          `json:"result"`
}

type concurrencyManifest struct {
	Status   string          `json:"status"`
	Measured bool            `json:"measured"`
	VUs      *uint64         `json:"vus"`
	Duration string          `json:"duration"`
	Metrics  *summaryMetrics `json:"metrics"`
	Result   string          `json:"result"`
}

type throughputManifest struct {
	Status          string          `json:"status"`
	Measured        bool            `json:"measured"`
	PreAllocatedVUs *uint64         `json:"preAllocatedVUs"`
	MaxVUs          *uint64         `json:"maxVUs"`
	Rates           []uint64        `json:"rates"`
	Duration        string          `json:"duration"`
	Metrics         *summaryMetrics `json:"metrics"`
	Result          string          `json:"result"`
}

type scenariosManifest struct {
	Warmup      warmupManifest      `json:"warmup"`
	Single      singleManifest      `json:"single"`
	Concurrency concurrencyManifest `json:"concurrency"`
	Throughput  throughputManifest  `json:"throughput"`
}

type manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	RunID         string            `json:"runId"`
	Status        string            `json:"status"`
	StartedAt     string            `json:"startedAt"`
	FinishedAt    *string           `json:"finishedAt"`
	Failure       *failureManifest  `json:"failure"`
	Node          nodeManifest      `json:"node"`
	Snapshot      snapshotManifest  `json:"snapshot"`
	Corpus        corpusManifest    `json:"corpus"`
	Juno          junoManifest      `json:"juno"`
	Runner        imageManifest     `json:"runner"`
	Scenarios     scenariosManifest `json:"scenarios"`
}

func (r *runner) currentManifest() manifest {
	warmup := r.scenarios["warmup"]
	single := r.scenarios[stageSingle]
	concurrency := r.scenarios[stageConcurrency]
	throughput := r.scenarios[stageThroughput]
	var finishedAt *string
	if !r.finishedAt.IsZero() {
		formatted := r.finishedAt.UTC().Format(time.RFC3339)
		finishedAt = &formatted
	}

	var failure *failureManifest
	if r.failureReason != "" {
		failure = &failureManifest{Stage: r.currentStage, Reason: r.failureReason}
	}

	var chainID, version, corpusSHA *string
	if r.actualChainID != "" {
		value := r.actualChainID
		chainID = &value
	}
	if r.actualJunoVersion != "" {
		value := r.actualJunoVersion
		version = &value
	}
	if r.actualCorpusSHA != "" {
		value := r.actualCorpusSHA
		corpusSHA = &value
	}

	var expectedBlock, actualBlock, iterations, vus, throughputVUs *uint64
	if r.config.expectedBlockParsed {
		value := r.config.expectedBlock
		expectedBlock = &value
	}
	if digitsPattern.MatchString(r.actualBlockNumber) {
		if value, err := strconv.ParseUint(r.actualBlockNumber, 10, 64); err == nil {
			actualBlock = &value
		}
	}
	if value, err := strconv.ParseUint(r.config.iterationsRaw, 10, 64); err == nil {
		iterations = &value
	}
	if value, err := strconv.ParseUint(r.config.vusRaw, 10, 64); err == nil {
		vus = &value
	}
	if value, err := strconv.ParseUint(r.config.throughputVUsRaw, 10, 64); err == nil {
		throughputVUs = &value
	}

	meta := r.corpusMeta
	if len(meta) == 0 {
		meta = json.RawMessage("null")
	}
	rates := r.config.rates
	if rates == nil {
		rates = []uint64{}
	}

	return manifest{
		SchemaVersion: 2,
		RunID:         r.config.runID,
		Status:        r.runStatus,
		StartedAt:     r.startedAt.UTC().Format(time.RFC3339),
		FinishedAt:    finishedAt,
		Failure:       failure,
		Node: nodeManifest{
			URL:                 r.config.nodeURL,
			Readiness:           r.readyStatus,
			ExpectedChainID:     r.config.expectedChainID,
			ChainID:             chainID,
			ExpectedBlockNumber: expectedBlock,
			BlockNumber:         actualBlock,
		},
		Snapshot: snapshotManifest{ID: r.config.snapshotID, SHA256: r.config.snapshotSHA256},
		Corpus:   corpusManifest{SHA256: corpusSHA, Meta: meta},
		Juno: junoManifest{
			Commit:      r.config.junoCommit,
			Version:     version,
			ImageDigest: r.config.junoImageDigest,
		},
		Runner: imageManifest{ImageDigest: r.config.runnerImageDigest},
		Scenarios: scenariosManifest{
			Warmup: warmupManifest{
				Status: warmup.status, Measured: false,
				Iterations: defaultWarmupIterations, Metrics: warmup.metrics,
			},
			Single: singleManifest{
				Status: single.status, Measured: true, Iterations: iterations,
				Metrics: single.metrics, Result: "single.json",
			},
			Concurrency: concurrencyManifest{
				Status: concurrency.status, Measured: true, VUs: vus,
				Duration: r.config.concurrencyDurationRaw,
				Metrics:  concurrency.metrics, Result: "concurrency.json",
			},
			Throughput: throughputManifest{
				Status: throughput.status, Measured: true,
				PreAllocatedVUs: throughputVUs, MaxVUs: throughputVUs,
				Rates: rates, Duration: r.config.throughputDurationRaw,
				Metrics: throughput.metrics, Result: "throughput.json",
			},
		},
	}
}

func (r *runner) writeManifest() error {
	data, err := json.MarshalIndent(r.currentManifest(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')
	return writeFileViaRename(filepath.Join(r.config.resultsDir, "manifest.json"), data)
}

func writeFileViaRename(path string, data []byte) error {
	temporary := temporaryFilePath(path, os.Getpid())
	if err := os.WriteFile(temporary, data, resultFileMode); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func temporaryFilePath(path string, pid int) string {
	return filepath.Join(filepath.Dir(path), fmt.Sprintf(".%s.tmp.%d", filepath.Base(path), pid))
}

func temporaryFilePattern(path string) string {
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp.*")
}
