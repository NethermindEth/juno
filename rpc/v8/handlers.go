package rpcv8

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

	version     string
	newHeads    *feed.Feed[*core.Block]
	reorgs      *feed.Feed[*sync.ReorgBlockRange]
	pendingData *feed.Feed[core.PendingData]
	l1Heads     *feed.Feed[*core.L1Head]

	idgen         func() string
	subscriptions stdsync.Map // map[string]*subscription

	blockTraceCache            *lru.Cache[rpccore.TraceCacheKey, []TracedBlockTransaction]
	submittedTransactionsCache *rpccore.SubmittedTransactionsCache

	filterLimit  uint
	callMaxSteps uint64

	l1Client        rpccore.L1Client
	coreContractABI abi.ABI
}

type subscription struct {
	cancel func()
	wg     conc.WaitGroup
	conn   jsonrpc.Conn
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, virtualMachine vm.VM,
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
		newHeads:    feed.New[*core.Block](),
		reorgs:      feed.New[*sync.ReorgBlockRange](),
		pendingData: feed.New[core.PendingData](),
		l1Heads:     feed.New[*core.L1Head](),

		blockTraceCache: lru.NewCache[rpccore.TraceCacheKey, []TracedBlockTransaction](rpccore.TraceCacheSize),
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

func (h *Handler) WithSubmittedTransactionsCache(cache *rpccore.SubmittedTransactionsCache) *Handler {
	h.submittedTransactionsCache = cache
	return h
}

// Currently only used for testing
func (h *Handler) Run(ctx context.Context) error {
	newHeadsSub := h.syncReader.SubscribeNewHeads().Subscription
	reorgsSub := h.syncReader.SubscribeReorg().Subscription
	pendingData := h.syncReader.SubscribePendingData().Subscription
	l1HeadsSub := h.bcReader.SubscribeL1Head().Subscription
	defer newHeadsSub.Unsubscribe()
	defer reorgsSub.Unsubscribe()
	defer pendingData.Unsubscribe()
	defer l1HeadsSub.Unsubscribe()
	feed.Tee(newHeadsSub, h.newHeads)
	feed.Tee(reorgsSub, h.reorgs)
	feed.Tee(pendingData, h.pendingData)
	feed.Tee(l1HeadsSub, h.l1Heads)

	<-ctx.Done()
	h.subscriptions.Range(func(key, value any) bool {
		sub := value.(*subscription)
		sub.wg.Wait()
		return true
	})
	return nil
}

func (h *Handler) Version() (string, *jsonrpc.Error) {
	return h.version, nil
}

func (h *Handler) SpecVersion() (string, *jsonrpc.Error) {
	return "0.8.1", nil
}

// Currently only used for testing
func (h *Handler) methods() ([]jsonrpc.Method, string) { //nolint: funlen
	return []jsonrpc.Method{
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
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.StorageAt,
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
			Name:    "starknet_getTransactionStatus",
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
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "simulation_flags"}, {Name: "block_id"}},
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
		{
			Name:    "starknet_traceBlockTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.TraceBlockTransactions,
		},
		{
			Name:    "starknet_specVersion",
			Handler: h.SpecVersion,
		},
		{
			Name:    "starknet_subscribeEvents",
			Params:  []jsonrpc.Parameter{{Name: "from_address", Optional: true}, {Name: "keys", Optional: true}, {Name: "block_id", Optional: true}},
			Handler: h.SubscribeEvents,
		},
		{
			Name:    "starknet_subscribeNewHeads",
			Params:  []jsonrpc.Parameter{{Name: "block_id", Optional: true}},
			Handler: h.SubscribeNewHeads,
		},
		{
			Name:    "starknet_subscribeTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.SubscribeTransactionStatus,
		},
		{
			Name:    "starknet_subscribePendingTransactions",
			Params:  []jsonrpc.Parameter{{Name: "transaction_details", Optional: true}, {Name: "sender_address", Optional: true}},
			Handler: h.SubscribePendingTxs,
		},
		{
			Name:    "starknet_unsubscribe",
			Params:  []jsonrpc.Parameter{{Name: "subscription_id"}},
			Handler: h.Unsubscribe,
		},
		{
			Name:    "starknet_getBlockWithReceipts",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithReceipts,
		},
		{
			Name:    "starknet_getCompiledCasm",
			Params:  []jsonrpc.Parameter{{Name: "class_hash"}},
			Handler: h.CompiledCasm,
		},
		{
			Name:    "starknet_getMessagesStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.GetMessageStatus,
		},
	}, "/v0_8"
}
