package pprof

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/utils"
	// #nosec G108
	_ "net/http/pprof"
)

type Profiler struct {
	enabled bool
	log     utils.SimpleLogger
	server  *http.Server
}

func New(enabled bool, port uint16, log utils.SimpleLogger) *Profiler {
	server := &http.Server{
		Addr:              "localhost:" + strconv.Itoa(int(port)),
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Profiler{
		enabled: enabled,
		server:  server,
		log:     log,
	}
}

func (p *Profiler) Run(ctx context.Context) error {
	if !p.enabled {
		return nil
	}

	go func() {
		p.log.Infow("Starting pprof...", "address", p.server.Addr)
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.log.Errorw("Pprof server error", "err", err)
		}
	}()

	<-ctx.Done()
	p.log.Infow("Shutting down pprof...")
	return p.server.Shutdown(context.Background())
}
