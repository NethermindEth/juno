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
	log           utils.Logger

	version  string
	newHeads *feed.Feed[*core.Block]

	idgen         func() uint64
	subscriptions stdsync.Map // map[uint64]*subscription

	blockTraceCache            *lru.Cache[traceCacheKey, []TracedBlockTransaction]
	submittedTransactionsCache *rpccore.SubmittedTransactionsCache

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
		newHeads: feed.New[*core.Block](),

		blockTraceCache: lru.NewCache[traceCacheKey, []TracedBlockTransaction](traceCacheSize),
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

func (h *Handler) WithSubmittedTransactionsCache(cache *rpccore.SubmittedTransactionsCache) *Handler {
	h.submittedTransactionsCache = cache
	return h
}

func (h *Handler) Version() (string, *jsonrpc.Error) {
	return h.version, nil
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

func (h *Handler) PendingData() (*sync.Pending, error) {
	pending, err := h.syncReader.PendingData()
	if err != nil {
		return nil, err
	}
	// If IsPending == true, network is still polling pending block and running on < 0.14.0
	if pending.IsPending {
		return &sync.Pending{
			Block:       pending.Block,
			NewClasses:  pending.NewClasses,
			StateUpdate: pending.StateUpdate,
		}, nil
	} else {
		// If IsPending == false, network is still polling pre_confirmed block and running on >= 0.14.0
		// pendingID == latest
		latestB, err := h.bcReader.Head()
		if err != nil {
			return nil, err
		}

		stateUpdate, err := h.bcReader.StateUpdateByNumber(latestB.Number)
		if err != nil {
			return nil, err
		}
		reader, _, err := h.bcReader.StateAtBlockNumber(latestB.Number)
		if err != nil {
			return nil, err
		}

		newClasses := make(map[felt.Felt]core.Class, 0)
		for classHash := range stateUpdate.StateDiff.DeclaredV1Classes {
			declaredClass, err := reader.Class(&classHash)
			if err != nil {
				return nil, err
			}
			newClasses[classHash] = declaredClass.Class
		}

		for _, classHash := range stateUpdate.StateDiff.DeclaredV0Classes {
			declaredClass, err := reader.Class(classHash)
			if err != nil {
				return nil, err
			}
			newClasses[*classHash] = declaredClass.Class
		}

		return &sync.Pending{
			Block:       latestB,
			StateUpdate: stateUpdate,
			NewClasses:  newClasses,
		}, nil
	}
}

func (h *Handler) PendingBlock() *core.Block {
	pending, err := h.PendingData()
	if err != nil {
		return nil
	}
	return pending.Block
}
