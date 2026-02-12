package rpccore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	l1types "github.com/NethermindEth/juno/l1/types"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	MaxEventChunkSize             = 10240
	MaxEventFilterKeys            = 1024
	TraceCacheSize                = 128
	ThrottledVMErr                = "VM throughput limit reached"
	MaxBlocksBack                 = 1024
	EntrypointNotFoundFelt string = "0x454e545259504f494e545f4e4f545f464f554e44"
	ErrEPSNotFound                = "Entry point EntryPointSelector(%s) not found in contract."
)

//go:generate mockgen -destination=../mocks/mock_gateway_handler.go -package=mocks github.com/NethermindEth/juno/rpc Gateway
type Gateway interface {
	AddTransaction(context.Context, json.RawMessage) (json.RawMessage, error)
}

type L1Client interface {
	TransactionReceipt(ctx context.Context, txHash *l1types.L1Hash) (*types.Receipt, error)
}

type TraceCacheKey struct {
	BlockHash felt.Felt
}

var (
	ErrContractNotFound   = &jsonrpc.Error{Code: 20, Message: "Contract not found"}
	ErrEntrypointNotFound = &jsonrpc.Error{
		Code:    21,
		Message: "Requested entrypoint does not exist in the contract",
	}
	ErrEntrypointNotFoundV0_10 = &jsonrpc.Error{
		Code:    21,
		Message: "Requested entry point does not exist in the contract",
	}
	ErrBlockNotFound                    = &jsonrpc.Error{Code: 24, Message: "Block not found"}
	ErrInvalidTxHash                    = &jsonrpc.Error{Code: 25, Message: "Invalid transaction hash"}
	ErrInvalidBlockHash                 = &jsonrpc.Error{Code: 26, Message: "Invalid block hash"}
	ErrInvalidTxIndex                   = &jsonrpc.Error{Code: 27, Message: "Invalid transaction index in a block"}
	ErrClassHashNotFound                = &jsonrpc.Error{Code: 28, Message: "Class hash not found"}
	ErrTxnHashNotFound                  = &jsonrpc.Error{Code: 29, Message: "Transaction hash not found"}
	ErrPageSizeTooBig                   = &jsonrpc.Error{Code: 31, Message: "Requested page size is too big"}
	ErrNoBlock                          = &jsonrpc.Error{Code: 32, Message: "There are no blocks"}
	ErrInvalidContinuationToken         = &jsonrpc.Error{Code: 33, Message: "The supplied continuation token is invalid or unknown"}
	ErrTooManyKeysInFilter              = &jsonrpc.Error{Code: 34, Message: "Too many keys provided in a filter"}
	ErrContractError                    = &jsonrpc.Error{Code: 40, Message: "Contract error"}
	ErrTransactionExecutionError        = &jsonrpc.Error{Code: 41, Message: "Transaction execution error"}
	ErrStorageProofNotSupported         = &jsonrpc.Error{Code: 42, Message: "the node doesn't support storage proofs for blocks that are too far in the past"} //nolint:lll
	ErrInvalidContractClass             = &jsonrpc.Error{Code: 50, Message: "Invalid contract class"}
	ErrClassAlreadyDeclared             = &jsonrpc.Error{Code: 51, Message: "Class already declared"}
	ErrInternal                         = &jsonrpc.Error{Code: jsonrpc.InternalError, Message: "Internal error"}
	ErrInvalidTransactionNonce          = &jsonrpc.Error{Code: 52, Message: "Invalid transaction nonce"}
	ErrInsufficientMaxFee               = &jsonrpc.Error{Code: 53, Message: "Max fee is smaller than the minimal transaction cost (validation plus fee transfer)"} //nolint:lll
	ErrInsufficientResourcesForValidate = &jsonrpc.Error{Code: 53, Message: "The transaction’s resources don’t cover validation or the minimal transaction fee"}   //nolint:lll
	ErrInsufficientAccountBalance       = &jsonrpc.Error{Code: 54, Message: "Account balance is smaller than the transaction's max_fee"}
	ErrInsufficientAccountBalanceV0_8   = &jsonrpc.Error{Code: 54, Message: "Account balance is smaller than the transaction's " +
		"maximal fee (calculated as the sum of each resource's limit x max price)"}
	ErrValidationFailure                 = &jsonrpc.Error{Code: 55, Message: "Account validation failed"}
	ErrCompilationFailed                 = &jsonrpc.Error{Code: 56, Message: "Compilation failed"}
	ErrContractClassSizeTooLarge         = &jsonrpc.Error{Code: 57, Message: "Contract class size is too large"}
	ErrNonAccount                        = &jsonrpc.Error{Code: 58, Message: "Sender address is not an account contract"}
	ErrDuplicateTx                       = &jsonrpc.Error{Code: 59, Message: "A transaction with the same hash already exists in the mempool"}
	ErrCompiledClassHashMismatch         = &jsonrpc.Error{Code: 60, Message: "the compiled class hash did not match the one supplied in the transaction"} //nolint:lll
	ErrUnsupportedTxVersion              = &jsonrpc.Error{Code: 61, Message: "the transaction version is not supported"}
	ErrUnsupportedContractClassVersion   = &jsonrpc.Error{Code: 62, Message: "the contract class version is not supported"}
	ErrUnexpectedError                   = &jsonrpc.Error{Code: 63, Message: "An unexpected error occurred"}
	ErrReplacementTransactionUnderPriced = &jsonrpc.Error{Code: 64, Message: "Replacement transaction is underpriced"}
	ErrFeeBelowMinimum                   = &jsonrpc.Error{Code: 65, Message: "Transaction fee below minimum"}
	ErrInvalidSubscriptionID             = &jsonrpc.Error{Code: 66, Message: "Invalid subscription id"}
	ErrTooManyAddressesInFilter          = &jsonrpc.Error{Code: 67, Message: "Too many addresses in filter sender_address filter"}
	ErrTooManyBlocksBack                 = &jsonrpc.Error{Code: 68, Message: fmt.Sprintf("Cannot go back more than %v blocks", MaxBlocksBack)}
	ErrCallOnPending                     = &jsonrpc.Error{Code: 69, Message: "This method does not support being called on the pending block"}
	ErrCallOnPreConfirmed                = &jsonrpc.Error{
		Code: 70, Message: "This method does not support being called on the pre_confirmed block",
	}

	// These errors can be only be returned by Juno-specific methods.
	ErrSubscriptionNotFound = &jsonrpc.Error{Code: 100, Message: "Subscription not found"}
)
