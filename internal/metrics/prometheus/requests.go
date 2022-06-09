package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Number of requests received
var (
	no_of_requests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "no_of_requests",
		Help: "No. of requests received",
	})
)

func increaseRequests() {
	no_of_requests.Inc()
}

// Block Sync Time
var (
	block_sync_time = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "block_sync_time",
		Help: "Time taken to sync the blockchain to the current state",
	})
)

func increaseBlockSyncTime() {

}

func init() {
	prometheus.MustRegister(no_of_requests)
	prometheus.MustRegister(block_sync_time)
}

// Services - Contract Hash, ABI,
func main() {

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2048", nil)
}
