package rpcv10

import (
	"context"
	"net/http"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
)

// RegisterMethods returns the JSON-RPC method bindings for v10. The
// list does NOT include `juno_version` — that lives on the outer
// rpc.Handler wrapper since it reads the cross-version build identifier.
//
//nolint:funlen // flat list of registrations
func (h *Handler) RegisterMethods() []jsonrpc.Method {
	return []jsonrpc.Method{
		jsonrpc.Register("starknet_chainId",
			func(_ *jsonrpc.NoParams) (*felt.Felt, *jsonrpc.Error) { return h.ChainID() }),
		jsonrpc.Register("starknet_blockNumber",
			func(_ *jsonrpc.NoParams) (uint64, *jsonrpc.Error) { return h.BlockNumber() }),
		jsonrpc.Register("starknet_blockHashAndNumber",
			func(_ *jsonrpc.NoParams) (*BlockHashAndNumber, *jsonrpc.Error) {
				return h.BlockHashAndNumber()
			}),
		jsonrpc.Register("starknet_getBlockWithTxHashes",
			func(p *BlockIDParams) (*BlockWithTxHashes, *jsonrpc.Error) {
				return h.BlockWithTxHashes(p.BlockID)
			}),
		jsonrpc.Register("starknet_getBlockWithTxs",
			func(p *BlockIDFlagsParams) (*BlockWithTxs, *jsonrpc.Error) {
				return h.BlockWithTxs(p.BlockID, p.ResponseFlags)
			}),
		jsonrpc.Register("starknet_getTransactionByHash",
			func(p *TxHashFlagsParams) (*Transaction, *jsonrpc.Error) {
				return h.TransactionByHash(p.TransactionHash, p.ResponseFlags)
			}),
		jsonrpc.Register("starknet_getTransactionReceipt",
			func(p *TxHashParams) (*TransactionReceipt, *jsonrpc.Error) {
				return h.TransactionReceiptByHash(p.TransactionHash)
			}),
		jsonrpc.Register("starknet_getBlockTransactionCount",
			func(p *BlockIDParams) (uint64, *jsonrpc.Error) {
				return h.BlockTransactionCount(p.BlockID)
			}),
		jsonrpc.Register("starknet_getTransactionByBlockIdAndIndex",
			func(p *TxByBlockIDAndIndexParams) (*Transaction, *jsonrpc.Error) {
				return h.TransactionByBlockIDAndIndex(p.BlockID, p.Index, p.ResponseFlags)
			}),
		jsonrpc.Register("starknet_getStateUpdate",
			func(p *StateUpdateParams) (StateUpdate, *jsonrpc.Error) {
				return h.StateUpdate(p.BlockID, p.ContractAddresses)
			}),
		jsonrpc.Register("starknet_syncing",
			func(_ *jsonrpc.NoParams) (*Sync, *jsonrpc.Error) { return h.Syncing() }),
		jsonrpc.Register("starknet_getNonce",
			func(p *NonceParams) (*felt.Felt, *jsonrpc.Error) {
				return h.Nonce(p.BlockID, p.ContractAddress)
			}),
		jsonrpc.Register("starknet_getStorageAt",
			func(p *StorageAtParams) (*StorageAtResponse, *jsonrpc.Error) {
				return h.StorageAt(p.ContractAddress, p.Key, p.BlockID, p.Flags)
			}),
		jsonrpc.Register("starknet_getClassHashAt",
			func(p *ClassAtParams) (*felt.Felt, *jsonrpc.Error) {
				return h.ClassHashAt(p.BlockID, p.ContractAddress)
			}),
		jsonrpc.Register("starknet_getClass",
			func(p *ClassParams) (*Class, *jsonrpc.Error) {
				return h.Class(p.BlockID, p.ClassHash)
			}),
		jsonrpc.Register("starknet_getClassAt",
			func(p *ClassAtParams) (*Class, *jsonrpc.Error) {
				return h.ClassAt(p.BlockID, p.ContractAddress)
			}),
		jsonrpc.RegisterC("starknet_addInvokeTransaction",
			func(ctx context.Context, p *AddInvokeTxParams) (AddTxResponse, *jsonrpc.Error) {
				return h.AddTransaction(ctx, p.InvokeTransaction)
			}),
		jsonrpc.RegisterC("starknet_addDeployAccountTransaction",
			func(ctx context.Context, p *AddDeployAccountTxParams) (AddTxResponse, *jsonrpc.Error) {
				return h.AddTransaction(ctx, p.DeployAccountTransaction)
			}),
		jsonrpc.RegisterC("starknet_addDeclareTransaction",
			func(ctx context.Context, p *AddDeclareTxParams) (AddTxResponse, *jsonrpc.Error) {
				return h.AddTransaction(ctx, p.DeclareTransaction)
			}),
		jsonrpc.Register("starknet_getEvents",
			func(p *EventsParams) (EventsChunk, *jsonrpc.Error) {
				return h.Events(p.Filter)
			}),
		jsonrpc.RegisterC("starknet_getTransactionStatus",
			func(ctx context.Context, p *TxHashParams) (TransactionStatus, *jsonrpc.Error) {
				return h.TransactionStatus(ctx, p.TransactionHash)
			}),
		jsonrpc.Register("starknet_call",
			func(p *CallParams) ([]*felt.Felt, *jsonrpc.Error) {
				return h.Call(p.Request, p.BlockID)
			}),
		jsonrpc.RegisterCH("starknet_estimateFee",
			func(ctx context.Context, p *EstimateFeeParams) ([]FeeEstimate, http.Header, *jsonrpc.Error) {
				return h.EstimateFee(ctx, p.Request, p.SimulationFlags, p.BlockID)
			}),
		jsonrpc.RegisterCH("starknet_estimateMessageFee",
			func(
				ctx context.Context,
				p *EstimateMessageFeeParams,
			) (FeeEstimate, http.Header, *jsonrpc.Error) {
				return h.EstimateMessageFee(ctx, p.Message, p.BlockID)
			}),
		jsonrpc.RegisterCH("starknet_traceTransaction",
			func(ctx context.Context, p *TxHashParams) (TransactionTrace, http.Header, *jsonrpc.Error) {
				return h.TraceTransaction(ctx, p.TransactionHash)
			}),
		jsonrpc.RegisterCH("starknet_simulateTransactions",
			func(
				ctx context.Context,
				p *SimulateTransactionsParams,
			) (SimulateTransactionsResponse, http.Header, *jsonrpc.Error) {
				return h.SimulateTransactions(ctx, p.BlockID, p.Transactions, p.SimulationFlags)
			}),
		jsonrpc.RegisterCH("starknet_traceBlockTransactions",
			func(
				ctx context.Context,
				p *TraceBlockTransactionsParams,
			) (TraceBlockTransactionsResponse, http.Header, *jsonrpc.Error) {
				return h.TraceBlockTransactions(ctx, p.BlockID, p.TraceFlags)
			}),
		jsonrpc.Register("starknet_specVersion",
			func(_ *jsonrpc.NoParams) (string, *jsonrpc.Error) { return h.SpecVersion() }),
		jsonrpc.RegisterC("starknet_subscribeEvents",
			func(ctx context.Context, p *SubscribeEventsParams) (SubscriptionID, *jsonrpc.Error) {
				return h.SubscribeEvents(ctx, p.FromAddress, p.Keys, p.BlockID, p.FinalityStatus)
			}),
		jsonrpc.RegisterC("starknet_subscribeNewTransactionReceipts",
			func(
				ctx context.Context,
				p *SubscribeNewTransactionReceiptsParams,
			) (SubscriptionID, *jsonrpc.Error) {
				return h.SubscribeNewTransactionReceipts(ctx, p.SenderAddress, p.FinalityStatus)
			}),
		jsonrpc.RegisterC("starknet_subscribeNewHeads",
			func(ctx context.Context, p *SubscribeNewHeadsParams) (SubscriptionID, *jsonrpc.Error) {
				return h.SubscribeNewHeads(ctx, p.BlockID)
			}),
		jsonrpc.RegisterC("starknet_subscribeTransactionStatus",
			func(ctx context.Context, p *TxHashParams) (SubscriptionID, *jsonrpc.Error) {
				return h.SubscribeTransactionStatus(ctx, p.TransactionHash)
			}),
		jsonrpc.RegisterC("starknet_subscribeNewTransactions",
			func(ctx context.Context, p *SubscribeNewTransactionsParams) (SubscriptionID, *jsonrpc.Error) {
				return h.SubscribeNewTransactions(ctx, p.FinalityStatus, p.SenderAddress, p.Tags)
			}),
		jsonrpc.RegisterC("starknet_unsubscribe",
			func(ctx context.Context, p *UnsubscribeParams) (bool, *jsonrpc.Error) {
				return h.Unsubscribe(ctx, p.SubscriptionID)
			}),
		jsonrpc.Register("starknet_getBlockWithReceipts",
			func(p *BlockIDFlagsParams) (*BlockWithReceipts, *jsonrpc.Error) {
				return h.BlockWithReceipts(p.BlockID, p.ResponseFlags)
			}),
		jsonrpc.Register("starknet_getCompiledCasm",
			func(p *ClassHashParams) (CompiledCasmResponse, *jsonrpc.Error) {
				return h.CompiledCasm(p.ClassHash)
			}),
		jsonrpc.RegisterC("starknet_getMessagesStatus",
			func(ctx context.Context, p *GetMessageStatusParams) ([]MsgStatus, *jsonrpc.Error) {
				return h.GetMessageStatus(ctx, p.TransactionHash)
			}),
		jsonrpc.Register("starknet_getStorageProof",
			func(p *StorageProofParams) (*StorageProofResult, *jsonrpc.Error) {
				return h.StorageProof(p.BlockID, p.ClassHashes, p.ContractAddresses, p.ContractsStorageKeys)
			}),
	}
}
