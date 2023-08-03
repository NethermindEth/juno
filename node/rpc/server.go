package rpcserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/validator"
	"github.com/sourcegraph/conc"
)

type Server struct {
	listener net.Listener
	mux      *http.ServeMux
}

var _ service.Service = (*Server)(nil)

func New(listener net.Listener, rpcHandler *rpc.Handler, log utils.SimpleLogger) (*Server, error) {
	methods := methods(rpcHandler)

	jsonrpcServer := jsonrpc.NewServer(log).WithValidator(validator.Validator())
	for _, method := range methods {
		if err := jsonrpcServer.RegisterMethod(method); err != nil {
			return nil, err
		}
	}

	httpHandler := jsonrpc.NewHTTP(jsonrpcServer, log)
	wsHandler := jsonrpc.NewWebsocket(jsonrpcServer, log)

	rpcHTTPHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Upgrade") == "websocket" {
			wsHandler.ServeHTTP(w, req)
			return
		}
		httpHandler.ServeHTTP(w, req)
	})
	mux := http.NewServeMux()
	mux.Handle("/", rpcHTTPHandler)
	mux.Handle("/v0_4", rpcHTTPHandler)

	return &Server{
		listener: listener,
		mux:      mux,
	}, nil
}

func (srv *Server) Run(ctx context.Context) error {
	httpSrv := &http.Server{
		Addr:    srv.listener.Addr().String(),
		Handler: srv.mux,
		// ReadTimeout also sets ReadHeaderTimeout and IdleTimeout.
		ReadTimeout: 30 * time.Second,
	}

	errCh := make(chan error)
	defer close(errCh)

	var wg conc.WaitGroup
	defer wg.Wait()
	wg.Go(func() {
		if err := httpSrv.Serve(srv.listener); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	})

	select {
	case <-ctx.Done():
		return httpSrv.Shutdown(context.Background())
	case err := <-errCh:
		return err
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
	}
}
