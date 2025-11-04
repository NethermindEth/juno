use serde::Serialize;

use crate::{
    ffi_type::{
        function_invocation::FunctionInvocation,
        state_diff::{make_state_diff, StateDiff},
    },
    state_reader::JunoStateReader,
};
use blockifier::{
    blockifier_versioned_constants::VersionedConstants,
    state::{
        cached_state::{CachedState, TransactionalState},
        errors::StateError,
    },
    transaction::objects::TransactionExecutionInfo,
};
use starknet_api::{
    core::ClassHash,
    transaction::{
        fields::GasVectorComputationMode, DeclareTransaction, Transaction as StarknetApiTransaction,
    },
};

#[derive(Serialize, Default)]
#[serde(rename_all = "UPPERCASE")]
pub enum TransactionType {
    // dummy type for implementing Default trait
    #[default]
    Unknown,
    Invoke,
    Declare,
    #[serde(rename = "DEPLOY_ACCOUNT")]
    DeployAccount,
    #[serde(rename = "L1_HANDLER")]
    L1Handler,
}

#[allow(clippy::large_enum_variant)]
#[derive(Serialize)]
#[serde(untagged)]
pub enum ExecuteInvocation {
    Ok(FunctionInvocation),
    Revert { revert_reason: String },
}

#[derive(Serialize, Default)]
pub struct TransactionTrace {
    #[serde(skip_serializing_if = "Option::is_none")]
    validate_invocation: Option<FunctionInvocation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    execute_invocation: Option<ExecuteInvocation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    fee_transfer_invocation: Option<FunctionInvocation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    constructor_invocation: Option<FunctionInvocation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    function_invocation: Option<ExecuteInvocation>,
    r#type: TransactionType,
    state_diff: StateDiff,
}
pub fn new_transaction_trace(
    tx: &StarknetApiTransaction,
    info: TransactionExecutionInfo,
    state: &mut TransactionalState<CachedState<JunoStateReader>>,
    versioned_constants: &VersionedConstants,
    gas_vector_computation_mode: &GasVectorComputationMode,
) -> Result<TransactionTrace, StateError> {
    let mut trace = TransactionTrace::default();
    let mut deprecated_declared_class_hash: Option<ClassHash> = None;
    match tx {
        StarknetApiTransaction::L1Handler(_) => {
            trace.function_invocation = match info.revert_error {
                Some(err) => Some(ExecuteInvocation::Revert {
                    revert_reason: err.to_string(),
                }),
                None => info.execute_call_info.map(|v| {
                    ExecuteInvocation::Ok(FunctionInvocation::from_call_info(
                        v,
                        versioned_constants,
                        gas_vector_computation_mode,
                    ))
                }),
            };
            trace.r#type = TransactionType::L1Handler;
        }
        StarknetApiTransaction::DeployAccount(_) => {
            trace.validate_invocation = info.validate_call_info.map(|v| {
                FunctionInvocation::from_call_info(
                    v,
                    versioned_constants,
                    gas_vector_computation_mode,
                )
            });
            trace.constructor_invocation = info.execute_call_info.map(|v| {
                FunctionInvocation::from_call_info(
                    v,
                    versioned_constants,
                    gas_vector_computation_mode,
                )
            });
            trace.fee_transfer_invocation = info.fee_transfer_call_info.map(|v| {
                FunctionInvocation::from_call_info(
                    v,
                    versioned_constants,
                    gas_vector_computation_mode,
                )
            });
            trace.r#type = TransactionType::DeployAccount;
        }
        StarknetApiTransaction::Invoke(_) => {
            trace.validate_invocation = info.validate_call_info.map(|v| {
                FunctionInvocation::from_call_info(
                    v,
                    versioned_constants,
                    gas_vector_computation_mode,
                )
            });
            trace.execute_invocation = match info.revert_error {
                Some(err) => Some(ExecuteInvocation::Revert {
                    revert_reason: err.to_string(),
                }),
                None => info.execute_call_info.map(|v| {
                    ExecuteInvocation::Ok(FunctionInvocation::from_call_info(
                        v,
                        versioned_constants,
                        gas_vector_computation_mode,
                    ))
                }),
            };
            trace.fee_transfer_invocation = info.fee_transfer_call_info.map(|v| {
                FunctionInvocation::from_call_info(
                    v,
                    versioned_constants,
                    gas_vector_computation_mode,
                )
            });
            trace.r#type = TransactionType::Invoke;
        }
        StarknetApiTransaction::Declare(declare_txn) => {
            trace.validate_invocation = info.validate_call_info.map(|v| {
                FunctionInvocation::from_call_info(
                    v,
                    versioned_constants,
                    gas_vector_computation_mode,
                )
            });
            trace.fee_transfer_invocation = info.fee_transfer_call_info.map(|v| {
                FunctionInvocation::from_call_info(
                    v,
                    versioned_constants,
                    gas_vector_computation_mode,
                )
            });
            trace.r#type = TransactionType::Declare;
            deprecated_declared_class_hash = if info.revert_error.is_none() {
                match declare_txn {
                    DeclareTransaction::V0(_) => Some(declare_txn.class_hash()),
                    DeclareTransaction::V1(_) => Some(declare_txn.class_hash()),
                    _ => None,
                }
            } else {
                None
            }
        }
        StarknetApiTransaction::Deploy(_) => {
            // shouldn't happen since we don't support deploy
            panic!("Can't create transaction trace for deploy transaction (unsupported)");
        }
    };

    trace.state_diff = make_state_diff(state, deprecated_declared_class_hash)?;
    Ok(trace)
}
