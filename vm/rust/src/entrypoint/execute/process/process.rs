use crate::entrypoint::execute::process::binary_search_execution::execute_transaction_with_binary_search;
use crate::entrypoint::execute::process::execution::execute_transaction;
use crate::entrypoint::execute::process::utils::{
    determine_gas_vector_mode, is_l2_gas_accounting_enabled,
};
use crate::error::execution::ExecutionError;
use crate::state_reader::JunoStateReader;

use blockifier::transaction::transaction_execution::Transaction;
use blockifier::{
    context::BlockContext,
    state::cached_state::{CachedState, TransactionalState},
    transaction::{errors::TransactionExecutionError, objects::TransactionExecutionInfo},
};

pub fn process_transaction(
    txn: &mut Transaction,
    txn_state: &mut TransactionalState<'_, CachedState<JunoStateReader>>,
    block_context: &BlockContext,
    error_on_revert: bool,
    allow_binary_search: bool,
) -> Result<TransactionExecutionInfo, ExecutionError> {
    let execute_binary_search_result = is_l2_gas_accounting_enabled(
        txn,
        txn_state,
        block_context,
        &determine_gas_vector_mode(txn),
    );

    match execute_binary_search_result {
        Ok(true) if allow_binary_search => {
            execute_transaction_with_binary_search(txn, txn_state, block_context, error_on_revert)
        }
        Ok(_) => execute_transaction(txn, txn_state, block_context, error_on_revert),
        Err(error) => Err(ExecutionError::new(TransactionExecutionError::StateError(
            error,
        ))),
    }
}
