package core

import (
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/performance"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	allVerifyComm                           uint64
	allVerifyCommTime                       int64
	allRegisterClassesTime                  int64
	allUpdateClassTrieTime                  int64
	allStateCommitClassesTrieCommitTime     int64
	allRegisterDeployedContractsTime        int64
	allStateCommitKeysSortTime              int64
	allStateCommitContractStorageCommitTime int64
	allStateCommitContractTrieUpdateTime    int64
	allUpdateContractsTime                  int64
	allStateCommitContractTrieCommitTime    int64
	allReplaceContractsTime                 int64
	allUpdateContractNoncesTime             int64
	allUpdateContractsStorageTime           int64
	allCheckContractsDeployedTime           int64
)

func incCounter(c *uint64) {
	atomic.AddUint64(c, 1)
}

func addDuration(c *int64, d time.Duration) {
	atomic.AddInt64(c, d.Nanoseconds())
	// Also add to performance reporter
	performance.AddDuration(getMetricName(c), d.Nanoseconds())
}

func getMetricName(c *int64) string {
	switch c {
	case &allVerifyCommTime:
		return "allVerifyCommTime"
	case &allRegisterClassesTime:
		return "allRegisterClassesTime"
	case &allUpdateClassTrieTime:
		return "allUpdateClassTrieTime"
	case &allStateCommitClassesTrieCommitTime:
		return "allStateCommitClassesTrieCommitTime"
	case &allRegisterDeployedContractsTime:
		return "allRegisterDeployedContractsTime"
	case &allStateCommitKeysSortTime:
		return "allStateCommitKeysSortTime"
	case &allStateCommitContractStorageCommitTime:
		return "allStateCommitContractStorageCommitTime"
	case &allStateCommitContractTrieUpdateTime:
		return "allStateCommitContractTrieUpdateTime"
	case &allUpdateContractsTime:
		return "allUpdateContractsTime"
	case &allStateCommitContractTrieCommitTime:
		return "allStateCommitContractTrieCommitTime"
	case &allReplaceContractsTime:
		return "allReplaceContractsTime"
	case &allUpdateContractNoncesTime:
		return "allUpdateContractNoncesTime"
	case &allUpdateContractsStorageTime:
		return "allUpdateContractsStorageTime"
	case &allCheckContractsDeployedTime:
		return "allCheckContractsDeployedTime"
	default:
		return "unknown"
	}
}

type StateMetricsCollector struct{}

func (c *StateMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	descs := []*prometheus.Desc{
		prometheus.NewDesc("x_deprecated_state_all_verify_comm", "All verify comm", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_verify_comm_time_ns", "Total time spent verifying comm (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_register_classes_time_ns", "Total time spent registering classes (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_update_class_trie_time_ns", "Total time spent updating class trie (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_state_commit_classes_trie_commit_time_ns", "Total time spent committing classes trie (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_register_deployed_contracts_time_ns", "Total time spent registering deployed contracts (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_state_commit_keys_sort_time_ns", "Total time spent sorting keys (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_state_commit_contract_storage_commit_time_ns", "Total time spent committing contract storage (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_state_commit_contract_trie_update_time_ns", "Total time spent updating contract trie (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_update_contracts_time_ns", "Total time spent updating contracts (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_state_commit_contract_trie_commit_time_ns", "Total time spent committing contract trie (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_replace_contracts_time_ns", "Total time spent replacing contracts (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_update_contract_nonces_time_ns", "Total time spent updating contract nonces (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_update_contracts_storage_time_ns", "Total time spent updating contract storage (ns)", nil, nil),
		prometheus.NewDesc("x_deprecated_state_all_check_contracts_deployed_time_ns", "Total time spent checking contracts deployed (ns)", nil, nil),
	}
	for _, d := range descs {
		ch <- d
	}
}

func (c *StateMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_verify_comm", "All verify comm", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&allVerifyComm)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_verify_comm_time_ns", "Total time spent verifying comm (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allVerifyCommTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_register_classes_time_ns", "Total time spent registering classes (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allRegisterClassesTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_update_class_trie_time_ns", "Total time spent updating class trie (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allUpdateClassTrieTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_state_commit_classes_trie_commit_time_ns", "Total time spent committing classes trie (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitClassesTrieCommitTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_register_deployed_contracts_time_ns", "Total time spent registering deployed contracts (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allRegisterDeployedContractsTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_state_commit_keys_sort_time_ns", "Total time spent sorting keys (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitKeysSortTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_state_commit_contract_storage_commit_time_ns", "Total time spent committing contract storage (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitContractStorageCommitTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_state_commit_contract_trie_update_time_ns", "Total time spent updating contract trie (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitContractTrieUpdateTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_update_contracts_time_ns", "Total time spent updating contracts (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allUpdateContractsTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_state_commit_contract_trie_commit_time_ns", "Total time spent committing contract trie (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitContractTrieCommitTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_replace_contracts_time_ns", "Total time spent replacing contracts (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allReplaceContractsTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_update_contract_nonces_time_ns", "Total time spent updating contract nonces (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allUpdateContractNoncesTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_update_contracts_storage_time_ns", "Total time spent updating contract storage (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allUpdateContractsStorageTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_deprecated_state_all_check_contracts_deployed_time_ns", "Total time spent checking contracts deployed (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allCheckContractsDeployedTime)),
	)
}
