package rpc

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/jsonrpc"
	. "github.com/NethermindEth/juno/pkg/jsonrpc/providers/http"
)

type HttpRpc struct {
	server   *http.Server
	provider *HttpProvider
	pattern  string
}

func NewHttpRpc(addr, pattern string, rpc *jsonrpc.JsonRpc) (*HttpRpc, error) {
	httpRpc := new(HttpRpc)
	httpRpc.provider = NewHttpProvider(rpc)
	httpRpc.pattern = pattern
	mux := http.NewServeMux()
	mux.Handle(pattern, httpRpc.provider)
	httpRpc.server = &http.Server{Addr: addr, Handler: mux}
	return httpRpc, nil
}

func (h *HttpRpc) ListenAndServe(errCh chan<- error) {
	// notest
	go h.listenAndServe(errCh)
}

func (h *HttpRpc) listenAndServe(errCh chan<- error) {
	// notest
	ip, err := getIpAddress(NetworkHandler{
		GetInterfaces: net.Interfaces,
		GetAddressFromInterface: func(p net.Interface) ([]net.Addr, error) {
			return p.Addrs()
		},
	})
	if err != nil {
		errCh <- err
		return
	}
	Logger.With("Running at", "http://"+ip.String()+h.server.Addr+h.pattern).Info("Listening for JSON-RPC connections...")

	// Since ListenAndServe always returns an error we need to ensure that there
	// is no write to a closed channel. Therefore, we check for ErrServerClosed
	// since that is only returned after ShutDown or Closed is called. Hence, no
	// write to the channel is required. Otherwise, any other error is written
	// which will cause the program to exit.
	if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errCh <- errors.New("Failed to ListenAndServe on Metrics server: " + err.Error())
	}
	close(errCh)
}

func (h *HttpRpc) Close(timeout time.Duration) error {
	Logger.Info("Shutting down JSON-RPC server...")
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	return h.server.Shutdown(ctx)
}

type NetworkHandler struct {
	GetInterfaces           func() ([]net.Interface, error)
	GetAddressFromInterface func(p net.Interface) ([]net.Addr, error)
}

func getIpAddress(networkHandler NetworkHandler) (net.IP, error) {
	interfaces, err := networkHandler.GetInterfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range interfaces {
		addresses, err := networkHandler.GetAddressFromInterface(i)
		if err != nil {
			return nil, err
		}
		for _, addr := range addresses {
			if len(addr.String()) < 4 {
				continue
			}

			ip := net.ParseIP(addr.String()[0 : len(addr.String())-3])
			if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
				return ip, nil
			}
		}
	}

	return nil, errors.New("network: ip not found")
}
