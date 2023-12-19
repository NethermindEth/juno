package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/NethermindEth/juno/db"
	junogrpc "github.com/NethermindEth/juno/grpc"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/sourcegraph/conc"
	"google.golang.org/grpc"
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

func makeHTTPService(host string, port uint16, handler http.Handler) *httpService {
	portStr := strconv.FormatUint(uint64(port), 10)
	return &httpService{
		srv: &http.Server{
			Addr:    net.JoinHostPort(host, portStr),
			Handler: handler,
			// ReadTimeout also sets ReadHeaderTimeout and IdleTimeout.
			ReadTimeout: 30 * time.Second,
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
	log utils.SimpleLogger, metricsEnabled bool,
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
	return makeHTTPService(host, port, cors.Default().Handler(mux))
}

func makeRPCOverWebsocket(host string, port uint16, servers map[string]*jsonrpc.Server,
	log utils.SimpleLogger, metricsEnabled bool,
) *httpService {
	var listener jsonrpc.NewRequestListener
	if metricsEnabled {
		listener = makeWSMetrics()
	}

	mux := http.NewServeMux()
	for path, server := range servers {
		wsHandler := jsonrpc.NewWebsocket(server, log)
		if listener != nil {
			wsHandler = wsHandler.WithListener(listener)
		}
		mux.Handle(path, exactPathServer(path, wsHandler))
		wsPrefixedPath := strings.TrimSuffix("/ws"+path, "/")
		mux.Handle(wsPrefixedPath, exactPathServer(wsPrefixedPath, wsHandler))
	}
	return makeHTTPService(host, port, cors.Default().Handler(mux))
}

type ipcService struct {
	handlers []*jsonrpc.Ipc
}

var _ service.Service = (*ipcService)(nil)

// Run is a method of the ipcService struct that starts the execution of IPC handlers in parallel.
// It takes context as a parameter to control the execution and returns an error if any handler fails.
func (ipc *ipcService) Run(ctx context.Context) error {
	var wg conc.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancelCause(ctx)

	// Iterate over all registered handlers and execute them concurrently.
	for i := range ipc.handlers {
		handler := ipc.handlers[i]
		wg.Go(func() {
			cancel(handler.Run(ctx))
		})
	}

	<-ctx.Done()
	if err := context.Cause(ctx); err == ctx.Err() {
		// If the context is explicitly canceled or expired, don't return an error.
		return nil
	} else {
		return err
	}
}

func makeRPCOverIpc(dir string, servers map[string]*jsonrpc.Server,
	log utils.SimpleLogger, metricsEnabled bool,
) (*ipcService, error) {
	var listener jsonrpc.NewRequestListener
	if metricsEnabled {
		listener = makeIpcMetrics()
	}

	// Sorted buffer for consistent initialization.
	paths := make([]string, 0, len(servers))
	for path := range servers {
		paths = append(paths, path)
	}
	slices.Sort(paths)

	srv := &ipcService{handlers: make([]*jsonrpc.Ipc, 0)}
	for _, path := range paths {
		server := servers[path]
		trimmed, ok := strings.CutPrefix(path, "/")
		if !ok {
			continue
		}
		endpoint := filepath.Join(dir, fmt.Sprintf("juno_%s.ipc", trimmed))
		if trimmed == "" {
			endpoint = filepath.Join(dir, "juno.ipc")
		}
		ipcHandler, err := jsonrpc.NewIpc(server, endpoint, log)
		if err != nil {
			// ugly hack to not leave stale sockets.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			srv.Run(ctx) //nolint:errcheck
			return nil, err
		}
		if listener != nil {
			ipcHandler = ipcHandler.WithListener(listener)
		}
		srv.handlers = append(srv.handlers, ipcHandler)
	}
	return srv, nil
}

func makeMetrics(host string, port uint16) *httpService {
	return makeHTTPService(host, port,
		promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{Registry: prometheus.DefaultRegisterer}))
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
	l, err := net.Listen("tcp", addr)
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

func makeGRPC(host string, port uint16, database db.DB, version string) *grpcService {
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
