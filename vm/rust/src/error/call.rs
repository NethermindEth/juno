use blockifier::execution::errors::{
    ConstructorEntryPointExecutionError, EntryPointExecutionError,
};
use blockifier::execution::stack_trace::gen_tx_execution_error_trace;
use blockifier::state::errors::StateError;
use blockifier::transaction::errors::TransactionExecutionError;
use starknet_api::core::{ClassHash, ContractAddress, EntryPointSelector};
use starknet_api::StarknetApiError;

use crate::error::stack::ErrorStack;

#[derive(Debug)]
pub enum CallError {
    ContractError(String, ErrorStack),
    Internal(String),
    Custom(String),
}

impl CallError {
    pub fn from_entry_point_execution_error(
        error: EntryPointExecutionError,
        contract_address: ContractAddress,
        class_hash: ClassHash,
        entry_point: EntryPointSelector,
    ) -> Self {
        TransactionExecutionError::ExecutionError {
            error,
            class_hash,
            storage_address: contract_address,
            selector: entry_point,
        }
        .into()
    }
}

impl From<TransactionExecutionError> for CallError {
    fn from(value: TransactionExecutionError) -> Self {
        let error_stack = gen_tx_execution_error_trace(&value);

        use TransactionExecutionError::*;
        match value {
            ContractConstructorExecutionFailed(
                ConstructorEntryPointExecutionError::ExecutionError { error, .. },
            )
            | ExecutionError { error, .. }
            | ValidateTransactionError { error, .. } => {
                Self::ContractError(error.to_string(), error_stack.into())
            }
            e => Self::ContractError(e.to_string(), error_stack.into()),
        }
    }
}

impl From<StateError> for CallError {
    fn from(e: StateError) -> Self {
        match e {
            StateError::StateReadError(_) => Self::Internal(e.to_string()),
            _ => Self::Custom(anyhow::anyhow!("State error: {}", e).to_string()),
        }
    }
}

impl From<StarknetApiError> for CallError {
    fn from(value: starknet_api::StarknetApiError) -> Self {
        Self::Custom(value.to_string())
    }
}

impl From<anyhow::Error> for CallError {
    fn from(value: anyhow::Error) -> Self {
        Self::Internal(value.to_string())
    }
}
