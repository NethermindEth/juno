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

const (
	InterfaceWlan     = "wlan0"
	InterfaceEthernet = "eth0"
)

// A NetEnumerator enumerates local IP addresses.
type NetEnumerator struct {
	Interfaces     func() ([]net.Interface, error)
	InterfaceAddrs func(*net.Interface) ([]net.Addr, error)
}

// DefaultEnumerator returns a NetEnumerator that uses the default
// implementations from the net package.
func DefaultEnumerator() NetEnumerator {
	return NetEnumerator{
		Interfaces:     net.Interfaces,
		InterfaceAddrs: (*net.Interface).Addrs,
	}
}

type HttpRpc struct {
	server   *http.Server
	provider *HttpProvider
}

func NewHttpRpc(addr, pattern string, rpc *jsonrpc.JsonRpc) (*HttpRpc, error) {
	httpRpc := new(HttpRpc)
	httpRpc.provider = NewHttpProvider(rpc)
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
		GetAddrsFromInterface: func(p net.Interface) ([]net.Addr, error) {
			return p.Addrs()
		},
	})
	if err != nil {
		errCh <- err
		return
	}
	Logger.With("Running at", ip.String()+h.server.Addr).Info("Listening for JSON-RPC connections...")

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
	GetInterfaces         func() ([]net.Interface, error)
	GetAddrsFromInterface func(p net.Interface) ([]net.Addr, error)
}

func getIpAddress(networkHandler NetworkHandler) (net.IP, error) {
	ifaces, err := networkHandler.GetInterfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range ifaces {
		addresses, err := networkHandler.GetAddrsFromInterface(i)
		if err != nil {
			return nil, err
		}
		for _, addr := range addresses {
			// check the address type and if it is not a loopback the display it
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
