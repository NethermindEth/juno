// Package prometheus measures various metrics that
// help to monitor the functioning of Juno. Its main purpose
// is to show how various components of Juno are wonorking.
package prometheus

import (
	"context"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/internal/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

type Server struct {
	server http.Server
}

var (
	noOfRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "no_of_requests",
		Help: "No. of requests sent to and received from the feeder gateway",
	},
		[]string{"Status", "Type"},
	)
	noOfABI = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "no_of_abi",
		Help: "Number of ABI requests sent to and received from the feeder gateway",
	},
		[]string{"Status"},
	)
	countStarknetSync = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "count_starknet_sync",
		Help: "Number of updates and commits made or failed",
	},
		[]string{"Status"},
	)
	timeStarknetSync = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "time_starknet_sync",
		Help: "Number of updates and commits made or failed",
	},
		[]string{"Status"},
	)
)

// Keeps a track of the total number of correct responses received
// Is called whenever the function do in feeder.go is called
func IncreaseRequestsReceived() {
	noOfRequests.WithLabelValues("Received", "Total").Inc()
}

// Keeps a track of the total number of requests is sent
// Is called whenever the function do in feeder.go is called
func IncreaseRequestsSent() {
	noOfRequests.WithLabelValues("Sent", "Total").Inc()
}

// Keeps a track of the total number of requests is sent
// Is called whenever the function do in feeder.go is called
func IncreaseRequestsFailed() {
	noOfRequests.WithLabelValues("Failed", "Total").Inc()
}

// This increases when the request in GetCode in feeder.go is called
func IncreaseABISent() {
	noOfABI.WithLabelValues("Sent").Inc()
}

// This increases when the request in GetCode in feeder.go fails
func IncreaseABIFailed() {
	noOfABI.WithLabelValues("Failed").Inc()
}

// This increases when the response of GetCode in feeder.go is received
func IncreaseABIReceived() {
	noOfABI.WithLabelValues("Received").Inc()
}

// This increases when the request in GetContractAddresses in feeder.go is sent
func IncreaseContractAddressesSent() {
	noOfRequests.WithLabelValues("Sent", "Contract Addresses").Inc()
}

// This increases when the request in CallContract in feeder.go is sent
func IncreaseContractCallsSent() {
	noOfRequests.WithLabelValues("Sent", "Contract Calls").Inc()
}

// This increases when the request in GetBlock in feeder.go is sent
func IncreaseBlockSent() {
	noOfRequests.WithLabelValues("Sent", "Blocks").Inc()
}

// This increases when the request in GetStateUpdateGoerli in feeder.go is sent
func IncreaseStateUpdateGoerliSent() {
	// notest
	noOfRequests.WithLabelValues("Sent", "State Update Goerli").Inc()
}

// This increases when the request in GetStateUpdate in feeder.go is sent
func IncreaseStateUpdateSent() {
	noOfRequests.WithLabelValues("Sent", "State Update").Inc()
}

// This increases when the request in GetFullContract in feeder.go is sent
func IncreaseFullContractsSent() {
	noOfRequests.WithLabelValues("Sent", "Full Contracts").Inc()
}

// This increases when the request in GetStorage in feeder.go is sent
func IncreaseContractStorageSent() {
	noOfRequests.WithLabelValues("Sent", "Contract Storage").Inc()
}

// This increases when the request in GetTransactionStatus in feeder.go is sent
func IncreaseTxStatusSent() {
	noOfRequests.WithLabelValues("Sent", "Transaction Status").Inc()
}

// This increases when the request in GetTransactionTrace in feeder.go is sent
func IncreaseTxTraceSent() {
	// notest
	noOfRequests.WithLabelValues("Sent", "Transaction Trace").Inc()
}

// This increases when the request in GetTransaction in feeder.go is sent
func IncreaseTxSent() {
	noOfRequests.WithLabelValues("Sent", "Transaction").Inc()
}

// This increases when the request in GetTransactionReceipt in feeder.go is sent
func IncreaseTxReceiptSent() {
	noOfRequests.WithLabelValues("Sent", "Transaction Receipt").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go is sent
func IncreaseBlockHashSent() {
	noOfRequests.WithLabelValues("Sent", "BlockHashByID").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go is sent
func IncreaseBlockIDSent() {
	noOfRequests.WithLabelValues("Sent", "BlockIDByHash").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go is sent
func IncreaseTxHashSent() {
	noOfRequests.WithLabelValues("Sent", "TransactionHashByID").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go is sent
func IncreaseTxIDSent() {
	noOfRequests.WithLabelValues("Sent", "TransactionIDByHash").Inc()
}

// This increases when the response of GetContractAddresses in feeder.go is received
func IncreaseContractAddressesReceived() {
	noOfRequests.WithLabelValues("Received", "Contract Addresses").Inc()
}

// This increases when the response of CallContract in feeder.go is received
func IncreaseContractCallsReceived() {
	noOfRequests.WithLabelValues("Received", "Contract Calls").Inc()
}

// This increases when the response of GetBlock in feeder.go is received
func IncreaseBlockReceived() {
	noOfRequests.WithLabelValues("Received", "Blocks").Inc()
}

// This increases when the response of GetStateUpdateGoerli in feeder.go is received
func IncreaseStateUpdateGoerliReceived() {
	// notest
	noOfRequests.WithLabelValues("Received", "State Update Goerli").Inc()
}

// This increases when the response of GetStateUpdate in feeder.go is received
func IncreaseStateUpdateReceived() {
	noOfRequests.WithLabelValues("Received", "State Update").Inc()
}

// This increases when the response of GetFullContract in feeder.go is received
func IncreaseFullContractsReceived() {
	noOfRequests.WithLabelValues("Received", "Full Contracts").Inc()
}

// This increases when the response of GetStorage in feeder.go is received
func IncreaseContractStorageReceived() {
	noOfRequests.WithLabelValues("Received", "Contract Storage").Inc()
}

// This increases when the response of GetTransactionStatus in feeder.go is received
func IncreaseTxStatusReceived() {
	noOfRequests.WithLabelValues("Received", "Transaction Status").Inc()
}

// This increases when the response of GetTransactionTrace in feeder.go is received
func IncreaseTxTraceReceived() {
	// notest
	noOfRequests.WithLabelValues("Received", "Transaction Trace").Inc()
}

// This increases when the response of GetTransaction in feeder.go is received
func IncreaseTxReceived() {
	noOfRequests.WithLabelValues("Received", "Transaction").Inc()
}

// This increases when the response of GetTransactionReceipt in feeder.go is received
func IncreaseTxReceiptReceived() {
	noOfRequests.WithLabelValues("Received", "Transaction Receipt").Inc()
}

// This increases when the response of GetBlockHashById in feeder.go is received
func IncreaseBlockHashReceived() {
	noOfRequests.WithLabelValues("Received", "BlockHashByID").Inc()
}

// This increases when the response of GetBlockHashById in feeder.go is received
func IncreaseBlockIDReceived() {
	noOfRequests.WithLabelValues("Received", "BlockIDByHash").Inc()
}

// This increases when the response of GetBlockHashById in feeder.go is received
func IncreaseTxHashReceived() {
	noOfRequests.WithLabelValues("Received", "TransactionHashByID").Inc()
}

// This increases when the response of GetBlockHashById in feeder.go is received
func IncreaseTxIDReceived() {
	noOfRequests.WithLabelValues("Received", "TransactionIDByHash").Inc()
}

// This increases when the request in GetContractAddresses in feeder.go fails
func IncreaseContractAddressesFailed() {
	noOfRequests.WithLabelValues("Failed", "Contract Addresses").Inc()
}

// This increases when the request in CallContract in feeder.go fails
func IncreaseContractCallsFailed() {
	// notest
	noOfRequests.WithLabelValues("Failed", "Contract Calls").Inc()
}

// This increases when the request in GetBlock in feeder.go fails
func IncreaseBlockFailed() {
	noOfRequests.WithLabelValues("Failed", "Blocks").Inc()
}

// This increases when the request in GetStateUpdateGoerli in feeder.go fails
func IncreaseStateUpdateGoerliFailed() {
	// notest
	noOfRequests.WithLabelValues("Failed", "State Update Goerli").Inc()
}

// This increases when the request in GetStateUpdate in feeder.go fails
func IncreaseStateUpdateFailed() {
	noOfRequests.WithLabelValues("Failed", "State Update").Inc()
}

// This increases when the request in GetFullContract in feeder.go fails
func IncreaseFullContractsFailed() {
	noOfRequests.WithLabelValues("Failed", "Full Contracts").Inc()
}

// This increases when the request in GetStorage in feeder.go fails
func IncreaseContractStorageFailed() {
	noOfRequests.WithLabelValues("Failed", "Contract Storage").Inc()
}

// This increases when the request in GetTransactionStatus in feeder.go fails
func IncreaseTxStatusFailed() {
	noOfRequests.WithLabelValues("Failed", "Transaction Status").Inc()
}

// This increases when the request in GetTransactionTrace in feeder.go fails
func IncreaseTxTraceFailed() {
	// notest
	noOfRequests.WithLabelValues("Failed", "Transaction Trace").Inc()
}

// This increases when the request in GetTransaction in feeder.go fails
func IncreaseTxFailed() {
	noOfRequests.WithLabelValues("Failed", "Transaction").Inc()
}

// This increases when the request in GetTransactionReceipt in feeder.go fails
func IncreaseTxReceiptFailed() {
	noOfRequests.WithLabelValues("Failed", "Transaction Receipt").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go fails
func IncreaseBlockHashFailed() {
	noOfRequests.WithLabelValues("Failed", "BlockHashByID").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go fails
func IncreaseBlockIDFailed() {
	noOfRequests.WithLabelValues("Failed", "BlockIDByHash").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go fails
func IncreaseTxHashFailed() {
	noOfRequests.WithLabelValues("Failed", "TransactionHashByID").Inc()
}

// This increases when the request in GetBlockHashById in feeder.go fails
func IncreaseTxIDFailed() {
	noOfRequests.WithLabelValues("Failed", "TransactionIDByHash").Inc()
}

// Starknet sync metrics
// This increases when the StateUpdateAndCommit method in state.go throws an error
func IncreaseCountStarknetStateFailed() {
	// notest
	countStarknetSync.WithLabelValues("Failed").Inc()
}

// This increases when the StateUpdateAndCommit method in state.go updates the state successfully
func IncreaseCountStarknetStateSuccess() {
	countStarknetSync.WithLabelValues("Success").Inc()
}

// Changes the total and average amount of time needed for updating and committing a block
func UpdateStarknetSyncTime(t float64) {
	timeStarknetSync.WithLabelValues("Total").Add(t)
	val1 := &dto.Metric{}
	val2 := &dto.Metric{}
	timeStarknetSync.WithLabelValues("Total").Write(val1)
	countStarknetSync.WithLabelValues("Success").Write(val2)
	val3 := val1.Gauge.GetValue() / val2.Counter.GetValue()
	timeStarknetSync.WithLabelValues("Average").Set(val3)
}

func SetupMetric(port string) *Server {
	// notest
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return &Server{server: http.Server{Addr: port, Handler: mux}}
}

// ListenAndServe listens on the TCP network and handles requests on
// incoming connections.
func (s *Server) ListenAndServe() error {
	// notest
	log.Default.Info("Handling metrics .... ")

	return s.server.ListenAndServe()
}

// Close gracefully shuts down the server.
func (s *Server) Close(timeout time.Duration) error {
	ctx := context.Background()
	ctx, _ = context.WithTimeout(ctx, timeout)
	return s.server.Shutdown(ctx)
}
