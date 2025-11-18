use blockifier::execution::stack_trace::gen_tx_execution_error_trace;
use blockifier::state::errors::StateError;
use blockifier::transaction::errors::TransactionExecutionError;

use crate::error::stack::ErrorStack;

#[derive(Debug)]
pub enum ExecutionError {
    ExecutionError {
        error: String,
        error_stack: ErrorStack,
    },
    Internal(String),
    Custom(String),
}

impl From<StateError> for ExecutionError {
    fn from(e: StateError) -> Self {
        match e {
            StateError::StateReadError(_) => Self::Internal(e.to_string()),
            _ => Self::Custom(format!("State error: {e}")),
        }
    }
}

impl From<starknet_api::StarknetApiError> for ExecutionError {
    fn from(value: starknet_api::StarknetApiError) -> Self {
        Self::Custom(value.to_string())
    }
}

impl From<anyhow::Error> for ExecutionError {
    fn from(value: anyhow::Error) -> Self {
        Self::Internal(value.to_string())
    }
}

impl ExecutionError {
    pub fn new(error: TransactionExecutionError) -> Self {
        let error_stack = gen_tx_execution_error_trace(&error);

        Self::ExecutionError {
            error: error.to_string(),
            error_stack: error_stack.into(),
        }
    }
}
