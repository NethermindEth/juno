package rpcv7

import (
	"log"
	"math"
	"strings"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/l1/contract"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/lru"
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
	bcReader        blockchain.Reader
	syncReader      sync.Reader
	gatewayClient   rpccore.Gateway
	feederClient    *feeder.Client
	vm              vm.VM
	log             utils.Logger
	version         string
	blockTraceCache *lru.Cache[traceCacheKey, []TracedBlockTransaction]
	filterLimit     uint
	callMaxSteps    uint64
	l1Client        rpccore.L1Client
	coreContractABI abi.ABI
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, virtualMachine vm.VM, version string,
	logger utils.Logger,
) *Handler {
	contractABI, err := abi.JSON(strings.NewReader(contract.StarknetMetaData.ABI))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
	}
	return &Handler{
		bcReader:        bcReader,
		syncReader:      syncReader,
		log:             logger,
		vm:              virtualMachine,
		version:         version,
		blockTraceCache: lru.NewCache[traceCacheKey, []TracedBlockTransaction](traceCacheSize),
		filterLimit:     math.MaxUint,
		coreContractABI: contractABI,
	}
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
	return "0.7.1", nil
}
