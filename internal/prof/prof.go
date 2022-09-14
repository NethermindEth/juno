package prof

import (
	"context"
	"errors"
	"net/http"
	"net/http/pprof"
	"time"

	. "github.com/NethermindEth/juno/internal/log"
	"go.uber.org/zap"
)

// Prof represents a HTTP profiler over TCP.
type Prof struct {
	logger *zap.SugaredLogger
	server *http.Server
}

// register creates a new http.ServeMux and registers all the handlers
// that will be responsible for servicing profiling requests.
func register() http.Handler {
	mux := http.NewServeMux()

	// Leverage the http.ServeMux catch-all behaviour to redirect the user
	// to the profiling index page if they enter the wrong URL. Note that
	// all routes under /debug/pprof/ will still return 404 Not found
	// errors.
	mux.Handle("/", http.RedirectHandler("/debug/pprof/", http.StatusSeeOther))

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return mux
}

// New constructs a new Prof which will serve requests via TCP at addr
// where addr is of the form host:port.
func New(addr string, logger *zap.SugaredLogger) *Prof {
	// notest
	return &Prof{
		// TODO: Use injected logger.
		logger: Logger,
		server: &http.Server{
			Addr:    addr,
			Handler: register(),
		},
	}
}

// Serve starts the profiling server and starts listening on the address
// specified in the constructor.
func (p *Prof) Serve(ch chan<- error) {
	// notest
	p.logger.Infow("Starting profiling server on address", "address", p.server.Addr)

	go func(ch chan<- error) {
		if err := p.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			ch <- errors.New("prof: listen and serve: " + err.Error())
		}
		close(ch)
	}(ch)
}

// Close gracefully shuts down the profiling server.
func (p *Prof) Close(timeout time.Duration) error {
	// notest
	p.logger.Info("Shutting down profiling server")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return p.server.Shutdown(ctx)
}
