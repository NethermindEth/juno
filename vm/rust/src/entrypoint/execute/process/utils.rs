use crate::state_reader::JunoStateReader;
use blockifier::execution::contract_class::TrackedResource;
use blockifier::state::state_api::{StateReader, StateResult};
use blockifier::transaction::transaction_execution::Transaction;
use blockifier::{
    context::BlockContext,
    state::cached_state::{CachedState, TransactionalState},
};
use starknet_api::core::ClassHash;
use starknet_api::executable_transaction::AccountTransaction;
use starknet_api::transaction::fields::GasVectorComputationMode;
use starknet_api::transaction::{DeclareTransaction, DeployAccountTransaction, InvokeTransaction};

/// Determines whether L2 gas accounting should be enabled for fee estimation.
///
/// Starknet 0.13.4 introduced runtime L2 gas accounting, which is only enabled
/// if both the caller and the callee contract classes were compiled as Sierra 1.7.
/// This function checks whether the sender contract meets this requirement.
fn is_deploy_account_transaction(transaction: &Transaction) -> bool {
    matches!(
        transaction,
        Transaction::Account(
            blockifier::transaction::account_transaction::AccountTransaction {
                tx: starknet_api::executable_transaction::AccountTransaction::DeployAccount(_),
                ..
            }
        )
    )
}

pub fn is_l2_gas_accounting_enabled(
    transaction: &Transaction,
    state: &mut TransactionalState<'_, CachedState<JunoStateReader>>,
    block_context: &BlockContext,
    gas_computation_mode: &GasVectorComputationMode,
) -> StateResult<bool> {
    // DEPLOY_ACCOUNT: Cannot determine L2 gas accounting from sender's class hash (account doesn't exist yet).
    // For Braavos accounts, constructor also swaps class hash, breaking L2 gas detection entirely.
    if is_deploy_account_transaction(transaction) {
        return Ok(matches!(
            gas_computation_mode,
            GasVectorComputationMode::All
        ));
    }

    let sender_class_hash = state.get_class_hash_at(transaction.sender_address())?;

    // L2 gas accounting is disabled if the sender contract is uninitialized.
    if sender_class_hash == ClassHash::default() {
        return Ok(false);
    }

    let min_sierra_version = &block_context
        .versioned_constants()
        .min_sierra_version_for_sierra_gas;
    let sender_tracked_resource = state
        .get_compiled_class(sender_class_hash)?
        .tracked_resource(min_sierra_version, None);

    // L2 gas accounting is enabled if:
    // 1. The gas computation mode requires all gas vectors.
    // 2. The sender contract's tracked resource is Sierra gas.
    Ok(
        matches!(gas_computation_mode, GasVectorComputationMode::All)
            && sender_tracked_resource == TrackedResource::SierraGas,
    )
}

pub fn determine_gas_vector_mode(transaction: &Transaction) -> GasVectorComputationMode {
    match transaction {
        Transaction::Account(account_tx) => match &account_tx.tx {
            AccountTransaction::Declare(tx) => match &tx.tx {
                DeclareTransaction::V3(tx) => tx.resource_bounds.get_gas_vector_computation_mode(),
                _ => GasVectorComputationMode::NoL2Gas,
            },
            AccountTransaction::DeployAccount(tx) => match &tx.tx {
                DeployAccountTransaction::V3(tx) => {
                    tx.resource_bounds.get_gas_vector_computation_mode()
                }
                _ => GasVectorComputationMode::NoL2Gas,
            },
            AccountTransaction::Invoke(tx) => match &tx.tx {
                InvokeTransaction::V3(tx) => tx.resource_bounds.get_gas_vector_computation_mode(),
                _ => GasVectorComputationMode::NoL2Gas,
            },
        },
        Transaction::L1Handler(_) => GasVectorComputationMode::NoL2Gas,
    }
}
