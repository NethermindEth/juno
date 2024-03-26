package rpc

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"math"
	stdsync "sync"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/sourcegraph/conc"
)

//go:generate mockgen -destination=../mocks/mock_gateway_handler.go -package=mocks github.com/NethermindEth/juno/rpc Gateway
type Gateway interface {
	AddTransaction(context.Context, json.RawMessage) (json.RawMessage, error)
}

var (
	ErrContractNotFound                = &jsonrpc.Error{Code: 20, Message: "Contract not found"}
	ErrBlockNotFound                   = &jsonrpc.Error{Code: 24, Message: "Block not found"}
	ErrInvalidTxHash                   = &jsonrpc.Error{Code: 25, Message: "Invalid transaction hash"}
	ErrInvalidBlockHash                = &jsonrpc.Error{Code: 26, Message: "Invalid block hash"}
	ErrInvalidTxIndex                  = &jsonrpc.Error{Code: 27, Message: "Invalid transaction index in a block"}
	ErrClassHashNotFound               = &jsonrpc.Error{Code: 28, Message: "Class hash not found"}
	ErrTxnHashNotFound                 = &jsonrpc.Error{Code: 29, Message: "Transaction hash not found"}
	ErrPageSizeTooBig                  = &jsonrpc.Error{Code: 31, Message: "Requested page size is too big"}
	ErrNoBlock                         = &jsonrpc.Error{Code: 32, Message: "There are no blocks"}
	ErrInvalidContinuationToken        = &jsonrpc.Error{Code: 33, Message: "Invalid continuation token"}
	ErrTooManyKeysInFilter             = &jsonrpc.Error{Code: 34, Message: "Too many keys provided in a filter"}
	ErrContractError                   = &jsonrpc.Error{Code: 40, Message: "Contract error"}
	ErrTransactionExecutionError       = &jsonrpc.Error{Code: 41, Message: "Transaction execution error"}
	ErrInvalidContractClass            = &jsonrpc.Error{Code: 50, Message: "Invalid contract class"}
	ErrClassAlreadyDeclared            = &jsonrpc.Error{Code: 51, Message: "Class already declared"}
	ErrInternal                        = &jsonrpc.Error{Code: jsonrpc.InternalError, Message: "Internal error"}
	ErrInvalidTransactionNonce         = &jsonrpc.Error{Code: 52, Message: "Invalid transaction nonce"}
	ErrInsufficientMaxFee              = &jsonrpc.Error{Code: 53, Message: "Max fee is smaller than the minimal transaction cost (validation plus fee transfer)"} //nolint:lll
	ErrInsufficientAccountBalance      = &jsonrpc.Error{Code: 54, Message: "Account balance is smaller than the transaction's max_fee"}
	ErrValidationFailure               = &jsonrpc.Error{Code: 55, Message: "Account validation failed"}
	ErrCompilationFailed               = &jsonrpc.Error{Code: 56, Message: "Compilation failed"}
	ErrContractClassSizeTooLarge       = &jsonrpc.Error{Code: 57, Message: "Contract class size is too large"}
	ErrNonAccount                      = &jsonrpc.Error{Code: 58, Message: "Sender address is not an account contract"}
	ErrDuplicateTx                     = &jsonrpc.Error{Code: 59, Message: "A transaction with the same hash already exists in the mempool"}
	ErrCompiledClassHashMismatch       = &jsonrpc.Error{Code: 60, Message: "the compiled class hash did not match the one supplied in the transaction"} //nolint:lll
	ErrUnsupportedTxVersion            = &jsonrpc.Error{Code: 61, Message: "the transaction version is not supported"}
	ErrUnsupportedContractClassVersion = &jsonrpc.Error{Code: 62, Message: "the contract class version is not supported"}
	ErrUnexpectedError                 = &jsonrpc.Error{Code: 63, Message: "An unexpected error occurred"}

	// These errors can be only be returned by Juno-specific methods.
	ErrSubscriptionNotFound = &jsonrpc.Error{Code: 100, Message: "Subscription not found"}
)

const (
	maxEventChunkSize  = 10240
	maxEventFilterKeys = 1024
	traceCacheSize     = 128
	throttledVMErr     = "VM throughput limit reached"
)

type traceCacheKey struct {
	blockHash    felt.Felt
	v0_6Response bool
}

type Handler struct {
	bcReader      blockchain.Reader
	syncReader    sync.Reader
	gatewayClient Gateway
	feederClient  *feeder.Client
	vm            vm.VM
	log           utils.Logger
	version       string

	newHeads *feed.Feed[*core.Header]

	idgen         func() uint64
	mu            stdsync.Mutex // protects subscriptions.
	subscriptions map[uint64]*subscription

	blockTraceCache *lru.Cache[traceCacheKey, []TracedBlockTransaction]

	filterLimit  uint
	callMaxSteps uint64
}

type subscription struct {
	cancel func()
	wg     conc.WaitGroup
	conn   jsonrpc.Conn
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, virtualMachine vm.VM, version string, logger utils.Logger) *Handler {
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
		version:       version,
		newHeads:      feed.New[*core.Header](),
		subscriptions: make(map[uint64]*subscription),

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

func (h *Handler) WithGateway(gatewayClient Gateway) *Handler {
	h.gatewayClient = gatewayClient
	return h
}

func (h *Handler) Run(ctx context.Context) error {
	newHeadsSub := h.syncReader.SubscribeNewHeads().Subscription
	defer newHeadsSub.Unsubscribe()
	feed.Tee[*core.Header](newHeadsSub, h.newHeads)
	<-ctx.Done()
	for _, sub := range h.subscriptions {
		sub.wg.Wait()
	}
	return nil
}

func (h *Handler) Version() (string, *jsonrpc.Error) {
	return h.version, nil
}

func (h *Handler) SpecVersion() (string, *jsonrpc.Error) {
	return "0.7.0", nil
}

func (h *Handler) SpecVersionV0_6() (string, *jsonrpc.Error) {
	return "0.6.0", nil
}

func (h *Handler) Methods() ([]jsonrpc.Method, string) { //nolint: funlen
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.BlockHashAndNumber,
		},
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
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.ClassAt,
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
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.Version,
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
			Name:    "juno_subscribeNewHeads",
			Handler: h.SubscribeNewHeads,
		},
		{
			Name:    "juno_unsubscribe",
			Params:  []jsonrpc.Parameter{{Name: "id"}},
			Handler: h.Unsubscribe,
		},
		{
			Name:    "starknet_getBlockWithReceipts",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithReceipts,
		},
	}, "/v0_7"
}

func (h *Handler) MethodsV0_6() ([]jsonrpc.Method, string) { //nolint: funlen
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.BlockHashAndNumber,
		},
		{
			Name:    "starknet_getBlockWithTxHashes",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithTxHashesV0_6,
		},
		{
			Name:    "starknet_getBlockWithTxs",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithTxsV0_6,
		},
		{
			Name:    "starknet_getTransactionByHash",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionReceiptByHashV0_6,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.ClassAt,
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
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.Version,
		},
		{
			Name:    "starknet_getTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionStatus,
		},
		{
			Name:    "starknet_call",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: h.CallV0_6,
		},
		{
			Name:    "starknet_estimateFee",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "simulation_flags"}, {Name: "block_id"}},
			Handler: h.EstimateFeeV0_6,
		},
		{
			Name:    "starknet_estimateMessageFee",
			Params:  []jsonrpc.Parameter{{Name: "message"}, {Name: "block_id"}},
			Handler: h.EstimateMessageFeeV0_6,
		},
		{
			Name:    "starknet_traceTransaction",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TraceTransactionV0_6,
		},
		{
			Name:    "starknet_simulateTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "transactions"}, {Name: "simulation_flags"}},
			Handler: h.SimulateTransactionsV0_6,
		},
		{
			Name:    "starknet_traceBlockTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.TraceBlockTransactionsV0_6,
		},
		{
			Name:    "starknet_specVersion",
			Handler: h.SpecVersionV0_6,
		},
		{
			Name:    "juno_subscribeNewHeads",
			Handler: h.SubscribeNewHeads,
		},
		{
			Name:    "juno_unsubscribe",
			Params:  []jsonrpc.Parameter{{Name: "id"}},
			Handler: h.Unsubscribe,
		},
	}, "/v0_6"
}
