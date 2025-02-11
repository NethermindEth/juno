package rpc

import (
	"context"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/jsonrpc"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"golang.org/x/sync/errgroup"
)

type Handler struct {
	rpcv6Handler *rpcv6.Handler
	rpcv7Handler *rpcv7.Handler
	rpcv8Handler *rpcv8.Handler
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, virtualMachine vm.VM, version string,
	logger utils.Logger, network *utils.Network,
) *Handler {
	handlerv6 := rpcv6.New(bcReader, syncReader, virtualMachine, version, network, logger)
	handlerv7 := rpcv7.New(bcReader, syncReader, virtualMachine, version, network, logger)
	handlerv8 := rpcv8.New(bcReader, syncReader, virtualMachine, version, logger)

	return &Handler{
		rpcv6Handler: handlerv6,
		rpcv7Handler: handlerv7,
		rpcv8Handler: handlerv8,
	}
}

// WithFilterLimit sets the maximum number of blocks to scan in a single call for event filtering.
func (h *Handler) WithFilterLimit(limit uint) *Handler {
	h.rpcv6Handler.WithFilterLimit(limit)
	h.rpcv7Handler.WithFilterLimit(limit)
	h.rpcv8Handler.WithFilterLimit(limit)
	return h
}

func (h *Handler) WithL1Client(l1Client rpccore.L1Client) *Handler {
	h.rpcv8Handler.WithL1Client(l1Client)
	return h
}

func (h *Handler) WithCallMaxSteps(maxSteps uint64) *Handler {
	h.rpcv6Handler.WithCallMaxSteps(maxSteps)
	h.rpcv7Handler.WithCallMaxSteps(maxSteps)
	h.rpcv8Handler.WithCallMaxSteps(maxSteps)
	return h
}

func (h *Handler) WithIDGen(idgen func() uint64) *Handler {
	h.rpcv6Handler.WithIDGen(idgen)
	h.rpcv7Handler.WithIDGen(idgen)
	h.rpcv8Handler.WithIDGen(idgen)
	return h
}

func (h *Handler) WithFeeder(feederClient *feeder.Client) *Handler {
	h.rpcv6Handler.WithFeeder(feederClient)
	h.rpcv7Handler.WithFeeder(feederClient)
	h.rpcv8Handler.WithFeeder(feederClient)
	return h
}

func (h *Handler) WithGateway(gatewayClient rpccore.Gateway) *Handler {
	h.rpcv6Handler.WithGateway(gatewayClient)
	h.rpcv7Handler.WithGateway(gatewayClient)
	h.rpcv8Handler.WithGateway(gatewayClient)
	return h
}

func (h *Handler) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return h.rpcv6Handler.Run(ctx) })
	g.Go(func() error { return h.rpcv7Handler.Run(ctx) })
	g.Go(func() error { return h.rpcv8Handler.Run(ctx) })

	return g.Wait()
}

func (h *Handler) MethodsV0_8() ([]jsonrpc.Method, string) { //nolint: funlen
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.rpcv8Handler.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.rpcv8Handler.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.rpcv8Handler.BlockHashAndNumber,
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
			Handler: h.rpcv8Handler.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.rpcv8Handler.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv8Handler.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.rpcv8Handler.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv8Handler.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.rpcv8Handler.StorageAt,
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
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv8Handler.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.rpcv8Handler.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv8Handler.ClassAt,
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
			Handler: h.rpcv8Handler.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.rpcv8Handler.Version,
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
			Params:  []jsonrpc.Parameter{{Name: "id"}},
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
			Handler: h.rpcv7Handler.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.rpcv7Handler.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.rpcv7Handler.BlockHashAndNumber,
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
			Handler: h.rpcv7Handler.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.rpcv7Handler.TransactionReceiptByHash,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv7Handler.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.rpcv7Handler.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.rpcv7Handler.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.rpcv7Handler.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv7Handler.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.rpcv7Handler.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv7Handler.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.rpcv7Handler.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.rpcv7Handler.ClassAt,
		},
		{
			Name:    "starknet_addInvokeTransaction",
			Params:  []jsonrpc.Parameter{{Name: "invoke_transaction"}},
			Handler: h.rpcv7Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeployAccountTransaction",
			Params:  []jsonrpc.Parameter{{Name: "deploy_account_transaction"}},
			Handler: h.rpcv7Handler.AddTransaction,
		},
		{
			Name:    "starknet_addDeclareTransaction",
			Params:  []jsonrpc.Parameter{{Name: "declare_transaction"}},
			Handler: h.rpcv7Handler.AddTransaction,
		},
		{
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.rpcv7Handler.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.rpcv7Handler.Version,
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
			Handler: h.rpcv6Handler.Version,
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
