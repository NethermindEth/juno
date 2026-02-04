package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/db"
	junogrpc "github.com/NethermindEth/juno/grpc"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/sourcegraph/conc"
	"google.golang.org/grpc"
)

const (
	defaultReadTimeout       = 30 * time.Second
	defaultReadHeaderTimeout = 30 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 2 * time.Minute
)

type httpService struct {
	srv *http.Server
}

var _ service.Service = (*httpService)(nil)

func (h *httpService) Run(ctx context.Context) error {
	errCh := make(chan error)
	defer close(errCh)

	var wg conc.WaitGroup
	defer wg.Wait()
	wg.Go(func() {
		if err := h.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	})

	select {
	case <-ctx.Done():
		return h.srv.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

func (h *httpService) registerOnShutdown(f func()) {
	h.srv.RegisterOnShutdown(f)
}

func makeHTTPService(host string, port uint16, handler http.Handler) *httpService {
	portStr := strconv.FormatUint(uint64(port), 10)
	return &httpService{
		srv: &http.Server{
			Addr:              net.JoinHostPort(host, portStr),
			Handler:           handler,
			ReadHeaderTimeout: defaultReadHeaderTimeout,
			ReadTimeout:       defaultReadTimeout,
			WriteTimeout:      defaultWriteTimeout,
			IdleTimeout:       defaultIdleTimeout,
		},
	}
}

func exactPathServer(path string, handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	}
}

func makeRPCOverHTTP(host string, port uint16, servers map[string]*jsonrpc.Server,
	httpHandlers map[string]http.HandlerFunc, log utils.StructuredLogger, metricsEnabled bool, corsEnabled bool,
) *httpService {
	var listener jsonrpc.NewRequestListener
	if metricsEnabled {
		listener = makeHTTPMetrics()
	}

	mux := http.NewServeMux()
	for path, server := range servers {
		httpHandler := jsonrpc.NewHTTP(server, log)
		if listener != nil {
			httpHandler = httpHandler.WithListener(listener)
		}
		mux.Handle(path, exactPathServer(path, httpHandler))
	}
	for path, handler := range httpHandlers {
		mux.HandleFunc(path, handler)
	}

	var handler http.Handler = mux
	if corsEnabled {
		handler = cors.Default().Handler(handler)
	}
	return makeHTTPService(host, port, handler)
}

func makeRPCOverWebsocket(host string, port uint16, servers map[string]*jsonrpc.Server,
	log utils.StructuredLogger, metricsEnabled bool, corsEnabled bool,
) *httpService {
	var listener jsonrpc.NewRequestListener
	if metricsEnabled {
		listener = makeWSMetrics()
	}

	shutdown := make(chan struct{})

	mux := http.NewServeMux()
	for path, server := range servers {
		wsHandler := jsonrpc.NewWebsocket(server, shutdown, log)
		if listener != nil {
			wsHandler = wsHandler.WithListener(listener)
		}
		mux.Handle(path, exactPathServer(path, wsHandler))

		wsPrefixedPath := strings.TrimSuffix("/ws"+path, "/")
		mux.Handle(wsPrefixedPath, exactPathServer(wsPrefixedPath, wsHandler))
	}

	var handler http.Handler = mux
	if corsEnabled {
		handler = cors.Default().Handler(handler)
	}

	httpServ := makeHTTPService(host, port, handler)
	httpServ.registerOnShutdown(func() {
		close(shutdown)
	})
	return httpServ
}

func makeMetrics(host string, port uint16) *httpService {
	return makeHTTPService(host, port,
		promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{Registry: prometheus.DefaultRegisterer}))
}

// Create a new service that updates the log level and timeouts settings.
func makeHTTPUpdateService(host string, port uint16, logLevel *utils.LogLevel, feederClient *feeder.Client) *httpService {
	mux := http.NewServeMux()
	mux.HandleFunc("/log/level", func(w http.ResponseWriter, r *http.Request) {
		utils.HTTPLogSettings(w, r, logLevel)
	})
	mux.HandleFunc("/feeder/timeouts", func(w http.ResponseWriter, r *http.Request) {
		feeder.HTTPTimeoutsSettings(w, r, feederClient)
	})
	var handler http.Handler = mux
	return makeHTTPService(host, port, handler)
}

type grpcService struct {
	srv  *grpc.Server
	host string
	port uint16
}

func (g *grpcService) Run(ctx context.Context) error {
	errCh := make(chan error)
	defer close(errCh)

	portStr := strconv.FormatUint(uint64(g.port), 10)
	addr := net.JoinHostPort(g.host, portStr)

	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on addr %s: %w", addr, err)
	}

	var wg conc.WaitGroup
	defer wg.Wait()
	wg.Go(func() {
		if err := g.srv.Serve(l); err != nil {
			errCh <- err
		}
	})

	select {
	case <-ctx.Done():
		g.srv.Stop()
		return nil
	case err := <-errCh:
		return err
	}
}

func makeGRPC(host string, port uint16, database db.KeyValueStore, version string) *grpcService {
	srv := grpc.NewServer()
	gen.RegisterKVServer(srv, junogrpc.New(database, version))
	return &grpcService{
		srv:  srv,
		host: host,
		port: port,
	}
}

func makePPROF(host string, port uint16) *httpService {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return makeHTTPService(host, port, mux)
}

type readinessHandlers struct {
	bcReader                blockchain.Reader
	syncReader              sync.Reader
	readinessBlockTolerance uint
}

func NewReadinessHandlers(bcReader blockchain.Reader, syncReader sync.Reader, readinessBlockTolerance uint) *readinessHandlers {
	return &readinessHandlers{
		bcReader:                bcReader,
		syncReader:              syncReader,
		readinessBlockTolerance: readinessBlockTolerance,
	}
}

func (h *readinessHandlers) HandleReadySync(w http.ResponseWriter, r *http.Request) {
	if !h.isSynced() {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Node not synced yet.")) //nolint:errcheck
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *readinessHandlers) isSynced() bool {
	head, err := h.bcReader.HeadsHeader()
	if err != nil {
		return false
	}
	highestBlockHeader := h.syncReader.HighestBlockHeader()
	if highestBlockHeader == nil {
		return false
	}

	if head.Number > highestBlockHeader.Number {
		return false
	}

	return head.Number+uint64(h.readinessBlockTolerance) >= highestBlockHeader.Number
}

func (h *readinessHandlers) HandleLive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
