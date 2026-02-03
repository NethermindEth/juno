package migration

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

const ShutdownTimeout = 5 * time.Second

// RunWithServer starts a status HTTP server and runs the provided function.
// The server exposes health check endpoints:
//   - /live: Always returns 200 OK (liveness probe)
//   - /ready: Returns 503 Service Unavailable with message "Database migration in progress."
//     (readiness probe - indicates operation is running)
//
// The server is started in a goroutine, the function is executed, and then
// the server is shut down. If the function returns an error, it is returned.
func RunWithServer(
	log utils.StructuredLogger,
	host string,
	port uint16,
	fn func() error,
) error {
	srv := startMigrationServer(log, host, port)
	defer closeMigrationServer(srv, log)

	return fn()
}

// startMigrationServer starts an HTTP server that provides health check endpoints.
// The server runs in a goroutine and should be closed using closeMigrationServer.
func startMigrationServer(log utils.StructuredLogger, host string, port uint16) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err := w.Write([]byte("Database migration in progress."))
		if err != nil {
			log.Error("Failed to write migration status response", zap.Error(err))
		}
	})

	portStr := strconv.FormatUint(uint64(port), 10)
	addr := net.JoinHostPort(host, portStr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		log.Debug("Starting migration status server", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Migration status server failed", zap.Error(err))
		}
	}()
	return srv
}

// closeMigrationServer gracefully shuts down the status HTTP server.
// Uses a timeout to ensure the server doesn't hang indefinitely.
func closeMigrationServer(srv *http.Server, log utils.StructuredLogger) {
	log.Debug("Shutting down migration status server...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Migration status server shutdown failed, forcing close", zap.Error(err))
		// Force close if graceful shutdown fails
		if closeErr := srv.Close(); closeErr != nil {
			log.Error("Migration status server force close failed", zap.Error(closeErr))
		}
	}
}
