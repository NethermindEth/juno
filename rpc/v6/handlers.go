package rpcv6

import (
	"math"

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
	"github.com/hashicorp/go-set/v2"
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
	log           utils.Logger

	version                    string
	forceFeederTracesForBlocks *set.Set[uint64]

	newHeads        *feed.Feed[*core.Header]
	blockTraceCache *lru.Cache[traceCacheKey, []TracedBlockTransaction]

	filterLimit  uint
	callMaxSteps uint64
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, virtualMachine vm.VM, version string, network *utils.Network,
	logger utils.Logger,
) *Handler {
	return &Handler{
		bcReader:                   bcReader,
		syncReader:                 syncReader,
		log:                        logger,
		vm:                         virtualMachine,
		version:                    version,
		forceFeederTracesForBlocks: set.From(network.BlockHashMetaInfo.ForceFetchingTracesForBlocks),
		newHeads:                   feed.New[*core.Header](),

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
