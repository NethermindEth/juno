use crate::error::execution::ExecutionError;
use blockifier::state::state_api::UpdatableState;
use blockifier::transaction::objects::TransactionExecutionInfo;
use blockifier::transaction::transaction_execution::Transaction;
use blockifier::transaction::transactions::ExecutableTransaction;

pub fn execute_transaction<S>(
    tx: &Transaction,
    state: &mut S,
    block_context: &blockifier::context::BlockContext,
    error_on_revert: bool,
) -> Result<TransactionExecutionInfo, ExecutionError>
where
    S: UpdatableState,
{
    match tx.execute(state, block_context) {
        Ok(tx_info) => {
            if tx_info.is_reverted() && error_on_revert {
                if let Some(revert_error) = tx_info.revert_error {
                    let revert_string = revert_error.to_string();
                    return Err(ExecutionError::ExecutionError {
                        error: revert_string,
                        error_stack: revert_error.into(),
                    });
                }
            }
            Ok(tx_info)
        }
        Err(error) => Err(ExecutionError::new(error)),
    }
}
