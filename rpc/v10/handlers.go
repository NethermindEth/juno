package rpcv10

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	stdsync "sync"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
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

	newHeads                *feed.Feed[*core.Block]
	reorgs                  *feed.Feed[*sync.ReorgBlockRange]
	preConfirmedFeed        *feed.Feed[*pending.PreConfirmed]
	l1Heads                 *feed.Feed[*core.L1Head]
	receivedTransactionFeed *feed.Feed[core.Transaction]

	idgen         func() string
	subscriptions stdsync.Map // map[string]*subscription

	blockTraceCache *blockTraceCache
	// Serializes snapshot acquisition and retirement. Execution runs outside these locks.
	preConfirmedTraceMu     stdsync.Mutex
	preConfirmedTraceCaches map[uint64]*preConfirmedTraceCache
	// submittedTransactionsCache is a TTL membership set, unlike the coordinated block trace LRU.
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
	return &Handler{
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
		newHeads:         feed.New[*core.Block](),
		reorgs:           feed.New[*sync.ReorgBlockRange](),
		preConfirmedFeed: feed.New[*pending.PreConfirmed](),
		l1Heads:          feed.New[*core.L1Head](),

		blockTraceCache:         newBlockTraceCache(rpccore.TraceCacheSize),
		preConfirmedTraceCaches: make(map[uint64]*preConfirmedTraceCache),
		filterLimit:             math.MaxUint,
	}
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

func (h *Handler) WithReceivedTransactionFeed(feed *feed.Feed[core.Transaction]) *Handler {
	h.receivedTransactionFeed = feed
	return h
}

// Run forwards synchronization events and retires obsolete preconfirmed trace contexts.
func (h *Handler) Run(ctx context.Context) error {
	newHeadsSub := h.syncReader.SubscribeNewHeads().Subscription
	reorgsSub := h.syncReader.SubscribeReorg().Subscription
	preConfirmedSub := h.syncReader.SubscribePreConfirmed().Subscription
	l1HeadsSub := h.bcReader.SubscribeL1Head().Subscription
	defer newHeadsSub.Unsubscribe()
	defer reorgsSub.Unsubscribe()
	defer preConfirmedSub.Unsubscribe()
	defer l1HeadsSub.Unsubscribe()
	feed.Tee(l1HeadsSub, h.l1Heads)

updates:
	for {
		select {
		case <-ctx.Done():
			break updates
		case head := <-newHeadsSub.Recv():
			h.refreshPreConfirmedTraceCaches()
			h.newHeads.Send(head)
		case reorg := <-reorgsSub.Recv():
			h.refreshPreConfirmedTraceCaches()
			h.reorgs.Send(reorg)
		case preConfirmed := <-preConfirmedSub.Recv():
			h.refreshPreConfirmedTraceCaches()
			h.preConfirmedFeed.Send(preConfirmed)
		}
	}
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
