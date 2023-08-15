package node

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sourcegraph/conc"
	"google.golang.org/grpc"
)

type httpService struct {
	srv      *http.Server
	listener net.Listener
}

var _ service.Service = (*httpService)(nil)

func (h *httpService) Run(ctx context.Context) error {
	errCh := make(chan error)
	defer close(errCh)

	var wg conc.WaitGroup
	defer wg.Wait()
	wg.Go(func() {
		if err := h.srv.Serve(h.listener); !errors.Is(err, http.ErrServerClosed) {
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

func makeRPCOverHTTP(listener net.Listener, jsonrpcServer *jsonrpc.Server, log utils.SimpleLogger) (*httpService, error) {
	httpHandler := jsonrpc.NewHTTP(jsonrpcServer, log)
	mux := http.NewServeMux()
	mux.Handle("/", httpHandler)
	mux.Handle("/v0_4", httpHandler)
	return &httpService{
		srv: &http.Server{
			Addr:    listener.Addr().String(),
			Handler: mux,
			// ReadTimeout also sets ReadHeaderTimeout and IdleTimeout.
			ReadTimeout: 30 * time.Second,
		},
		listener: listener,
	}, nil
}

func makeRPCOverWebsocket(listener net.Listener, jsonrpcServer *jsonrpc.Server, log utils.SimpleLogger) (*httpService, error) {
	wsHandler := jsonrpc.NewWebsocket(jsonrpcServer, log)
	mux := http.NewServeMux()
	mux.Handle("/", wsHandler)
	mux.Handle("/v0_4", wsHandler)
	return &httpService{
		srv: &http.Server{
			Addr:    listener.Addr().String(),
			Handler: mux,
			// ReadTimeout also sets ReadHeaderTimeout and IdleTimeout.
			ReadTimeout: 30 * time.Second,
		},
		listener: listener,
	}, nil
}

func makeMetrics(listener net.Listener) *httpService {
	return &httpService{
		srv: &http.Server{
			Addr:    listener.Addr().String(),
			Handler: promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{Registry: prometheus.DefaultRegisterer}),
			// ReadTimeout also sets ReadHeaderTimeout and IdleTimeout.
			ReadTimeout: 30 * time.Second,
		},
		listener: listener,
	}
}

func makeGRPC(listener net.Listener, database db.DB, version string) *httpService {
	grpcHandler := grpc.NewServer()
	gen.RegisterKVServer(grpcHandler, junogrpc.New(database, version))
	return &httpService{
		srv: &http.Server{
			Addr:    listener.Addr().String(),
			Handler: grpcHandler,
			// ReadTimeout also sets ReadHeaderTimeout and IdleTimeout.
			ReadTimeout: 30 * time.Second,
		},
		listener: listener,
	}
}

func makePPROF(listener net.Listener) *httpService {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return &httpService{
		srv: &http.Server{
			Addr:    listener.Addr().String(),
			Handler: mux,
			// ReadTimeout also sets ReadHeaderTimeout and IdleTimeout.
			ReadTimeout: 30 * time.Second,
		},
		listener: listener,
	}
}

func methods(h *rpc.Handler) []jsonrpc.Method { //nolint: funlen
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.BlockHashAndNumber,
		},
		{
			Name:    "starknet_getBlockWithTxHashes",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithTxHashes,
		},
		{
			Name:    "starknet_getBlockWithTxs",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithTxs,
		},
		{
			Name:    "starknet_getTransactionByHash",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionReceiptByHash,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.ClassAt,
		},
		{
			Name:    "starknet_addInvokeTransaction",
			Params:  []jsonrpc.Parameter{{Name: "invoke_transaction"}},
			Handler: h.AddTransaction,
		},
		{
			Name:    "starknet_addDeployAccountTransaction",
			Params:  []jsonrpc.Parameter{{Name: "deploy_account_transaction"}},
			Handler: h.AddTransaction,
		},
		{
			Name:    "starknet_addDeclareTransaction",
			Params:  []jsonrpc.Parameter{{Name: "declare_transaction"}},
			Handler: h.AddTransaction,
		},
		{
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.Events,
		},
		{
			Name:    "starknet_pendingTransactions",
			Handler: h.PendingTransactions,
		},
		{
			Name:    "juno_version",
			Handler: h.Version,
		},
		{
			Name:    "juno_getTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionStatus,
		},
		{
			Name:    "starknet_call",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: h.Call,
		},
		{
			Name:    "starknet_estimateFee",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: h.EstimateFee,
		},
		{
			Name:    "starknet_estimateMessageFee",
			Params:  []jsonrpc.Parameter{{Name: "message"}, {Name: "block_id"}},
			Handler: h.EstimateMessageFee,
		},
		{
			Name:    "starknet_traceTransaction",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TraceTransaction,
		},
		{
			Name:    "starknet_simulateTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "transactions"}, {Name: "simulation_flags"}},
			Handler: h.SimulateTransactions,
		},
		{
			Name:    "starknet_traceBlockTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_hash"}},
			Handler: h.TraceBlockTransactions,
		},
	}
}
