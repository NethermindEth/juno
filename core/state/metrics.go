package state

import (
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/performance"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	allVerifyComm                    uint64
	allVerifyCommTime                int64
	allRegisterClassesTime           int64
	allUpdateClassTrieTime           int64
	allRegisterDeployedContractsTime int64
	allUpdateContractsTime           int64
	allStateCommitTime               int64
	allStateCachePushLayerTime       int64
	allStateFlushTime                int64
	allTrieDBUpdateTime              int64
	allStateObjectsTime              int64
	allDeleteContractTime            int64
	allDeleteStorageNodesByPathTime  int64
	allWriteContractTime             int64
	allWriteStorageHistoryTime       int64
	allWriteContractHistoryTime      int64
	allWriteContractNonceTime        int64
	allWriteContractClassHashTime    int64
	allWriteClassesLoopTime          int64
	allDeleteClassTime               int64
	allWriteClassTime                int64
	allWriteBatchTime                int64

	// state commit metrics
	allStateCommitKeysSortTime           int64
	allStateCommitStateObjectCommitTime  int64
	allStateCommitMergeTime              int64
	allStateCommitContractTrieUpdateTime int64
	allStateCommitClassesTrieCommitTime  int64
	allStateCommitContractTrieCommitTime int64
	allStateCommitTriesCommitTime        int64
	allStateContractNodesMergeTime       int64
	allStateStateCommitmentTime          int64
	allStateClassNodesCopyTime           int64
)

func incCounter(c *uint64) {
	atomic.AddUint64(c, 1)
}

func addDuration(c *int64, d time.Duration) {
	atomic.AddInt64(c, d.Nanoseconds())
	// Also add to performance reporter
	performance.AddDuration(getStateMetricName(c), d.Nanoseconds())
}

func getStateMetricName(c *int64) string {
	switch c {
	case &allVerifyCommTime:
		return "allVerifyCommTime"
	case &allRegisterClassesTime:
		return "allRegisterClassesTime"
	case &allUpdateClassTrieTime:
		return "allUpdateClassTrieTime"
	case &allRegisterDeployedContractsTime:
		return "allRegisterDeployedContractsTime"
	case &allUpdateContractsTime:
		return "allUpdateContractsTime"
	case &allStateCommitTime:
		return "allStateCommitTime"
	case &allStateCachePushLayerTime:
		return "allStateCachePushLayerTime"
	case &allStateFlushTime:
		return "allStateFlushTime"
	case &allTrieDBUpdateTime:
		return "allTrieDBUpdateTime"
	case &allStateObjectsTime:
		return "allStateObjectsTime"
	case &allDeleteContractTime:
		return "allDeleteContractTime"
	case &allDeleteStorageNodesByPathTime:
		return "allDeleteStorageNodesByPathTime"
	case &allWriteContractTime:
		return "allWriteContractTime"
	case &allWriteStorageHistoryTime:
		return "allWriteStorageHistoryTime"
	case &allWriteContractHistoryTime:
		return "allWriteContractHistoryTime"
	case &allWriteContractNonceTime:
		return "allWriteContractNonceTime"
	case &allWriteContractClassHashTime:
		return "allWriteContractClassHashTime"
	case &allWriteClassesLoopTime:
		return "allWriteClassesLoopTime"
	case &allDeleteClassTime:
		return "allDeleteClassTime"
	case &allWriteClassTime:
		return "allWriteClassTime"
	case &allWriteBatchTime:
		return "allWriteBatchTime"
	case &allStateCommitKeysSortTime:
		return "allStateCommitKeysSortTime"
	case &allStateCommitStateObjectCommitTime:
		return "allStateCommitStateObjectCommitTime"
	case &allStateCommitMergeTime:
		return "allStateCommitMergeTime"
	case &allStateCommitContractTrieUpdateTime:
		return "allStateCommitContractTrieUpdateTime"
	case &allStateCommitClassesTrieCommitTime:
		return "allStateCommitClassesTrieCommitTime"
	case &allStateCommitContractTrieCommitTime:
		return "allStateCommitContractTrieCommitTime"
	case &allStateCommitTriesCommitTime:
		return "allStateCommitTriesCommitTime"
	case &allStateContractNodesMergeTime:
		return "allStateContractNodesMergeTime"
	case &allStateStateCommitmentTime:
		return "allStateStateCommitmentTime"
	case &allStateClassNodesCopyTime:
		return "allStateClassNodesCopyTime"
	default:
		return "unknown"
	}
}

type StateMetricsCollector struct{}

func (c *StateMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	descs := []*prometheus.Desc{
		prometheus.NewDesc("x_state_all_verify_comm", "All verify comm", nil, nil),
		prometheus.NewDesc("x_state_all_verify_comm_time_ns", "Total time spent verifying comm (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_register_classes_time_ns", "Total time spent registering classes (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_update_class_trie_time_ns", "Total time spent updating class trie (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_register_deployed_contracts_time_ns", "Total time spent registering deployed contracts (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_update_contracts_time_ns", "Total time spent updating contracts (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_time_ns", "Total time spent committing state (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_cache_push_layer_time_ns", "Total time spent pushing layer to state cache (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_flush_time_ns", "Total time spent flushing state (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_trie_db_update_time_ns", "Total time spent updating trie db (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_objects_time_ns", "Total time spent updating state objects (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_delete_contract_time_ns", "Total time spent deleting contract (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_delete_storage_nodes_by_path_time_ns", "Total time spent deleting storage nodes by path (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_write_contract_time_ns", "Total time spent writing contract (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_write_storage_history_time_ns", "Total time spent writing storage history (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_write_contract_history_time_ns", "Total time spent writing contract history (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_write_contract_nonce_time_ns", "Total time spent writing contract nonce (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_write_contract_class_hash_time_ns", "Total time spent writing contract class hash (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_write_classes_loop_time_ns", "Total time spent writing classes loop (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_delete_class_time_ns", "Total time spent deleting class (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_write_class_time_ns", "Total time spent writing class (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_write_batch_time_ns", "Total time spent writing batch (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_keys_sort_time_ns", "Total time spent sorting keys (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_state_object_commit_time_ns", "Total time spent committing state objects (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_merge_time_ns", "Total time spent merging nodes (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_contract_trie_update_time_ns", "Total time spent updating contract trie (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_classes_trie_commit_time_ns", "Total time spent committing classes trie (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_contract_trie_commit_time_ns", "Total time spent committing contract trie (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_tries_commit_time_ns", "Total time spent committing tries (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_contract_nodes_merge_time_ns", "Total time spent merging contract nodes (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_state_commitment_time_ns", "Total time spent committing state commitment (ns)", nil, nil),
		prometheus.NewDesc("x_state_all_state_commit_class_nodes_copy_time_ns", "Total time spent copying class nodes (ns)", nil, nil),
	}
	for _, d := range descs {
		ch <- d
	}
}

func (c *StateMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_verify_comm", "All verify comm", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadUint64(&allVerifyComm)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_verify_comm_time_ns", "Total time spent verifying comm (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allVerifyCommTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_register_classes_time_ns", "Total time spent registering classes (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allRegisterClassesTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_update_class_trie_time_ns", "Total time spent updating class trie (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allUpdateClassTrieTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_register_deployed_contracts_time_ns", "Total time spent registering deployed contracts (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allRegisterDeployedContractsTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_update_contracts_time_ns", "Total time spent updating contracts (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allUpdateContractsTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_time_ns", "Total time spent committing state (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_cache_push_layer_time_ns", "Total time spent pushing layer to state cache (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCachePushLayerTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_flush_time_ns", "Total time spent flushing state (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateFlushTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_trie_db_update_time_ns", "Total time spent updating trie db (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allTrieDBUpdateTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_objects_time_ns", "Total time spent updating state objects (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateObjectsTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_delete_contract_time_ns", "Total time spent deleting contract (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allDeleteContractTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_delete_storage_nodes_by_path_time_ns", "Total time spent deleting storage nodes by path (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allDeleteStorageNodesByPathTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_write_contract_time_ns", "Total time spent writing contract (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allWriteContractTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_write_storage_history_time_ns", "Total time spent writing storage history (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allWriteStorageHistoryTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_write_contract_history_time_ns", "Total time spent writing contract history (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allWriteContractHistoryTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_write_contract_nonce_time_ns", "Total time spent writing contract nonce (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allWriteContractNonceTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_write_contract_class_hash_time_ns", "Total time spent writing contract history (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allWriteContractClassHashTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_write_classes_loop_time_ns", "Total time spent writing classes loop (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allWriteClassesLoopTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_delete_class_time_ns", "Total time spent deleting class (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allDeleteClassTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_write_class_time_ns", "Total time spent writing class (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allWriteClassTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_write_batch_time_ns", "Total time spent writing batch (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allWriteBatchTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_keys_sort_time_ns", "Total time spent sorting keys (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitKeysSortTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_state_object_commit_time_ns", "Total time spent committing state objects (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitStateObjectCommitTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_merge_time_ns", "Total time spent merging nodes (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitMergeTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_contract_trie_update_time_ns", "Total time spent updating contract trie (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitContractTrieUpdateTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_classes_trie_commit_time_ns", "Total time spent committing classes trie (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitClassesTrieCommitTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_contract_trie_commit_time_ns", "Total time spent committing contract trie (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitContractTrieCommitTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_tries_commit_time_ns", "Total time spent committing tries (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateCommitTriesCommitTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_contract_nodes_merge_time_ns", "Total time spent merging contract nodes (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateContractNodesMergeTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_state_commitment_time_ns", "Total time spent committing state commitment (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateStateCommitmentTime)),
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("x_state_all_state_commit_class_nodes_copy_time_ns", "Total time spent copying class nodes (ns)", nil, nil),
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&allStateClassNodesCopyTime)),
	)
}
