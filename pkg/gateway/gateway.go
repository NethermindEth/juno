package gateway

import (
	"context"
	"net/http"

	. "github.com/NethermindEth/juno/internal/log"
)

type Gateway struct{ Server *http.Server }

func New(addr string) *Gateway {
	srv := &http.Server{
		Addr:    addr,
		Handler: routes(),
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
			Logger.Errorw("error while shutting down", "error", err.Error())
			return
		}
	default:
	}
}
