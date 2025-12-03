package rpc

import (
	"context"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mempool"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"golang.org/x/sync/errgroup"
)

type Handler struct {
	rpcv6Handler  *rpcv6.Handler
	rpcv7Handler  *rpcv7.Handler
	rpcv8Handler  *rpcv8.Handler
	rpcv9Handler  *rpcv9.Handler
	rpcv10Handler *rpcv10.Handler
	version       string
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, virtualMachine vm.VM, version string,
	logger utils.Logger, network *utils.Network,
) *Handler {
	handlerv6 := rpcv6.New(bcReader, syncReader, virtualMachine, network, logger)
	handlerv7 := rpcv7.New(bcReader, syncReader, virtualMachine, network, logger)
	handlerv8 := rpcv8.New(bcReader, syncReader, virtualMachine, logger)
	handlerv9 := rpcv9.New(bcReader, syncReader, virtualMachine, logger)
	handlerv10 := rpcv10.New(bcReader, syncReader, virtualMachine, logger)

	return &Handler{
		rpcv6Handler:  handlerv6,
		rpcv7Handler:  handlerv7,
		rpcv8Handler:  handlerv8,
		rpcv9Handler:  handlerv9,
		rpcv10Handler: handlerv10,
		version:       version,
	}
}

// WithFilterLimit sets the maximum number of blocks to scan in a single call for event filtering.
func (h *Handler) WithFilterLimit(limit uint) *Handler {
	h.rpcv6Handler.WithFilterLimit(limit)
	h.rpcv7Handler.WithFilterLimit(limit)
	h.rpcv8Handler.WithFilterLimit(limit)
	h.rpcv9Handler.WithFilterLimit(limit)
	h.rpcv10Handler.WithFilterLimit(limit)
	return h
}

func (h *Handler) WithL1Client(l1Client rpccore.L1Client) *Handler {
	h.rpcv8Handler.WithL1Client(l1Client)
	h.rpcv9Handler.WithL1Client(l1Client)
	h.rpcv10Handler.WithL1Client(l1Client)
	return h
}

func (h *Handler) WithCallMaxSteps(maxSteps uint64) *Handler {
	h.rpcv6Handler.WithCallMaxSteps(maxSteps)
	h.rpcv7Handler.WithCallMaxSteps(maxSteps)
	h.rpcv8Handler.WithCallMaxSteps(maxSteps)
	h.rpcv9Handler.WithCallMaxSteps(maxSteps)
	h.rpcv10Handler.WithCallMaxSteps(maxSteps)
	return h
}

func (h *Handler) WithCallMaxGas(maxGas uint64) *Handler {
	h.rpcv6Handler.WithCallMaxGas(maxGas)
	h.rpcv7Handler.WithCallMaxGas(maxGas)
	h.rpcv8Handler.WithCallMaxGas(maxGas)
	h.rpcv9Handler.WithCallMaxGas(maxGas)
	h.rpcv10Handler.WithCallMaxGas(maxGas)
	return h
}

func (h *Handler) WithFeeder(feederClient *feeder.Client) *Handler {
	h.rpcv6Handler.WithFeeder(feederClient)
	h.rpcv7Handler.WithFeeder(feederClient)
	h.rpcv8Handler.WithFeeder(feederClient)
	h.rpcv9Handler.WithFeeder(feederClient)
	h.rpcv10Handler.WithFeeder(feederClient)
	return h
}

func (h *Handler) WithGateway(gatewayClient rpccore.Gateway) *Handler {
	h.rpcv6Handler.WithGateway(gatewayClient)
	h.rpcv7Handler.WithGateway(gatewayClient)
	h.rpcv8Handler.WithGateway(gatewayClient)
	h.rpcv9Handler.WithGateway(gatewayClient)
	h.rpcv10Handler.WithGateway(gatewayClient)
	return h
}

func (h *Handler) WithMempool(memPool mempool.Pool) *Handler {
	h.rpcv6Handler.WithMempool(memPool)
	h.rpcv8Handler.WithMempool(memPool)
	h.rpcv9Handler.WithMempool(memPool)
	h.rpcv10Handler.WithMempool(memPool)
	return h
}

func (h *Handler) WithSubmittedTransactionsCache(cache *rpccore.TransactionCache) *Handler {
	h.rpcv6Handler.WithSubmittedTransactionsCache(cache)
	h.rpcv7Handler.WithSubmittedTransactionsCache(cache)
	h.rpcv8Handler.WithSubmittedTransactionsCache(cache)
	h.rpcv9Handler.WithSubmittedTransactionsCache(cache)
	h.rpcv10Handler.WithSubmittedTransactionsCache(cache)
	return h
}

func (h *Handler) Version() (string, *jsonrpc.Error) {
	return h.version, nil
}

func (h *Handler) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return h.rpcv6Handler.Run(ctx) })
	g.Go(func() error { return h.rpcv7Handler.Run(ctx) })
	g.Go(func() error { return h.rpcv8Handler.Run(ctx) })
	g.Go(func() error { return h.rpcv9Handler.Run(ctx) })
	g.Go(func() error { return h.rpcv10Handler.Run(ctx) })

	return g.Wait()
}

//nolint:funlen,dupl // just registering methods for rpc v10, shares many methods with rpcv9
func (h *Handler) MethodsV0_10() ([]jsonrpc.Method, string) {
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.rpcv6Handler.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.rpcv6Handler.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.rpcv6Handler.BlockHashAndNumber,
		},
		{
			Name:    "starknet_getBlockWithTxHashes",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv10Handler.BlockWithTxHashes,
		},
		{
			Name:    "starknet_getBlockWithTxs",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv10Handler.BlockWithTxs,
		},
		{
			Name:    "starknet_getTransactionByHash",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv9Handler.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv10Handler.TransactionReceiptByHash,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv9Handler.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.rpcv9Handler.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv10Handler.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.rpcv6Handler.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv9Handler.Nonce,
		},
		{
			Name: "starknet_getStorageAt",
			Params: []jsonrpc.Parameter{
				{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"},
			},
			Handler: h.rpcv9Handler.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv9Handler.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.rpcv9Handler.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv9Handler.ClassAt,
		},
		{
			Name:    "starknet_addInvokeTransaction",
			Params:  []jsonrpc.Parameter{{Name: "invoke_transaction"}},
			Handler: h.rpcv9Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeployAccountTransaction",
			Params:  []jsonrpc.Parameter{{Name: "deploy_account_transaction"}},
			Handler: h.rpcv9Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeclareTransaction",
			Params:  []jsonrpc.Parameter{{Name: "declare_transaction"}},
			Handler: h.rpcv9Handler.AddTransaction,
		},
		{
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.rpcv10Handler.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.Version,
		},
		{
			Name:    "starknet_getTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv10Handler.TransactionStatus,
		},
		{
			Name:    "starknet_call",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: h.rpcv10Handler.Call,
		},
		{
			Name: "starknet_estimateFee",
			Params: []jsonrpc.Parameter{
				{Name: "request"}, {Name: "simulation_flags"}, {Name: "block_id"},
			},
			Handler: h.rpcv9Handler.EstimateFee,
		},
		{
			Name:    "starknet_estimateMessageFee",
			Params:  []jsonrpc.Parameter{{Name: "message"}, {Name: "block_id"}},
			Handler: h.rpcv9Handler.EstimateMessageFee,
		},
		{
			Name:    "starknet_traceTransaction",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv10Handler.TraceTransaction,
		},
		{
			Name: "starknet_simulateTransactions",
			Params: []jsonrpc.Parameter{
				{Name: "block_id"}, {Name: "transactions"}, {Name: "simulation_flags"},
			},
			Handler: h.rpcv10Handler.SimulateTransactions,
		},
		{
			Name:    "starknet_traceBlockTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv10Handler.TraceBlockTransactions,
		},
		{
			Name:    "starknet_specVersion",
			Handler: h.rpcv10Handler.SpecVersion,
		},
		{
			Name: "starknet_subscribeEvents",
			Params: []jsonrpc.Parameter{
				{Name: "from_address", Optional: true},
				{Name: "keys", Optional: true},
				{Name: "block_id", Optional: true},
				{Name: "finality_status", Optional: true},
			},
			Handler: h.rpcv10Handler.SubscribeEvents,
		},
		{
			Name: "starknet_subscribeNewTransactionReceipts",
			Params: []jsonrpc.Parameter{
				{Name: "sender_address", Optional: true},
				{Name: "finality_status", Optional: true},
			},
			Handler: h.rpcv10Handler.SubscribeNewTransactionReceipts,
		},
		{
			Name:    "starknet_subscribeNewHeads",
			Params:  []jsonrpc.Parameter{{Name: "block_id", Optional: true}},
			Handler: h.rpcv10Handler.SubscribeNewHeads,
		},
		{
			Name:    "starknet_subscribeTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv10Handler.SubscribeTransactionStatus,
		},
		{
			Name: "starknet_subscribeNewTransactions",
			Params: []jsonrpc.Parameter{
				{Name: "finality_status", Optional: true},
				{Name: "sender_address", Optional: true},
			},
			Handler: h.rpcv10Handler.SubscribeNewTransactions,
		},
		{
			Name:    "starknet_unsubscribe",
			Params:  []jsonrpc.Parameter{{Name: "subscription_id"}},
			Handler: h.rpcv10Handler.Unsubscribe,
		},
		{
			Name:    "starknet_getBlockWithReceipts",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv10Handler.BlockWithReceipts,
		},
		{
			Name:    "starknet_getCompiledCasm",
			Params:  []jsonrpc.Parameter{{Name: "class_hash"}},
			Handler: h.rpcv9Handler.CompiledCasm,
		},
		{
			Name:    "starknet_getMessagesStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv9Handler.GetMessageStatus,
		},
		{
			Name: "starknet_getStorageProof",
			Params: []jsonrpc.Parameter{
				{Name: "block_id"},
				{Name: "class_hashes", Optional: true},
				{Name: "contract_addresses", Optional: true},
				{Name: "contracts_storage_keys", Optional: true},
			},
			Handler: h.rpcv9Handler.StorageProof,
		},
	}, "/v0_10"
}

//nolint:funlen,dupl // just registering methods for rpc v9, shares many methods with rpcv10
func (h *Handler) MethodsV0_9() ([]jsonrpc.Method, string) {
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.rpcv6Handler.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.rpcv6Handler.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.rpcv6Handler.BlockHashAndNumber,
		},
		{
			Name:    "starknet_getBlockWithTxHashes",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv9Handler.BlockWithTxHashes,
		},
		{
			Name:    "starknet_getBlockWithTxs",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv9Handler.BlockWithTxs,
		},
		{
			Name:    "starknet_getTransactionByHash",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv9Handler.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv9Handler.TransactionReceiptByHash,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv9Handler.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.rpcv9Handler.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv9Handler.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.rpcv6Handler.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv9Handler.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.rpcv9Handler.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv9Handler.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.rpcv9Handler.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv9Handler.ClassAt,
		},
		{
			Name:    "starknet_addInvokeTransaction",
			Params:  []jsonrpc.Parameter{{Name: "invoke_transaction"}},
			Handler: h.rpcv9Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeployAccountTransaction",
			Params:  []jsonrpc.Parameter{{Name: "deploy_account_transaction"}},
			Handler: h.rpcv9Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeclareTransaction",
			Params:  []jsonrpc.Parameter{{Name: "declare_transaction"}},
			Handler: h.rpcv9Handler.AddTransaction,
		},
		{
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.rpcv9Handler.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.Version,
		},
		{
			Name:    "starknet_getTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv9Handler.TransactionStatus,
		},
		{
			Name:    "starknet_call",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: h.rpcv9Handler.Call,
		},
		{
			Name:    "starknet_estimateFee",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "simulation_flags"}, {Name: "block_id"}},
			Handler: h.rpcv9Handler.EstimateFee,
		},
		{
			Name:    "starknet_estimateMessageFee",
			Params:  []jsonrpc.Parameter{{Name: "message"}, {Name: "block_id"}},
			Handler: h.rpcv9Handler.EstimateMessageFee,
		},
		{
			Name:    "starknet_traceTransaction",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv9Handler.TraceTransaction,
		},
		{
			Name:    "starknet_simulateTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "transactions"}, {Name: "simulation_flags"}},
			Handler: h.rpcv9Handler.SimulateTransactions,
		},
		{
			Name:    "starknet_traceBlockTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv9Handler.TraceBlockTransactions,
		},
		{
			Name:    "starknet_specVersion",
			Handler: h.rpcv9Handler.SpecVersion,
		},
		{
			Name: "starknet_subscribeEvents",
			Params: []jsonrpc.Parameter{
				{Name: "from_address", Optional: true},
				{Name: "keys", Optional: true},
				{Name: "block_id", Optional: true},
				{Name: "finality_status", Optional: true},
			},
			Handler: h.rpcv9Handler.SubscribeEvents,
		},
		{
			Name:    "starknet_subscribeNewTransactionReceipts",
			Params:  []jsonrpc.Parameter{{Name: "sender_address", Optional: true}, {Name: "finality_status", Optional: true}},
			Handler: h.rpcv9Handler.SubscribeNewTransactionReceipts,
		},
		{
			Name:    "starknet_subscribeNewHeads",
			Params:  []jsonrpc.Parameter{{Name: "block_id", Optional: true}},
			Handler: h.rpcv9Handler.SubscribeNewHeads,
		},
		{
			Name:    "starknet_subscribeTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv9Handler.SubscribeTransactionStatus,
		},
		{
			Name:    "starknet_subscribeNewTransactions",
			Params:  []jsonrpc.Parameter{{Name: "finality_status", Optional: true}, {Name: "sender_address", Optional: true}},
			Handler: h.rpcv9Handler.SubscribeNewTransactions,
		},
		{
			Name:    "starknet_unsubscribe",
			Params:  []jsonrpc.Parameter{{Name: "subscription_id"}},
			Handler: h.rpcv9Handler.Unsubscribe,
		},
		{
			Name:    "starknet_getBlockWithReceipts",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv9Handler.BlockWithReceipts,
		},
		{
			Name:    "starknet_getCompiledCasm",
			Params:  []jsonrpc.Parameter{{Name: "class_hash"}},
			Handler: h.rpcv9Handler.CompiledCasm,
		},
		{
			Name:    "starknet_getMessagesStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv9Handler.GetMessageStatus,
		},
		{
			Name: "starknet_getStorageProof",
			Params: []jsonrpc.Parameter{
				{Name: "block_id"},
				{Name: "class_hashes", Optional: true},
				{Name: "contract_addresses", Optional: true},
				{Name: "contracts_storage_keys", Optional: true},
			},
			Handler: h.rpcv9Handler.StorageProof,
		},
	}, "/v0_9"
}

func (h *Handler) MethodsV0_8() ([]jsonrpc.Method, string) { //nolint:funlen
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.rpcv6Handler.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.rpcv6Handler.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.rpcv6Handler.BlockHashAndNumber,
		},
		{
			Name:    "starknet_getBlockWithTxHashes",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv8Handler.BlockWithTxHashes,
		},
		{
			Name:    "starknet_getBlockWithTxs",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv8Handler.BlockWithTxs,
		},
		{
			Name:    "starknet_getTransactionByHash",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv8Handler.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv8Handler.TransactionReceiptByHash,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv6Handler.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.rpcv8Handler.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv6Handler.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.rpcv6Handler.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv6Handler.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.rpcv8Handler.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv6Handler.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.rpcv6Handler.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv6Handler.ClassAt,
		},
		{
			Name:    "starknet_addInvokeTransaction",
			Params:  []jsonrpc.Parameter{{Name: "invoke_transaction"}},
			Handler: h.rpcv8Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeployAccountTransaction",
			Params:  []jsonrpc.Parameter{{Name: "deploy_account_transaction"}},
			Handler: h.rpcv8Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeclareTransaction",
			Params:  []jsonrpc.Parameter{{Name: "declare_transaction"}},
			Handler: h.rpcv8Handler.AddTransaction,
		},
		{
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.rpcv6Handler.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.Version,
		},
		{
			Name:    "starknet_getTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv8Handler.TransactionStatus,
		},
		{
			Name:    "starknet_call",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: h.rpcv8Handler.Call,
		},
		{
			Name:    "starknet_estimateFee",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "simulation_flags"}, {Name: "block_id"}},
			Handler: h.rpcv8Handler.EstimateFee,
		},
		{
			Name:    "starknet_estimateMessageFee",
			Params:  []jsonrpc.Parameter{{Name: "message"}, {Name: "block_id"}},
			Handler: h.rpcv8Handler.EstimateMessageFee,
		},
		{
			Name:    "starknet_traceTransaction",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv8Handler.TraceTransaction,
		},
		{
			Name:    "starknet_simulateTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "transactions"}, {Name: "simulation_flags"}},
			Handler: h.rpcv8Handler.SimulateTransactions,
		},
		{
			Name:    "starknet_traceBlockTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv8Handler.TraceBlockTransactions,
		},
		{
			Name:    "starknet_specVersion",
			Handler: h.rpcv8Handler.SpecVersion,
		},
		{
			Name:    "starknet_subscribeEvents",
			Params:  []jsonrpc.Parameter{{Name: "from_address", Optional: true}, {Name: "keys", Optional: true}, {Name: "block_id", Optional: true}},
			Handler: h.rpcv8Handler.SubscribeEvents,
		},
		{
			Name:    "starknet_subscribeNewHeads",
			Params:  []jsonrpc.Parameter{{Name: "block_id", Optional: true}},
			Handler: h.rpcv8Handler.SubscribeNewHeads,
		},
		{
			Name:    "starknet_subscribeTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv8Handler.SubscribeTransactionStatus,
		},
		{
			Name:    "starknet_subscribePendingTransactions",
			Params:  []jsonrpc.Parameter{{Name: "transaction_details", Optional: true}, {Name: "sender_address", Optional: true}},
			Handler: h.rpcv8Handler.SubscribePendingTxs,
		},
		{
			Name:    "starknet_unsubscribe",
			Params:  []jsonrpc.Parameter{{Name: "subscription_id"}},
			Handler: h.rpcv8Handler.Unsubscribe,
		},
		{
			Name:    "starknet_getBlockWithReceipts",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv8Handler.BlockWithReceipts,
		},
		{
			Name:    "starknet_getCompiledCasm",
			Params:  []jsonrpc.Parameter{{Name: "class_hash"}},
			Handler: h.rpcv8Handler.CompiledCasm,
		},
		{
			Name:    "starknet_getMessagesStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv8Handler.GetMessageStatus,
		},
		{
			Name: "starknet_getStorageProof",
			Params: []jsonrpc.Parameter{
				{Name: "block_id"},
				{Name: "class_hashes", Optional: true},
				{Name: "contract_addresses", Optional: true},
				{Name: "contracts_storage_keys", Optional: true},
			},
			Handler: h.rpcv8Handler.StorageProof,
		},
	}, "/v0_8"
}

func (h *Handler) MethodsV0_7() ([]jsonrpc.Method, string) { //nolint: funlen
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.rpcv6Handler.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.rpcv6Handler.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.rpcv6Handler.BlockHashAndNumber,
		},
		{
			Name:    "starknet_getBlockWithTxHashes",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv7Handler.BlockWithTxHashes,
		},
		{
			Name:    "starknet_getBlockWithTxs",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv7Handler.BlockWithTxs,
		},
		{
			Name:    "starknet_getTransactionByHash",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv6Handler.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv7Handler.TransactionReceiptByHash,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv6Handler.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.rpcv7Handler.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv6Handler.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.rpcv6Handler.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv6Handler.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.rpcv7Handler.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv6Handler.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.rpcv6Handler.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv6Handler.ClassAt,
		},
		{
			Name:    "starknet_addInvokeTransaction",
			Params:  []jsonrpc.Parameter{{Name: "invoke_transaction"}},
			Handler: h.rpcv6Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeployAccountTransaction",
			Params:  []jsonrpc.Parameter{{Name: "deploy_account_transaction"}},
			Handler: h.rpcv6Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeclareTransaction",
			Params:  []jsonrpc.Parameter{{Name: "declare_transaction"}},
			Handler: h.rpcv6Handler.AddTransaction,
		},
		{
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.rpcv6Handler.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.Version,
		},
		{
			Name:    "starknet_getTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv7Handler.TransactionStatusV0_7,
		},
		{
			Name:    "starknet_call",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: h.rpcv7Handler.Call,
		},
		{
			Name:    "starknet_estimateFee",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "simulation_flags"}, {Name: "block_id"}},
			Handler: h.rpcv7Handler.EstimateFee,
		},
		{
			Name:    "starknet_estimateMessageFee",
			Params:  []jsonrpc.Parameter{{Name: "message"}, {Name: "block_id"}},
			Handler: h.rpcv7Handler.EstimateMessageFee,
		},
		{
			Name:    "starknet_traceTransaction",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv7Handler.TraceTransaction,
		},
		{
			Name:    "starknet_simulateTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "transactions"}, {Name: "simulation_flags"}},
			Handler: h.rpcv7Handler.SimulateTransactions,
		},
		{
			Name:    "starknet_traceBlockTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv7Handler.TraceBlockTransactions,
		},
		{
			Name:    "starknet_specVersion",
			Handler: h.rpcv7Handler.SpecVersion,
		},
		{
			Name:    "starknet_getBlockWithReceipts",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv7Handler.BlockWithReceipts,
		},
		{
			Name:    "juno_subscribeNewHeads",
			Handler: h.rpcv7Handler.SubscribeNewHeads,
		},
		{
			Name:    "juno_unsubscribe",
			Params:  []jsonrpc.Parameter{{Name: "id"}},
			Handler: h.rpcv7Handler.Unsubscribe,
		},
	}, "/v0_7"
}

func (h *Handler) MethodsV0_6() ([]jsonrpc.Method, string) { //nolint: funlen
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.rpcv6Handler.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.rpcv6Handler.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.rpcv6Handler.BlockHashAndNumber,
		},
		{
			Name:    "starknet_getBlockWithTxHashes",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv6Handler.BlockWithTxHashes,
		},
		{
			Name:    "starknet_getBlockWithTxs",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv6Handler.BlockWithTxs,
		},
		{
			Name:    "starknet_getTransactionByHash",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv6Handler.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv6Handler.TransactionReceiptByHash,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv6Handler.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.rpcv6Handler.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv6Handler.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.rpcv6Handler.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv6Handler.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.rpcv6Handler.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv6Handler.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.rpcv6Handler.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv6Handler.ClassAt,
		},
		{
			Name:    "starknet_addInvokeTransaction",
			Params:  []jsonrpc.Parameter{{Name: "invoke_transaction"}},
			Handler: h.rpcv6Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeployAccountTransaction",
			Params:  []jsonrpc.Parameter{{Name: "deploy_account_transaction"}},
			Handler: h.rpcv6Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeclareTransaction",
			Params:  []jsonrpc.Parameter{{Name: "declare_transaction"}},
			Handler: h.rpcv6Handler.AddTransaction,
		},
		{
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.rpcv6Handler.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.Version,
		},
		{
			Name:    "starknet_getTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv6Handler.TransactionStatus,
		},
		{
			Name:    "starknet_call",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: h.rpcv6Handler.Call,
		},
		{
			Name:    "starknet_estimateFee",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "simulation_flags"}, {Name: "block_id"}},
			Handler: h.rpcv6Handler.EstimateFee,
		},
		{
			Name:    "starknet_estimateMessageFee",
			Params:  []jsonrpc.Parameter{{Name: "message"}, {Name: "block_id"}},
			Handler: h.rpcv6Handler.EstimateMessageFee,
		},
		{
			Name:    "starknet_traceTransaction",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv6Handler.TraceTransaction,
		},
		{
			Name:    "starknet_simulateTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "transactions"}, {Name: "simulation_flags"}},
			Handler: h.rpcv6Handler.SimulateTransactions,
		},
		{
			Name:    "starknet_traceBlockTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv6Handler.TraceBlockTransactions,
		},
		{
			Name:    "starknet_specVersion",
			Handler: h.rpcv6Handler.SpecVersion,
		},
		{
			Name:    "juno_subscribeNewHeads",
			Handler: h.rpcv6Handler.SubscribeNewHeads,
		},
		{
			Name:    "juno_unsubscribe",
			Params:  []jsonrpc.Parameter{{Name: "id"}},
			Handler: h.rpcv6Handler.Unsubscribe,
		},
	}, "/v0_6"
}
