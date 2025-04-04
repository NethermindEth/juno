package execution

import (
	"wer/api"
	"wer/state"
	"wer/transaction"

	"github.com/NethermindEth/juno/core"
)

// Main execution entrypoint.
// ExecuteTransactionally executes a transaction with automatic state management.
// State changes are committed (/aborted) for successful (/failing) transactions.
// (Big) Todo: What about concurrent execution?
func ExecuteTransaction[T core.InvokeTransaction | core.DeclareTransaction | core.DeployAccountTransaction | core.L1HandlerTransaction](
	txn T, // tmp note: value allows for stack alloc. Don't need to mutate.
	state *state.TransactionalState, // tmp note: ptr because we will modify state
	blockContext api.BlockContext,
	executionFlags api.ExecutionFlags,
) (transaction.TransactionExecutionInfo, error) {
	var result transaction.TransactionExecutionInfo

	tx, err := transaction.NewExecutableTransaction(txn, executionFlags)
	if err != nil {
		return result, err
	}

	if err := tx.Validate(state.CachedState); err != nil {
		return result, err
	}

	result, err = tx.Execute(state.CachedState, blockContext)
	if err != nil {
		state.Abort()
		return result, err
	}

	if err := state.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
