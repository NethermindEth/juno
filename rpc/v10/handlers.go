package rpcv10

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	stdsync "sync"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/broadcaster"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/utils/lru"
	"github.com/NethermindEth/juno/vm"
	"github.com/sourcegraph/conc"
)

type Handler struct {
	bcReader      blockchain.Reader
	syncReader    sync.Reader
	gatewayClient rpccore.Gateway
	feederClient  feeder.Reader
	vm            vm.VM
	logger        log.Logger
	memPool       mempool.Pool

	// One Subscribable per stream, per handler; each WebSocket client opens its own
	// Subscription off them.
	newHeadsSubscribable            broadcaster.Subscribable[*core.Block]
	reorgSubscribable               broadcaster.Subscribable[*sync.ReorgBlockRange]
	preConfirmedSubscribable        broadcaster.Subscribable[*pending.PreConfirmed]
	l1HeadSubscribable              broadcaster.Subscribable[*core.L1Head]
	receivedTransactionSubscribable broadcaster.Subscribable[core.Transaction]
	receivedTransactionPub          broadcaster.Publisher[core.Transaction]

	idgen         func() string
	subscriptions stdsync.Map // map[string]*subscription

	blockTraceCache *lru.Cache[felt.Felt, TraceBlockTransactionsResponse]
	// todo(rdr): Can this cache be genericified and can it be applied to the `blockTraceCache`
	submittedTransactionsCache *rpccore.TransactionCache

	filterLimit  uint
	callMaxSteps uint64
	callMaxGas   uint64

	compiler compiler.Compiler

	l1Client rpccore.L1Client
}

type subscription struct {
	cancel func()
	wg     conc.WaitGroup
	conn   jsonrpc.Conn
}

func New(
	bcReader blockchain.Reader,
	syncReader sync.Reader,
	virtualMachine vm.VM,
	logger log.Logger,
) *Handler {
	h := &Handler{
		bcReader:   bcReader,
		syncReader: syncReader,
		logger:     logger,
		vm:         virtualMachine,
		idgen: func() string {
			var n uint64
			for err := binary.Read(rand.Reader, binary.LittleEndian, &n); err != nil; {
			}
			return fmt.Sprintf("%d", n)
		},

		blockTraceCache: lru.New[
			felt.Felt,
			TraceBlockTransactionsResponse,
		](rpccore.TraceCacheSize),
		filterLimit: math.MaxUint,
	}
	return h
}

// initSubscribables binds one Subscribable per stream to its lag policy. Called from
// Run() so New() stays free of side effects on its readers.
func (h *Handler) initSubscribables() {
	h.newHeadsSubscribable = h.syncReader.NewHeadsSource().NewSubscribable(
		broadcaster.LagPolicyBlockReplay(h.bcReader, h.logger),
	)
	h.reorgSubscribable = h.syncReader.ReorgsSource().NewSubscribable(
		broadcaster.LagPolicyLog[*sync.ReorgBlockRange](h.logger),
	)
	h.preConfirmedSubscribable = h.syncReader.PreConfirmedSource().NewSubscribable(
		broadcaster.LagPolicyLog[*pending.PreConfirmed](h.logger),
	)
	h.l1HeadSubscribable = h.bcReader.L1HeadsSource().NewSubscribable(
		broadcaster.LagPolicyDrop[*core.L1Head],
	)
}

func (h *Handler) WithCompiler(compiler compiler.Compiler) *Handler {
	h.compiler = compiler
	return h
}

func (h *Handler) WithMempool(memPool mempool.Pool) *Handler {
	h.memPool = memPool
	return h
}

// WithFilterLimit sets the maximum number of blocks to scan in a single call for event filtering.
func (h *Handler) WithFilterLimit(limit uint) *Handler {
	h.filterLimit = limit
	return h
}

func (h *Handler) WithL1Client(l1Client rpccore.L1Client) *Handler {
	h.l1Client = l1Client
	return h
}

func (h *Handler) WithCallMaxSteps(maxSteps uint64) *Handler {
	h.callMaxSteps = maxSteps
	return h
}

func (h *Handler) WithCallMaxGas(maxGas uint64) *Handler {
	h.callMaxGas = maxGas
	return h
}

func (h *Handler) WithIDGen(idgen func() string) *Handler {
	h.idgen = idgen
	return h
}

func (h *Handler) WithFeeder(feederClient feeder.Reader) *Handler {
	h.feederClient = feederClient
	return h
}

func (h *Handler) WithGateway(gatewayClient rpccore.Gateway) *Handler {
	h.gatewayClient = gatewayClient
	return h
}

func (h *Handler) WithSubmittedTransactionsCache(cache *rpccore.TransactionCache) *Handler {
	h.submittedTransactionsCache = cache
	return h
}

func (h *Handler) WithReceivedTransactionHub(
	hub broadcaster.BroadcastHub[core.Transaction],
) *Handler {
	h.receivedTransactionPub = hub.NewPublisher()
	h.receivedTransactionSubscribable = hub.NewSubscribable(
		broadcaster.LagPolicyLog[core.Transaction](h.logger),
	)
	return h
}

func (h *Handler) Run(ctx context.Context) error {
	h.initSubscribables()
	<-ctx.Done()
	h.subscriptions.Range(func(key, value any) bool {
		sub := value.(*subscription)
		sub.wg.Wait()
		return true
	})
	return nil
}

func (h *Handler) SpecVersion() (string, *jsonrpc.Error) {
	return "0.10.3", nil
}
