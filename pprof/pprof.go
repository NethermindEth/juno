package pprof

import (
	"context"
	"net/http"
	// #nosec G108
	_ "net/http/pprof"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
)

var _ service.Service = (*Profiler)(nil)

type Profiler struct {
	log    utils.SimpleLogger
	server *http.Server
}

func New(port uint16, log utils.SimpleLogger) *Profiler {
	server := &http.Server{
		Addr:              "0.0.0.0:" + strconv.Itoa(int(port)),
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Profiler{
		server: server,
		log:    log,
	}
}

func (p *Profiler) Run(ctx context.Context) error {
	go func() {
		p.log.Infow("Starting pprof...", "address", p.server.Addr)
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.log.Errorw("Pprof server error", "err", err)
		}
	}()

	<-ctx.Done()
	return p.server.Shutdown(context.Background())
}
