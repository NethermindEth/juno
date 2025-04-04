package transaction

import (
	"errors"
	"fmt"
	"wer/api"
	"wer/state"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

// TransactionType represents the type of transaction
type TransactionType int

const (
	TxTypeInvoke TransactionType = iota
	TxTypeDeclare
	TxTypeDeployAccount
	TxTypeL1Handler
)

// Note: Golang has an explicit abstraction-vs-efficeincy overhead, where interfaces lie on one side, and hand-written types lie on the other.
// Generics can help create zero-cost abstractions, but only for concrete types, NOT reference types (it depends on the GSchape of the type, which is a bit compilcated).
// There is a Box type where we define each concrete txn as seperate fields in ExecutableTransaction, and just access them accordingly. This seems to be the most efficent. But kind of ugly.
// Note: ExecutableTransaction[T core.Transaction] shouldn't be used since wrapping an interface in a Generic can be more inefficeint than even using an interface.
// Note: The current approach constraints the allowed types to the known transactions. However, we must type cast at some point (sad face?). We should only need to do it once per execution, so it _should_ have negligable impact on performance
// Note: Maybe we can replae this with a "box" type that contains all the information we need. No interfaces. Types known at compile time. Slightly ugly, but not soo bad (?).
// Note: A union of types in the Generic definition will force a runtime type check at some point. We could get away with this if we use the `unsafe`, pkg, but could make solving bugs difficult.
// Note: use code generators???
// Note: imo, we should push ahead with the Generic[union] approach for now. Test if type casting is really a bottleneck, and if so, switch to the box solution. Shouldn't introduce `unsafe` code imo.
// Note: core.InvokeTransaction etc use reference types as internal fields which force heap allocation. Maybe we should redefine these types locally with value types (until they are fixed in the rest of the code)?
type ExecutableTransaction[T core.InvokeTransaction | core.DeclareTransaction | core.DeployAccountTransaction | core.L1HandlerTransaction] struct {
	Inner              T
	ExecutionFlags     api.ExecutionFlags
	TransactionContext TransactionContext
	isL1Handler        bool
	revertibile        bool // Whether the transaction is revertible

	TxnType  TransactionType
	callData []felt.Felt // Todo: remove if we just type cast?
}

// Todo: Update NewExecutableTransaction such that we store each type of transaction (value type), and populate it.
// Then when we need to access these fields, we access the data directly.
// The benefit is that we can directly access all the data, the ExecutableTransaciotn acts like a single transaction type. And we have no dynamic dispatch so the compiler can aggresively optimise.
// The downside is that we must implement all the code by hand, eg can't use interfaces etc.

// NewExecutableTransaction creates a ExecutableTransaction from a core Transaction
func NewExecutableTransaction[T core.InvokeTransaction | core.DeclareTransaction | core.DeployAccountTransaction | core.L1HandlerTransaction](tx T, flags api.ExecutionFlags) (ExecutableTransaction[T], error) {
	var txnInfo TransactionInfo
	var txnContext TransactionContext
	var txnType TransactionType
	isL1Handler := false
	result := ExecutableTransaction[T]{}
	revertibile := false

	// Determine transaction type once during creation
	switch concrete := any(tx).(type) {
	case core.InvokeTransaction:
		txnType = TxTypeInvoke
		if !concrete.Version.Is(0) {
			// Only Invoke v1 and v3 are revertible
			revertibile = true
		}
		calldata := make([]felt.Felt, len(concrete.CallData))
		for i := range concrete.CallData { // Todo: update core?
			calldata[i] = *concrete.CallData[i]
		}
	case core.DeclareTransaction:
		txnType = TxTypeDeclare
	case core.DeployAccountTransaction:
		txnType = TxTypeDeployAccount
	case core.L1HandlerTransaction:
		txnType = TxTypeL1Handler
	default:
		return ExecutableTransaction[T]{}, fmt.Errorf("unsupported transaction type: %T", concrete)
	}

	// Todo: create and assign block Info
	txnContext.TxInfo = txnInfo
	result.Inner = tx
	result.TransactionContext = txnContext
	result.ExecutionFlags = flags
	result.isL1Handler = isL1Handler
	result.revertibile = revertibile
	result.TxnType = txnType
	return result, nil
}

// Execute implements the StarknetTx interface
func (tx ExecutableTransaction[T]) Execute(state *state.CachedState, blockContext api.BlockContext) (TransactionExecutionInfo, error) {
	// Update transaction context with block context
	tx.TransactionContext.BlockContext = blockContext

	// Call the appropriate execution function based on transaction type
	if tx.isL1Handler {
		// For L1 handler transactions
		return executeL1HandlerTransaction(tx, state)
	} else {
		// For account transactions (Invoke, Declare, DeployAccount)
		return executeAccountTransaction(tx, state)
	}
}

func executeAccountTransaction[T core.InvokeTransaction | core.DeclareTransaction | core.DeployAccountTransaction | core.L1HandlerTransaction](tx ExecutableTransaction[T], state *state.CachedState) (TransactionExecutionInfo, error) {
	txContext := tx.TransactionContext

	// if err := tx.verifyTxVersion(txContext.TxInfo.Version()); err != nil {
	// 	return TransactionExecutionInfo{}, err
	// }

	// Perform pre-validation stage with strict nonce check
	// strictNonceCheck := true
	// if err := tx.performPreValidationStage(state, &txContext, strictNonceCheck); err != nil {
	// 	return TransactionExecutionInfo{}, err
	// }

	// Initialize gas counter with initial gas
	initialGas := txContext.InitialSierraGas()
	remainingGas := GasCounter{RemainingGas: initialGas}

	// Run validation and execution (note: this is run_or_revert)
	var validateExecuteCallInfo TransactionExecutionInfo
	var err error
	if tx.revertibile {
		validateExecuteCallInfo, err = run_revertible(tx, state, tx.ExecutionFlags, txContext, remainingGas)
	} else {
		validateExecuteCallInfo, err = run_non_revertible(tx, state, tx.ExecutionFlags, txContext, remainingGas)
	}
	if err != nil {
		return TransactionExecutionInfo{}, err
	}

	// Handle fee transfer
	// feeTransferCallInfo, err := tx.handleFee(
	// 	state,
	// 	txContext,
	// 	validateExecuteCallInfo.FinalCost.Fee,
	// 	tx.ExecutionFlags.ChargeFee,
	// 	tx.ExecutionFlags.OnlyQuery, // Assuming concurrency_mode is related to OnlyQuery
	// )
	// if err != nil {
	// 	return TransactionExecutionInfo{}, err
	// }

	// TODO: populate the struct
	// Construct the transaction execution info
	// txExecutionInfo := TransactionExecutionInfo{}

	return validateExecuteCallInfo, nil
}

func executeL1HandlerTransaction[T core.InvokeTransaction | core.DeclareTransaction | core.DeployAccountTransaction | core.L1HandlerTransaction](tx ExecutableTransaction[T], state *state.CachedState) (TransactionExecutionInfo, error) {
	// Implement the logic for executing L1 handler transactions
	// This should include calling tx.executeFn and handling any specific logic for L1 handler transactions

	// // Todo
	// gasLimitFromCosntants := GasCounter{}

	// txExecutionInfo, err := tx.executeFn(tx.Inner, state, tx.ExecutionFlags, tx.TransactionContext, gasLimitFromCosntants)
	// if err != nil {
	// 	return TransactionExecutionInfo{}, err
	// }

	// // Additional logic for L1 handler transactions can be added here

	return TransactionExecutionInfo{}, errors.New("todo")
}

func (tx ExecutableTransaction[T]) Validate(state *state.CachedState) error {
	return errors.New("todo: implement")
}

// Add this for L1 handlers
func run_l1_handler(tx core.L1HandlerTransaction, state *state.CachedState, flags api.ExecutionFlags, txContext TransactionContext, remainingGas GasCounter) (TransactionExecutionInfo, error) {
	// Implement L1 handler execution logic
	// This can call into run_execute_l1handler
	return TransactionExecutionInfo{}, nil
}
