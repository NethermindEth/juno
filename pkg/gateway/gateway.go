package gateway

import (
	"context"
	"log"
	"net/http"
	"os"

	zap "github.com/NethermindEth/juno/internal/log"
)

// TODO: Substitute for zap.
var (
	logErr  = log.New(os.Stderr, "ERROR ", log.LstdFlags|log.Lshortfile)
	logInfo = log.New(os.Stdout, "INFO ", log.LstdFlags)
)

type Gateway struct{ Server *http.Server }

func New(addr string) *Gateway {
	srv := &http.Server{
		Addr: addr,
		// TODO: How to do this with zap?
		ErrorLog: logErr,
		Handler:  routes(),
	}
	return &Gateway{srv}
}

func (g *Gateway) Run() error {
	return g.Server.ListenAndServe()
}

func (g *Gateway) Shutdown(ctx context.Context) {
	// notest
	select {
	case <-ctx.Done():
		err := g.Server.Shutdown(ctx)
		if err != nil {
			zap.Logger.With("Error", err).Info("Exiting with error.")
			return
		}
	default:
	}
}
