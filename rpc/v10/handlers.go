package rpcv10

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	stdsync "sync"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/l1/contract"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/sourcegraph/conc"
)

type Handler struct {
	bcReader      blockchain.Reader
	syncReader    sync.Reader
	gatewayClient rpccore.Gateway
	feederClient  *feeder.Client
	vm            vm.VM
	log           utils.Logger
	memPool       mempool.Pool

	newHeads      *feed.Feed[*core.Block]
	reorgs        *feed.Feed[*sync.ReorgBlockRange]
	pendingData   *feed.Feed[core.PendingData]
	l1Heads       *feed.Feed[*core.L1Head]
	preLatestFeed *feed.Feed[*core.PreLatest]

	idgen         func() string
	subscriptions stdsync.Map // map[string]*subscription

	// todo(rdr): why do we have the `TraceCacheKey` type and why it feels uncomfortable
	// to use. It makes no sense, why not use `Felt` or `Hash` directly?
	blockTraceCache *lru.Cache[rpccore.TraceCacheKey, []TracedBlockTransaction]
	// todo(rdr): Can this cache be genericified and can it be applied to the `blockTraceCache`
	submittedTransactionsCache *rpccore.TransactionCache

	filterLimit  uint
	callMaxSteps uint64
	callMaxGas   uint64

	l1Client        rpccore.L1Client
	coreContractABI abi.ABI
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
	logger utils.Logger,
) *Handler {
	contractABI, err := abi.JSON(strings.NewReader(contract.StarknetMetaData.ABI))
	if err != nil {
		logger.Fatalf("Failed to parse ABI: %v", err)
	}
	return &Handler{
		bcReader:   bcReader,
		syncReader: syncReader,
		log:        logger,
		vm:         virtualMachine,
		idgen: func() string {
			var n uint64
			for err := binary.Read(rand.Reader, binary.LittleEndian, &n); err != nil; {
			}
			return fmt.Sprintf("%d", n)
		},
		newHeads:      feed.New[*core.Block](),
		reorgs:        feed.New[*sync.ReorgBlockRange](),
		pendingData:   feed.New[core.PendingData](),
		l1Heads:       feed.New[*core.L1Head](),
		preLatestFeed: feed.New[*core.PreLatest](),

		blockTraceCache: lru.NewCache[
			rpccore.TraceCacheKey,
			[]TracedBlockTransaction,
		](rpccore.TraceCacheSize),
		filterLimit:     math.MaxUint,
		coreContractABI: contractABI,
	}
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

func (h *Handler) WithFeeder(feederClient *feeder.Client) *Handler {
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

// Currently only used for testing
func (h *Handler) Run(ctx context.Context) error {
	newHeadsSub := h.syncReader.SubscribeNewHeads().Subscription
	reorgsSub := h.syncReader.SubscribeReorg().Subscription
	pendingData := h.syncReader.SubscribePendingData().Subscription
	l1HeadsSub := h.bcReader.SubscribeL1Head().Subscription
	preLatestSub := h.syncReader.SubscribePreLatest().Subscription
	defer newHeadsSub.Unsubscribe()
	defer reorgsSub.Unsubscribe()
	defer pendingData.Unsubscribe()
	defer l1HeadsSub.Unsubscribe()
	defer preLatestSub.Unsubscribe()
	feed.Tee(newHeadsSub, h.newHeads)
	feed.Tee(reorgsSub, h.reorgs)
	feed.Tee(pendingData, h.pendingData)
	feed.Tee(l1HeadsSub, h.l1Heads)
	feed.Tee(preLatestSub, h.preLatestFeed)

	<-ctx.Done()
	h.subscriptions.Range(func(key, value any) bool {
		sub := value.(*subscription)
		sub.wg.Wait()
		return true
	})
	return nil
}

func (h *Handler) SpecVersion() (string, *jsonrpc.Error) {
	return "0.10.0-rc2", nil
}
