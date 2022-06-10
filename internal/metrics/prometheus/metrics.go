// Package prometheus measures various metrics that
// help to monitor the functioning of Juno. Its main purpose
// is to show how various components of Juno are working.
package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Number of requests received
var (
	no_of_requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "no_of_requests",
		Help: "No. of requests sent to and received from the feeder gateway",
	},
		[]string{"Status", "Type"},
	)
	no_of_abi = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "no_of_abi_sent",
		Help: "Number of ABI sent",
	},
		[]string{"Status"},
	)
)

// block_sync_time = promauto.NewHistogram(prometheus.HistogramOpts{
// 	Name: "block_sync_time",
// 	Help: "Time taken to sync the blockchain to the current state",
// })
// func IncreaseBlockSyncTime() {

// }

// Keeps a track of the total of correct response received
// Is called whenever the function do in feeder.go is called
func IncreaseRequestsReceived() {
	no_of_requests.WithLabelValues("Received", "Total").Inc()
}

func IncreaseRequestsSent() {
	no_of_requests.WithLabelValues("Sent", "Total").Inc()
}

func IncreaseRequestsFailed() {
	no_of_requests.WithLabelValues("Failed", "Failed", "Total").Inc()
}

// This increases when the request in GetCode in feeder.go is called
func IncreaseABISent() {
	no_of_abi.WithLabelValues("Sent").Inc()
}

// This increases when the request in GetCode in feeder.go fails
func IncreaseABIFailed() {
	no_of_abi.WithLabelValues("Failed").Inc()
}

// This increases when the response of GetCode in feeder.go is received
func IncreaseABIReceived() {
	no_of_abi.WithLabelValues("Received").Inc()
}

// This increases when the request in GetContractAddresses in feeder.go is sent
func IncreaseContractAddressesSent() {
	no_of_requests.WithLabelValues("Sent", "Contract Addresses").Inc()
}

// This increases when the request in CallContract in feeder.go is sent
func IncreaseContractCallsSent() {
	no_of_requests.WithLabelValues("Sent", "Contract Calls").Inc()
}

// This increases when the request in GetBlock in feeder.go is sent
func IncreaseBlockSent() {
	no_of_requests.WithLabelValues("Sent", "Blocks").Inc()
}

// This increases when the request in GetStateUpdate in feeder.go is sent
func IncreaseStateUpdateSent() {
	no_of_requests.WithLabelValues("Sent", "State Update").Inc()
}

// This increases when the request in GetFullContract in feeder.go is sent
func IncreaseFullContractsSent() {
	no_of_requests.WithLabelValues("Sent", "Full Contracts").Inc()
}

// This increases when the request in GetStorage in feeder.go is sent
func IncreaseContractStorageSent() {
	no_of_requests.WithLabelValues("Sent", "Contract Storage").Inc()
}

// This increases when the request in GetTransactionStatus in feeder.go is sent
func IncreaseTxStatusSent() {
	no_of_requests.WithLabelValues("Sent", "Transaction Status").Inc()
}

// This increases when the request in GetTransaction in feeder.go is sent
func IncreaseTxSent() {
	no_of_requests.WithLabelValues("Sent", "Transaction").Inc()
}

// This increases when the request in GetTransactionReceipt in feeder.go is sent
func IncreaseTxReceiptSent() {
	no_of_requests.WithLabelValues("Sent", "Transaction Receipt").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go is sent
func IncreaseBlockHashSent() {
	no_of_requests.WithLabelValues("Sent", "BlockHashByID").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go is sent
func IncreaseBlockIDSent() {
	no_of_requests.WithLabelValues("Sent", "BlockIDByHash").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go is sent
func IncreaseTxHashSent() {
	no_of_requests.WithLabelValues("Sent", "TransactionHashByID").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go is sent
func IncreaseTxIDSent() {
	no_of_requests.WithLabelValues("Sent", "TransactionIDByHash").Inc()
}

// This increases when the response of GetContractAddresses in feeder.go is received
func IncreaseContractAddressesReceived() {
	no_of_requests.WithLabelValues("Received", "Contract Addresses").Inc()
}

// This increases when the response of CallContract in feeder.go is received
func IncreaseContractCallsReceived() {
	no_of_requests.WithLabelValues("Received", "Contract Calls").Inc()
}

// This increases when the response of GetBlock in feeder.go is received
func IncreaseBlockReceived() {
	no_of_requests.WithLabelValues("Received", "Blocks").Inc()
}

// This increases when the response of GetStateUpdate in feeder.go is received
func IncreaseStateUpdateRecived() {
	no_of_requests.WithLabelValues("Received", "State Update").Inc()
}

// This increases when the response of GetFullContract in feeder.go is received
func IncreaseFullContractsReceived() {
	no_of_requests.WithLabelValues("Received", "Full Contracts").Inc()
}

// This increases when the response of GetStorage in feeder.go is received
func IncreaseContractStorageReceived() {
	no_of_requests.WithLabelValues("Received", "Contract Storage").Inc()
}

// This increases when the response of GetTransactionStatus in feeder.go is received
func IncreaseTxStatusReceived() {
	no_of_requests.WithLabelValues("Received", "Transaction Status").Inc()
}

// This increases when the response of GetTransaction in feeder.go is received
func IncreaseTxReceived() {
	no_of_requests.WithLabelValues("Received", "Transaction").Inc()
}

// This increases when the response of GetTransactionReceipt in feeder.go is received
func IncreaseTxReceiptReceived() {
	no_of_requests.WithLabelValues("Received", "Transaction Receipt").Inc()
}

// This increases when the response of GetBlockHashById in feeder.go is received
func IncreaseBlockHashReceived() {
	no_of_requests.WithLabelValues("Received", "BlockHashByID").Inc()
}

// This increases when the response of GetBlockHashById in feeder.go is received
func IncreaseBlockIDReceived() {
	no_of_requests.WithLabelValues("Received", "BlockIDByHash").Inc()
}

// This increases when the response of GetBlockHashById in feeder.go is received
func IncreaseTxHashReceived() {
	no_of_requests.WithLabelValues("Received", "TransactionHashByID").Inc()
}

// This increases when the response of GetBlockHashById in feeder.go is received
func IncreaseTxIDReceived() {
	no_of_requests.WithLabelValues("Received", "TransactionIDByHash").Inc()
}

// This increases when the request in GetContractAddresses in feeder.go fails
func IncreaseContractAddressesFailed() {
	no_of_requests.WithLabelValues("Failed", "Contract Addresses").Inc()
}

// This increases when the request in CallContract in feeder.go fails
func IncreaseContractCallsFailed() {
	no_of_requests.WithLabelValues("Failed", "Contract Calls").Inc()
}

// This increases when the request in GetBlock in feeder.go fails
func IncreaseBlockFailed() {
	no_of_requests.WithLabelValues("Failed", "Blocks").Inc()
}

// This increases when the request in GetStateUpdate in feeder.go fails
func IncreaseStateUpdateFailed() {
	no_of_requests.WithLabelValues("Failed", "State Update").Inc()
}

// This increases when the request in GetFullContract in feeder.go fails
func IncreaseFullContractsFailed() {
	no_of_requests.WithLabelValues("Failed", "Full Contracts").Inc()
}

// This increases when the request in GetStorage in feeder.go fails
func IncreaseContractStorageFailed() {
	no_of_requests.WithLabelValues("Failed", "Contract Storage").Inc()
}

// This increases when the request in GetTransactionStatus in feeder.go fails
func IncreaseTxStatusFailed() {
	no_of_requests.WithLabelValues("Failed", "Transaction Status").Inc()
}

// This increases when the request in GetTransaction in feeder.go fails
func IncreaseTxFailed() {
	no_of_requests.WithLabelValues("Failed", "Transaction").Inc()
}

// This increases when the request in GetTransactionReceipt in feeder.go fails
func IncreaseTxReceiptFailed() {
	no_of_requests.WithLabelValues("Failed", "Transaction Receipt").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go fails
func IncreaseBlockHashFailed() {
	no_of_requests.WithLabelValues("Failed", "BlockHashByID").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go fails
func IncreaseBlockIDFailed() {
	no_of_requests.WithLabelValues("Failed", "BlockIDByHash").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go fails
func IncreaseTxHashFailed() {
	no_of_requests.WithLabelValues("Failed", "TransactionHashByID").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go fails
func IncreaseTxIDFailed() {
	no_of_requests.WithLabelValues("Failed", "TransactionIDByHash").Inc()
}

func init() {
	prometheus.MustRegister(no_of_requests)
	// prometheus.MustRegister(block_sync_time)
	prometheus.MustRegister(no_of_abi)
}

func main() {

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2048", nil)
}
