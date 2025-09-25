package performance

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type PerformanceReporter struct {
	mu             sync.Mutex
	file           *os.File
	writer         *csv.Writer
	blockNumber    uint64
	lastReported   uint64
	reportInterval uint64
	metrics        map[string]int64
	useNewState    bool
}

func NewPerformanceReporter(useNewState bool, reportInterval uint64) (*PerformanceReporter, error) {
	if err := os.MkdirAll("reports", 0755); err != nil {
		return nil, err
	}

	filename := "old_state_performance.csv"
	if useNewState {
		filename = "new_state_performance.csv"
	}

	filePath := filepath.Join("reports", filename)

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	writer := csv.NewWriter(file)
	reporter := &PerformanceReporter{
		file:           file,
		writer:         writer,
		reportInterval: reportInterval,
		metrics:        make(map[string]int64),
		useNewState:    useNewState,
	}

	// Always ensure headers are written
	if err := reporter.ensureHeaders(); err != nil {
		file.Close()
		return nil, err
	}

	// Add initial row at block 1 with zeros
	if err := reporter.addInitialRow(); err != nil {
		file.Close()
		return nil, err
	}

	return reporter, nil
}

func (pr *PerformanceReporter) writeHeaders() error {
	var headers []string

	if pr.useNewState {
		headers = []string{
			"BlockNumber",
			"Timestamp",
			"allVerifyCommTime",
			"allRegisterClassesTime",
			"allUpdateClassTrieTime",
			"allRegisterDeployedContractsTime",
			"allUpdateContractsTime",
			"allStateCommitTime",
			"allStateCommitStateObjectCommitTime",
			"allStateCommitContractTrieUpdateTime",
			"allStateCommitClassesTrieCommitTime",
			"allStateCommitContractTrieCommitTime",
			"allStateCommitTriesCommitTime",
			"allTrieDBUpdateTime",
			"allStateObjectsTime",
			"allWriteClassesLoopTime",
			"allStateCachePushLayerTime",
			"allStateFlushTime",
			"allWriteBatchTime",
			"allCommitHashCalculationTime",
			"allCollectTime",
			"allUpdateTime",
			"innerStoreTime",
			"allStoreTime",
		}
	} else {
		headers = []string{
			"BlockNumber",
			"Timestamp",
			"allVerifyCommTime",
			"allRegisterClassesTime",
			"allUpdateClassTrieTime",
			"allStateCommitClassesTrieCommitTime",
			"allRegisterDeployedContractsTime",
			"allStateCommitKeysSortTime",
			"allStateCommitContractStorageCommitTime",
			"allStateCommitContractTrieUpdateTime",
			"allUpdateContractsTime",
			"allStateCommitContractTrieCommitTime",
			"allCommitHashCalculationTime",
			"allUpdateTime",
			"innerStoreTime",
			"allStoreTime",
		}
	}

	return pr.writer.Write(headers)
}

func (pr *PerformanceReporter) ensureHeaders() error {
	fileInfo, err := pr.file.Stat()
	if err != nil {
		return err
	}

	if fileInfo.Size() == 0 {
		return pr.writeHeaders()
	}

	return nil
}

func (pr *PerformanceReporter) addInitialRow() error {
	fileInfo, err := pr.file.Stat()
	if err != nil {
		return err
	}

	if fileInfo.Size() < 200 {
		now := time.Now()
		timestamp := now.Format("2006-01-02 15:04:05")

		var record []string

		if pr.useNewState {
			record = []string{
				"1",
				timestamp,
				"0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0",
			}
		} else {
			record = []string{
				"1",
				timestamp,
				"0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0", "0",
			}
		}

		if err := pr.writer.Write(record); err != nil {
			return err
		}

		pr.writer.Flush()
		pr.lastReported = 1
	}

	return nil
}

func (pr *PerformanceReporter) UpdateBlockNumber(blockNumber uint64) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.blockNumber = blockNumber

	// Check if we should write a report
	if blockNumber > 0 && blockNumber%pr.reportInterval == 0 && blockNumber != pr.lastReported {
		pr.writeReport()
		pr.lastReported = blockNumber
	}
}

func (pr *PerformanceReporter) AddDuration(metric string, duration int64) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.metrics[metric] += duration
}

func (pr *PerformanceReporter) writeReport() error {
	now := time.Now()
	timestamp := now.Format("2006-01-02 15:04:05")

	var record []string

	if pr.useNewState {
		record = []string{
			fmt.Sprintf("%d", pr.blockNumber),
			timestamp,
			fmt.Sprintf("%d", pr.metrics["allVerifyCommTime"]),
			fmt.Sprintf("%d", pr.metrics["allRegisterClassesTime"]),
			fmt.Sprintf("%d", pr.metrics["allUpdateClassTrieTime"]),
			fmt.Sprintf("%d", pr.metrics["allRegisterDeployedContractsTime"]),
			fmt.Sprintf("%d", pr.metrics["allUpdateContractsTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitStateObjectCommitTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitContractTrieUpdateTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitClassesTrieCommitTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitContractTrieCommitTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitTriesCommitTime"]),
			fmt.Sprintf("%d", pr.metrics["allTrieDBUpdateTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateObjectsTime"]),
			fmt.Sprintf("%d", pr.metrics["allWriteClassesLoopTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCachePushLayerTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateFlushTime"]),
			fmt.Sprintf("%d", pr.metrics["allWriteBatchTime"]),
			fmt.Sprintf("%d", pr.metrics["allCommitHashCalculationTime"]),
			fmt.Sprintf("%d", pr.metrics["allCollectTime"]),
			fmt.Sprintf("%d", pr.metrics["allUpdateTime"]),
			fmt.Sprintf("%d", pr.metrics["innerStoreTime"]),
			fmt.Sprintf("%d", pr.metrics["allStoreTime"]),
		}
	} else {
		record = []string{
			fmt.Sprintf("%d", pr.blockNumber),
			timestamp,
			fmt.Sprintf("%d", pr.metrics["allVerifyCommTime"]),
			fmt.Sprintf("%d", pr.metrics["allRegisterClassesTime"]),
			fmt.Sprintf("%d", pr.metrics["allUpdateClassTrieTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitClassesTrieCommitTime"]),
			fmt.Sprintf("%d", pr.metrics["allRegisterDeployedContractsTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitKeysSortTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitContractStorageCommitTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitContractTrieUpdateTime"]),
			fmt.Sprintf("%d", pr.metrics["allUpdateContractsTime"]),
			fmt.Sprintf("%d", pr.metrics["allStateCommitContractTrieCommitTime"]),
			fmt.Sprintf("%d", pr.metrics["allCommitHashCalculationTime"]),
			fmt.Sprintf("%d", pr.metrics["allUpdateTime"]),
			fmt.Sprintf("%d", pr.metrics["innerStoreTime"]),
			fmt.Sprintf("%d", pr.metrics["allStoreTime"]),
		}
	}

	if err := pr.writer.Write(record); err != nil {
		return err
	}

	pr.writer.Flush()
	return nil
}

func (pr *PerformanceReporter) Close() error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.writer.Flush()
	return pr.file.Close()
}
