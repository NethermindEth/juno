package rpcv7

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"math"
	stdsync "sync"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/sourcegraph/conc"
)

const (
	maxBlocksBack      = 1024
	maxEventChunkSize  = 10240
	maxEventFilterKeys = 1024
	traceCacheSize     = 128
	throttledVMErr     = "VM throughput limit reached"
)

type traceCacheKey struct {
	blockHash felt.Felt
}

type Handler struct {
	bcReader      blockchain.Reader
	syncReader    sync.Reader
	gatewayClient rpccore.Gateway
	feederClient  *feeder.Client
	vm            vm.VM
	compiler      compiler.Compiler
	log           utils.Logger

	newHeads *feed.Feed[*core.Block]

	idgen         func() uint64
	subscriptions stdsync.Map // map[uint64]*subscription

	blockTraceCache            *lru.Cache[traceCacheKey, []TracedBlockTransaction]
	submittedTransactionsCache *rpccore.TransactionCache

	filterLimit  uint
	callMaxSteps uint64
	callMaxGas   uint64
}

type subscription struct {
	cancel func()
	wg     conc.WaitGroup
	conn   jsonrpc.Conn
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, virtualMachine vm.VM, network *utils.Network,
	logger utils.Logger,
) *Handler {
	return &Handler{
		bcReader:   bcReader,
		syncReader: syncReader,
		log:        logger,
		vm:         virtualMachine,
		idgen: func() uint64 {
			var n uint64
			for err := binary.Read(rand.Reader, binary.LittleEndian, &n); err != nil; {
			}
			return n
		},
		newHeads: feed.New[*core.Block](),

		blockTraceCache: lru.NewCache[traceCacheKey, []TracedBlockTransaction](traceCacheSize),
		filterLimit:     math.MaxUint,
	}
}

func (h *Handler) WithCompiler(compiler compiler.Compiler) *Handler {
	h.compiler = compiler
	return h
}

// WithFilterLimit sets the maximum number of blocks to scan in a single call for event filtering.
func (h *Handler) WithFilterLimit(limit uint) *Handler {
	h.filterLimit = limit
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

func (h *Handler) WithIDGen(idgen func() uint64) *Handler {
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

func (h *Handler) SpecVersion() (string, *jsonrpc.Error) {
	return "0.7.1", nil
}

func (h *Handler) Run(ctx context.Context) error {
	newHeadsSub := h.syncReader.SubscribeNewHeads().Subscription
	defer newHeadsSub.Unsubscribe()
	feed.Tee(newHeadsSub, h.newHeads)
	<-ctx.Done()
	h.subscriptions.Range(func(key, value any) bool {
		sub := value.(*subscription)
		sub.wg.Wait()
		return true
	})
	return nil
}
