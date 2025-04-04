package transaction

import (
	"errors"
	"wer/api"
	"wer/state"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/felt"
)

// tmp note:
// Execute() 							calls	run_non_revertible / run_revertible
// run_non_revertible / run_revertible 	calls 	run_execute
// run_execute							calls 	run_execute_invoke / run_execute_deploy / etc

// run_non_revertible executes a non-revertible transaction
// Todo: call run_execute
func run_non_revertible[T core.InvokeTransaction | core.DeclareTransaction | core.DeployAccountTransaction | core.L1HandlerTransaction](tx ExecutableTransaction[T], state *state.CachedState, flags ExecutionFlags, txnContext TransactionContext, gasConsumed GasCounter) (TransactionExecutionInfo, error) {
	// TODO: Implement non-revertible transaction execution logic

	// Flow A - DeployAccount
	// 1. execution_context
	// 2. execute_call_info = run_execute
	// 3. handle_validate_tx

	// Flow B - !DeployAccount (Declare, InvokeV0)
	// 1. handle_validate_tx
	// 2. execution_context
	// 3. execute_call_info = run_execute

	// 4. return (receipt, error)

	return TransactionExecutionInfo{}, nil
}

// run_revertible executes a revertible transaction
// Invoke V1, V3
// 1. execution_context
// 2. validate_state_cache
// 3. ex_results = run_execute
// 4. return (receipt, error)
func run_revertible[T core.InvokeTransaction | core.DeclareTransaction | core.DeployAccountTransaction | core.L1HandlerTransaction](
	tx ExecutableTransaction[T],
	state *state.CachedState,
	flags api.ExecutionFlags,
	txContext TransactionContext,
	remainingGas GasCounter) (TransactionExecutionInfo, error) {

	// 1. Run validation via handle_validate_tx
	_, err := handleValidateTx(tx, state, txContext, remainingGas)
	if err != nil {
		return TransactionExecutionInfo{}, err
	}

	// 2. Create execution context
	// Get sierra gas limit from block context
	sierraGasLimit := txContext.BlockContext.Constants.SierraGasLimit(ExecutionModeExecute)
	// Limit gas usage
	gasAmount := NewGasAmount(remainingGas.LimitUsage(sierraGasLimit).Amount)

	executionContext := NewEntryPointExecutionContextForInvoke(
		txContext,
		flags,
		NewSierraGasRevertTracker(gasAmount),
	)

	// Calculate execution steps accounting for validation and overhead
	// nAllottedExecutionSteps := executionContext.SubtractValidationAndOverheadSteps(
	// 	validateCallInfo,
	// 	getTxType(tx),
	// 	getCalldataLength(tx),
	// )

	// 3. Save state changes from validation
	// validateStateCache := state.Clone() // Clone current state to preserve validation changes

	// 4. Create transactional state for execution
	// txState := state.Clone() // Use Clone instead of CreateTransactionalState

	// 5. Run execution
	// Todo: er determine the type when calling NewExecutableTransaction, consider updating this.
	// Declare variables but don't use them yet
	// var executeCallInfo []CallInfo
	// var executeError error

	switch tx.TxnType {
	case TxTypeInvoke:
		_, _ = run_execute_invoke(tx, state, executionContext, remainingGas)
	case TxTypeDeclare:
		_, _ = run_execute_declare(tx, state, executionContext, remainingGas)
	case TxTypeDeployAccount:
		_, _ = run_execute_deploy(tx, state, executionContext, remainingGas)
	default:
		return TransactionExecutionInfo{}, errors.New("run_revertible received an unknown transaction type")
	}

	// todo create TransactionExecutionInfo from call info

	return TransactionExecutionInfo{}, nil
	// TODO : complete all this stuff

	// Calculate consumed execution steps
	// executionStepsConsumed := nAllottedExecutionSteps - executionContext.GetRemainingSteps()
	//

	// Function to get receipt in case of revert
	// getRevertReceipt := func() TransactionReceipt {
	// 	return TransactionReceiptFromAccountTx(
	// 		tx,
	// 		txContext,
	// 		validateStateCache.ToStateDiff(),
	// 		CallInfo.SummarizeMany(
	// 			validateCallInfo,
	// 			txContext.BlockContext.VersionedConstants(),
	// 		),
	// 		executionStepsConsumed,
	// 		executionContext.SierraGasRevertTracker.GetCurrentGasConsumption(),
	// 	)
	// }

	// 6. Handle execution result
	// if executionErr == nil {
	// 	// Execution succeeded, calculate fee before committing
	// 	txReceipt := TransactionReceiptFromAccountTx(
	// 		tx,
	// 		txContext,
	// 		SquashStateDiff(
	// 			[]StateCache{validateStateCache, executionState.Clone()},
	// 			txContext.BlockContext.VersionedConstants().ComprehensiveStateDiff,
	// 		),
	// 		CallInfo.SummarizeMany(
	// 			append(validateCallInfo, executeCallInfo...),
	// 			txContext.BlockContext.VersionedConstants(),
	// 		),
	// 		0,
	// 		NewGasAmount(0),
	// 	)

	// 	// Post-execution checks
	// 	postExecutionReport, err := NewPostExecutionReport(
	// 		executionState,
	// 		txContext,
	// 		txReceipt,
	// 		flags,
	// 	)
	// 	if err != nil {
	// 		return TransactionExecutionInfo{}, err
	// 	}

	// 	if postExecError := postExecutionReport.Error(); postExecError != nil {
	// 		// Post-execution check failed - revert execution
	// 		executionState.Abort()
	// 		revertReceipt := getRevertReceipt()
	// 		revertReceipt.Fee = postExecutionReport.RecommendedFee()

	// 		return TransactionExecutionInfoFromReverted(
	// 			validateCallInfo,
	// 			postExecError,
	// 			revertReceipt,
	// 		), nil
	// 	} else {
	// 		// Post-execution check passed - commit execution
	// 		executionState.Commit()
	// 		return TransactionExecutionInfoFromAccepted(
	// 			validateCallInfo,
	// 			executeCallInfo,
	// 			txReceipt,
	// 		), nil
	// 	}
	// } else {
	// 	// Error during execution - revert
	// 	revertReceipt := getRevertReceipt()
	// 	executionState.Abort()

	// 	postExecutionReport, err := NewPostExecutionReport(
	// 		state,
	// 		txContext,
	// 		revertReceipt,
	// 		flags,
	// 	)
	// 	if err != nil {
	// 		return TransactionExecutionInfo{}, err
	// 	}

	// 	revertReceipt.Fee = postExecutionReport.RecommendedFee()
	// 	return TransactionExecutionInfoFromReverted(
	// 		validateCallInfo,
	// 		GenerateTxExecutionErrorTrace(executionErr),
	// 		revertReceipt,
	// 	), nil
	// }
}

// run_execute_invoke executes an invoke transaction
// Note: we have a union of types here, to avoid runtime casting.
func run_execute_invoke[T core.InvokeTransaction | core.DeclareTransaction | core.DeployAccountTransaction | core.L1HandlerTransaction](
	tx ExecutableTransaction[T],
	state *state.CachedState,
	executionContext *EntryPointExecutionContext,
	remainingGas GasCounter,
) ([]CallInfo, error) {

	// Todo: set this, should know it from caller info
	// V0-> get from caller
	// V1. V3 ->  selector_from_name(constants::EXECUTE_ENTRY_POINT_NAME)
	// Determine entry point selector based on transaction version
	var entryPointSelector felt.Felt

	// Todo: get sender address from the tx context
	// Get sender address from transaction context
	storageAddress := executionContext.TxContext.TxInfo.SenderAddress()

	// Todo: Get class hash at the sender addres, from the state
	classHash, err := state.GetClassHash(storageAddress)
	if err != nil {
		return nil, err
	}

	// Create call entry point
	executeCallEP := CallEntryPointVariant{
		EntryPointType:     EntryPointTypeExternal,
		EntryPointSelector: entryPointSelector,
		Calldata:           tx.callData,
		ClassHash:          nil,
		CodeAddress:        nil,
		StorageAddress:     storageAddress,
		CallerAddress:      ContractAddressDefault(),
		CallType:           CallTypeCall,
		InitialGas:         remainingGas.Amount,
	}

	// Execute the call non-revertingly // tmp note: The txn type is no longer needed beyond this point
	callInfo, err := executeCallEP.NonRevertingExecute(state.StateReader, executionContext, remainingGas)
	if err != nil {
		return nil, TransactionExecutionError{
			Error:          err,
			ClassHash:      classHash,
			StorageAddress: storageAddress,
			Selector:       entryPointSelector,
		}
	}

	return []CallInfo{callInfo}, nil
}

// run_execute_declare executes a declare transaction
// Note: we have a union of types here, to avoid runtime casting.
func run_execute_declare[T core.InvokeTransaction | core.DeclareTransaction | core.DeployAccountTransaction | core.L1HandlerTransaction](
	tx ExecutableTransaction[T],
	state *state.CachedState,
	executionContext *EntryPointExecutionContext,
	remainingGas GasCounter,
) ([]CallInfo, error) {
	declareTx, ok := any(tx).(*core.DeclareTransaction)
	if !ok {
		panic("expected DeclareTransaction")
	}

	// TODO: Implement declare transaction execution logic
	// Note: this will be a simple func, it simply calls set_contract_class and set_compiled_class_hash
	_ = declareTx // Use the variable to avoid unused var warning
	return []CallInfo{}, nil
}

// run_execute_deploy executes a deploy transaction
// Note: we have a union of types here, to avoid runtime casting.
func run_execute_deploy[T core.InvokeTransaction | core.DeclareTransaction | core.DeployAccountTransaction | core.L1HandlerTransaction](
	tx ExecutableTransaction[T],
	state *state.CachedState,
	executionContext *EntryPointExecutionContext,
	remainingGas GasCounter,
) ([]CallInfo, error) {
	deployTx, ok := any(tx).(*core.DeployTransaction)
	if !ok {
		panic("expected DeployTransaction")
	}

	// TODO: Implement deploy transaction execution logic
	// calls execute_constructor_entry_point() which calls non_reverting_execute()
	_ = deployTx // Use the variable to avoid unused var warning
	return []CallInfo{}, nil
}

// run_execute_l1handler executes an l1 handler txn
func run_execute_l1handler[T core.Transaction](
	tx T,
	state *state.CachedState,
	executionContext *EntryPointExecutionContext,
	remainingGas GasCounter,
) ([]CallInfo, error) {
	l1HandlerTx, ok := any(tx).(*core.L1HandlerTransaction)
	if !ok {
		panic("expected L1HandlerTransaction")
	}

	// TODO: Implement L1 handler transaction execution logic
	_ = l1HandlerTx // Use the variable to avoid unused var warning
	return []CallInfo{}, nil
}

// handleValidateTx validates a transaction
func handleValidateTx[T core.Transaction](
	tx T,
	state *state.CachedState,
	txContext TransactionContext,
	remainingGas GasCounter,
) ([]CallInfo, error) {
	// TODO: Implement transaction validation logic
	return []CallInfo{}, nil
}

// NewGasAmount creates a new gas amount
func NewGasAmount(amount uint64) GasAmount {
	return GasAmount{Amount: amount}
}

// NewEntryPointExecutionContextForInvoke creates a new execution context for invoke transactions
func NewEntryPointExecutionContextForInvoke(
	txContext TransactionContext,
	flags ExecutionFlags,
	gasTracker *SierraGasRevertTracker,
) *EntryPointExecutionContext {
	return &EntryPointExecutionContext{
		TxContext:              &txContext,
		ExecutionFlags:         flags,
		SierraGasRevertTracker: *gasTracker,
	}
}

// NewSierraGasRevertTracker creates a new gas revert tracker
func NewSierraGasRevertTracker(gasAmount GasAmount) *SierraGasRevertTracker {
	return &SierraGasRevertTracker{
		Gas: gasAmount,
	}
}

// getTxType gets the transaction type
func getTxType[T core.Transaction](tx T) TransactionType {
	// TODO: Implement transaction type detection
	return TransactionType(0)
}

// getCalldataLength gets the calldata length of a transaction
func getCalldataLength[T core.Transaction](tx T) int {
	// TODO: Implement calldata length calculation
	return 0
}

// TransactionReceiptFromAccountTx creates a transaction receipt from an account transaction
func TransactionReceiptFromAccountTx[T core.Transaction](
	tx T,
	txContext TransactionContext,
	stateDiff StateDiff,
	callInfos []CallInfoSummary,
	executionStepsConsumed uint64,
	gasConsumption GasAmount,
) TransactionReceipt {
	return TransactionReceipt{
		// TODO: Fill in receipt fields
		Fee: felt.NewFelt(gasConsumption.Amount), // Convert GasAmount to felt.Felt
	}
}

// SquashStateDiff squashes multiple state diffs into one
func SquashStateDiff(
	stateCaches []StateCache,
	comprehensiveStateDiff bool,
) StateDiff {
	// TODO: Implement state diff squashing
	return StateDiff{}
}

// NewPostExecutionReport creates a new post-execution report
func NewPostExecutionReport(
	state *state.CachedState,
	txContext TransactionContext,
	receipt TransactionReceipt,
	flags ExecutionFlags,
) (*PostExecutionReport, error) {
	return &PostExecutionReport{}, nil
}

// TransactionExecutionInfoFromReverted creates transaction execution info for a reverted transaction
func TransactionExecutionInfoFromReverted(
	validateCallInfo []CallInfo,
	executionError error,
	receipt TransactionReceipt,
) TransactionExecutionInfo {
	return TransactionExecutionInfo{
		// TODO: Fill in execution info fields
	}
}

// TransactionExecutionInfoFromAccepted creates transaction execution info for an accepted transaction
func TransactionExecutionInfoFromAccepted(
	validateCallInfo []CallInfo,
	executeCallInfo []CallInfo,
	receipt TransactionReceipt,
) TransactionExecutionInfo {
	return TransactionExecutionInfo{
		// TODO: Fill in execution info fields
	}
}

// GenerateTxExecutionErrorTrace generates an error trace from an execution error
func GenerateTxExecutionErrorTrace(err error) error {
	// TODO: Implement error trace generation
	return err
}

// Define missing types
type StateDiff struct {
	// Add necessary fields
}

type CallInfoSummary struct {
	// Add necessary fields
}

type StateCache struct {
	// Add necessary fields
}

type PostExecutionReport struct {
	// Add necessary fields
}
