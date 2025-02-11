package rpcv6

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
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/sourcegraph/conc"
)

type traceCacheKey struct {
	blockHash    felt.Felt
	v0_6Response bool
}

type Handler struct {
	bcReader      blockchain.Reader
	syncReader    sync.Reader
	gatewayClient rpccore.Gateway
	feederClient  *feeder.Client
	vm            vm.VM
	idgen         func() uint64
	subscriptions stdsync.Map // map[uint64]*subscription
	newHeads      *feed.Feed[*core.Header]

	log             utils.Logger
	version         string
	blockTraceCache *lru.Cache[traceCacheKey, []TracedBlockTransaction]

	filterLimit  uint
	callMaxSteps uint64
}

type subscription struct {
	cancel func()
	wg     conc.WaitGroup
	conn   jsonrpc.Conn
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, virtualMachine vm.VM, version string, network *utils.Network,
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
		version:  version,
		newHeads: feed.New[*core.Header](),

		blockTraceCache: lru.NewCache[traceCacheKey, []TracedBlockTransaction](rpccore.TraceCacheSize),
		filterLimit:     math.MaxUint,
	}
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

func (h *Handler) Version() (string, *jsonrpc.Error) {
	return h.version, nil
}

func (h *Handler) SpecVersion() (string, *jsonrpc.Error) {
	return "0.6.0", nil
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
