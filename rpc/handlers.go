package rpc

import (
	"context"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mempool"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"golang.org/x/sync/errgroup"
)

type Handler struct {
	rpcv8Handler  *rpcv8.Handler
	rpcv9Handler  *rpcv9.Handler
	rpcv10Handler *rpcv10.Handler
	version       string
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, virtualMachine vm.VM, version string,
	logger log.Logger, network *networks.Network,
) *Handler {
	handlerv8 := rpcv8.New(bcReader, syncReader, virtualMachine, logger)
	handlerv9 := rpcv9.New(bcReader, syncReader, virtualMachine, logger)
	handlerv10 := rpcv10.New(bcReader, syncReader, virtualMachine, logger)

	return &Handler{
		rpcv8Handler:  handlerv8,
		rpcv9Handler:  handlerv9,
		rpcv10Handler: handlerv10,
		version:       version,
	}
}

func (h *Handler) WithCompiler(compiler compiler.Compiler) *Handler {
	h.rpcv8Handler.WithCompiler(compiler)
	h.rpcv9Handler.WithCompiler(compiler)
	h.rpcv10Handler.WithCompiler(compiler)
	return h
}

// WithFilterLimit sets the maximum number of blocks to scan in a single call for event filtering.
func (h *Handler) WithFilterLimit(limit uint) *Handler {
	h.rpcv8Handler.WithFilterLimit(limit)
	h.rpcv9Handler.WithFilterLimit(limit)
	h.rpcv10Handler.WithFilterLimit(limit)
	return h
}

func (h *Handler) WithL1Client(l1Client rpccore.L1Client) *Handler {
	h.rpcv8Handler.WithL1Client(l1Client)
	h.rpcv9Handler.WithL1Client(l1Client)
	h.rpcv10Handler.WithL1Client(l1Client)
	return h
}

func (h *Handler) WithCallMaxSteps(maxSteps uint64) *Handler {
	h.rpcv8Handler.WithCallMaxSteps(maxSteps)
	h.rpcv9Handler.WithCallMaxSteps(maxSteps)
	h.rpcv10Handler.WithCallMaxSteps(maxSteps)
	return h
}

func (h *Handler) WithCallMaxGas(maxGas uint64) *Handler {
	h.rpcv8Handler.WithCallMaxGas(maxGas)
	h.rpcv9Handler.WithCallMaxGas(maxGas)
	h.rpcv10Handler.WithCallMaxGas(maxGas)
	return h
}

func (h *Handler) WithFeeder(feederClient feeder.Reader) *Handler {
	h.rpcv8Handler.WithFeeder(feederClient)
	h.rpcv9Handler.WithFeeder(feederClient)
	h.rpcv10Handler.WithFeeder(feederClient)
	return h
}

func (h *Handler) WithGateway(gatewayClient rpccore.Gateway) *Handler {
	h.rpcv8Handler.WithGateway(gatewayClient)
	h.rpcv9Handler.WithGateway(gatewayClient)
	h.rpcv10Handler.WithGateway(gatewayClient)
	return h
}

func (h *Handler) WithMempool(memPool mempool.Pool) *Handler {
	h.rpcv8Handler.WithMempool(memPool)
	h.rpcv9Handler.WithMempool(memPool)
	h.rpcv10Handler.WithMempool(memPool)
	return h
}

func (h *Handler) WithSubmittedTransactionsCache(cache *rpccore.TransactionCache) *Handler {
	h.rpcv8Handler.WithSubmittedTransactionsCache(cache)
	h.rpcv9Handler.WithSubmittedTransactionsCache(cache)
	h.rpcv10Handler.WithSubmittedTransactionsCache(cache)
	return h
}

func (h *Handler) WithReceivedTransactionFeed(feed *feed.Feed[core.Transaction]) *Handler {
	h.rpcv8Handler.WithReceivedTransactionFeed(feed)
	h.rpcv9Handler.WithReceivedTransactionFeed(feed)
	h.rpcv10Handler.WithReceivedTransactionFeed(feed)
	return h
}

func (h *Handler) Version() (string, *jsonrpc.Error) {
	return h.version, nil
}

func (h *Handler) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return h.rpcv8Handler.Run(ctx) })
	g.Go(func() error { return h.rpcv9Handler.Run(ctx) })
	g.Go(func() error { return h.rpcv10Handler.Run(ctx) })

	return g.Wait()
}

// junoVersionMethod returns the cross-version `juno_version` binding.
// Each MethodsV0_X appends this; the underlying version handlers don't
// register it themselves since it reads the wrapping rpc.Handler's
// build identifier.
func (h *Handler) junoVersionMethod() jsonrpc.Method {
	return jsonrpc.Register("juno_version",
		func(_ *jsonrpc.NoParams) (string, *jsonrpc.Error) { return h.Version() })
}

func (h *Handler) MethodsV0_10() ([]jsonrpc.Method, string) {
	methods := h.rpcv10Handler.RegisterMethods()
	methods = append(methods, h.junoVersionMethod())
	return methods, "/v0_10"
}

func (h *Handler) MethodsV0_9() ([]jsonrpc.Method, string) {
	methods := h.rpcv9Handler.RegisterMethods()
	methods = append(methods, h.junoVersionMethod())
	return methods, "/v0_9"
}

func (h *Handler) MethodsV0_8() ([]jsonrpc.Method, string) {
	methods := h.rpcv8Handler.RegisterMethods()
	methods = append(methods, h.junoVersionMethod())
	return methods, "/v0_8"
}
