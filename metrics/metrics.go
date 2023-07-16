package metrics

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var Enabled = false

func MustRegister(collectors ...prometheus.Collector) {
	if Enabled {
		prometheus.MustRegister(collectors...)
	}
}

type Metrics struct {
	listener net.Listener
}

func New(listener net.Listener) *Metrics {
	return &Metrics{
		listener: listener,
	}
}

func (m *Metrics) Run(ctx context.Context) error {
	errCh := make(chan error)

	srv := &http.Server{
		Addr:    m.listener.Addr().String(),
		Handler: promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{Registry: prometheus.DefaultRegisterer}),
		// ReadTimeout is also treated as ReadHeaderTimeout and IdleTimeout.
		ReadTimeout: 30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		errCh <- srv.Shutdown(context.Background())
		close(errCh)
	}()

	if err := srv.Serve(m.listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-errCh
}
