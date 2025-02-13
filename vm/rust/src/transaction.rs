use crate::execution::TxnAndQueryBit;
use crate::juno_state_reader::{class_info_from_json_str, JunoStateReader};
use blockifier::execution::contract_class::TrackedResource;
use blockifier::fee::fee_checks::FeeCheckError;
use blockifier::state::state_api::{StateReader, StateResult};
use blockifier::transaction::account_transaction::ExecutionFlags;
use blockifier::transaction::transaction_execution::Transaction;
use blockifier::transaction::transactions::ExecutableTransaction;
use blockifier::{
    context::BlockContext,
    state::cached_state::{CachedState, TransactionalState},
    transaction::{errors::TransactionExecutionError, objects::TransactionExecutionInfo},
};
use starknet_api::contract_class::ClassInfo;
use starknet_api::core::ClassHash;
use starknet_api::executable_transaction::AccountTransaction;
use starknet_api::execution_resources::GasAmount;
use starknet_api::transaction::fields::{GasVectorComputationMode, ValidResourceBounds};
use starknet_api::transaction::{fields::Fee, Transaction as StarknetApiTransaction};
use starknet_api::transaction::{
    DeclareTransaction, DeployAccountTransaction, InvokeTransaction, TransactionHash,
};

pub fn execute_transaction(
    txn: &mut Transaction,
    txn_state: &mut TransactionalState<'_, CachedState<JunoStateReader>>,
    block_context: &BlockContext,
) -> Result<TransactionExecutionInfo, TransactionExecutionError> {
    match is_l2_gas_accounting_enabled(
        txn,
        txn_state,
        block_context,
        &determine_gas_vector_mode(txn),
    ) {
        Ok(true) => determine_l2_gas_limit_and_execute(txn, txn_state, block_context),
        Ok(false) => txn.execute(txn_state, block_context),
        Err(error) => Err(TransactionExecutionError::StateError(error)),
    }
}

/// Determines whether L2 gas accounting should be enabled for fee estimation.
///
/// Starknet 0.13.4 introduced runtime L2 gas accounting, which is only enabled
/// if both the caller and the callee contract classes were compiled as Sierra 1.7.
/// This function checks whether the sender contract meets this requirement.
pub fn is_l2_gas_accounting_enabled(
    transaction: &Transaction,
    state: &mut TransactionalState<'_, CachedState<JunoStateReader>>,
    block_context: &BlockContext,
    gas_computation_mode: &GasVectorComputationMode,
) -> StateResult<bool> {
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
    match &transaction {
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

pub fn preprocess_transaction(
    txn_and_query_bit: &TxnAndQueryBit,
    classes: &mut Vec<Box<serde_json::value::RawValue>>,
    paid_fees_on_l1: &mut Vec<Box<Fee>>,
    charge_fee: bool,
    validate: bool,
) -> Result<Transaction, String> {
    let class_info = match txn_and_query_bit.txn {
        StarknetApiTransaction::Declare(_) => {
            if classes.is_empty() {
                return Err("missing declared class".to_string());
            }
            let class_json_str = classes.remove(0);
            let maybe_cc = class_info_from_json_str(class_json_str.get());
            match maybe_cc {
                Ok(cc) => Some(cc),
                Err(e) => return Err(e),
            }
        }
        _ => None,
    };

    let paid_fee_on_l1 = match txn_and_query_bit.txn {
        StarknetApiTransaction::L1Handler(_) => {
            if paid_fees_on_l1.is_empty() {
                return Err("missing fee paid on l1b".to_string());
            }
            Some(*paid_fees_on_l1.remove(0))
        }
        _ => None,
    };

    let account_execution_flags = ExecutionFlags {
        only_query: txn_and_query_bit.query_bit,
        charge_fee,
        validate,
    };

    transaction_from_api(
        txn_and_query_bit.txn.clone(),
        txn_and_query_bit.txn_hash,
        class_info,
        paid_fee_on_l1,
        account_execution_flags,
    )
}

fn transaction_from_api(
    tx: StarknetApiTransaction,
    tx_hash: TransactionHash,
    class_info: Option<ClassInfo>,
    paid_fee_on_l1: Option<Fee>,
    execution_flags: ExecutionFlags,
) -> Result<Transaction, String> {
    match tx {
        StarknetApiTransaction::Deploy(_) => {
            return Err(format!(
                "Unsupported deploy transaction in the traced block (transaction_hash={})",
                tx_hash,
            ))
        }
        StarknetApiTransaction::Declare(_) if class_info.is_none() => {
            return Err(format!(
                "Declare transaction must be created with a ContractClass (transaction_hash={})",
                tx_hash,
            ))
        }
        _ => {} // all ok
    };

    Transaction::from_api(
        tx,
        tx_hash,
        class_info,
        paid_fee_on_l1,
        None,
        execution_flags,
    )
    .map_err(|err| format!("failed to create transaction from api: {:?}", err))
}

enum SimulationError {
    OutOfGas,
    ExecutionError(TransactionExecutionError),
}

/// The margin used in binary search for finding the minimal L2 gas limit.
const L2_GAS_SEARCH_MARGIN: GasAmount = GasAmount(1_000_000);

/// Determines the optimal L2 gas limit required for a transaction to execute successfully.
/// If the required gas exceeds the initial limit, the transaction is reverted.
fn determine_l2_gas_limit_and_execute(
    transaction: &mut Transaction,
    state: &mut TransactionalState<'_, CachedState<JunoStateReader>>,
    block_context: &blockifier::context::BlockContext,
) -> Result<TransactionExecutionInfo, TransactionExecutionError> {
    let initial_gas_limit = extract_l2_gas_limit(transaction);

    // Simulate transaction execution with maximum possible gas to get actual gas usage.
    set_l2_gas_limit(transaction, GasAmount::MAX);
    let simulation_result = match simulate_execution(transaction, state, block_context) {
        Ok(info) => info,
        Err(SimulationError::ExecutionError(error)) => return Err(error),
        Err(SimulationError::OutOfGas) => {
            let resource_bounds = extract_l2_gas_limit(transaction);
            return Err(TransactionExecutionError::FeeCheckError(
                FeeCheckError::MaxGasAmountExceeded {
                    resource: starknet_api::transaction::fields::Resource::L2Gas,
                    max_amount: GasAmount::MAX,
                    actual_amount: resource_bounds,
                },
            ));
        }
    };

    let GasAmount(gas_used) = simulation_result.receipt.gas.l2_gas;

    // Add a 10% buffer to the actual gas usage to prevent underestimation.
    let l2_gas_adjusted = GasAmount(gas_used.saturating_add(gas_used / 10));
    set_l2_gas_limit(transaction, l2_gas_adjusted);

    // Initialize binary search bounds.
    let mut lower_bound = GasAmount(gas_used);
    let mut upper_bound = GasAmount::MAX;
    let mut current_limit = calculate_midpoint(lower_bound, upper_bound);

    // Perform binary search to find the minimum gas limit required.
    loop {
        set_l2_gas_limit(transaction, current_limit);
        let bounds_diff = upper_bound
            .checked_sub(lower_bound)
            .expect("Upper bound >= lower bound");

        // Prevent infinite loop when search is at minimal granularity.
        if bounds_diff == GasAmount(1) && current_limit == lower_bound {
            lower_bound = upper_bound;
            current_limit = upper_bound;
        }

        match simulate_execution(transaction, state, block_context) {
            Ok(_) => {
                // If search is complete within margin, break loop.
                if is_search_complete(lower_bound, upper_bound, L2_GAS_SEARCH_MARGIN) {
                    break;
                }
                upper_bound = current_limit;
            }
            Err(SimulationError::OutOfGas) => {
                lower_bound = current_limit;
            }
            Err(SimulationError::ExecutionError(error)) => return Err(error),
        }
        current_limit = calculate_midpoint(lower_bound, upper_bound);
    }

    // If the computed gas limit exceeds the initial limit, revert the transaction.
    if current_limit > initial_gas_limit {
        tracing::debug!(
            initial_limit=%initial_gas_limit,
            final_limit=%current_limit,
            "Exceeded initial L2 gas limit"
        );
        set_l2_gas_limit(transaction, GasAmount::ZERO);
        return transaction.execute(state, block_context);
    }

    // Execute the transaction with the determined gas limit and update the estimate.
    let mut execution_info = execute_transaction(transaction, state, block_context)
        .expect("Transaction should have already succeeded");
    execution_info.receipt.gas.l2_gas = current_limit;

    Ok(execution_info)
}

fn calculate_midpoint(a: GasAmount, b: GasAmount) -> GasAmount {
    let GasAmount(a) = a;
    let GasAmount(b) = b;
    let distance = b.checked_sub(a).expect("b >= a");
    GasAmount(a + distance / 2)
}

fn is_search_complete(lower: GasAmount, upper: GasAmount, margin: GasAmount) -> bool {
    upper
        .checked_sub(lower)
        .expect("Upper bound must be greater than lower bound")
        <= margin
}

fn simulate_execution(
    transaction: &Transaction,
    state: &mut TransactionalState<'_, CachedState<JunoStateReader>>,
    block_context: &blockifier::context::BlockContext,
) -> Result<TransactionExecutionInfo, SimulationError> {
    let mut simulated_state = CachedState::<_>::create_transactional(state);
    match transaction.execute(&mut simulated_state, block_context) {
        Ok(info) if is_out_of_gas(&info) => Err(SimulationError::OutOfGas),
        Ok(info) => Ok(info),
        Err(error) => Err(SimulationError::ExecutionError(error)),
    }
}

/// Updates the L2 gas limit for a given transaction.
fn set_l2_gas_limit(transaction: &mut Transaction, gas_limit: GasAmount) {
    if let Transaction::Account(ref mut account_transaction) = transaction {
        match &mut account_transaction.tx {
            AccountTransaction::Declare(ref mut tx) => {
                if let DeclareTransaction::V3(ref mut tx) = &mut tx.tx {
                    match &mut tx.resource_bounds {
                        ValidResourceBounds::L1Gas(_) => {}
                        ValidResourceBounds::AllResources(ref mut all_resource_bounds) => {
                            all_resource_bounds.l2_gas.max_amount = gas_limit;
                            return;
                        }
                    }
                }
            }
            AccountTransaction::DeployAccount(ref mut tx) => {
                if let DeployAccountTransaction::V3(ref mut tx) = &mut tx.tx {
                    match &mut tx.resource_bounds {
                        ValidResourceBounds::L1Gas(_) => {}
                        ValidResourceBounds::AllResources(ref mut all_resource_bounds) => {
                            all_resource_bounds.l2_gas.max_amount = gas_limit;
                            return;
                        }
                    }
                }
            }
            AccountTransaction::Invoke(tx) => {
                if let InvokeTransaction::V3(ref mut tx) = &mut tx.tx {
                    match &mut tx.resource_bounds {
                        ValidResourceBounds::L1Gas(_) => {}
                        ValidResourceBounds::AllResources(ref mut all_resource_bounds) => {
                            all_resource_bounds.l2_gas.max_amount = gas_limit;
                            return;
                        }
                    }
                }
            }
        }
        unreachable!();
    }
}

/// Extracts the L2 gas limit from a transaction.
fn extract_l2_gas_limit(transaction: &Transaction) -> GasAmount {
    if let Transaction::Account(account_transaction) = transaction {
        use starknet_api::executable_transaction::AccountTransaction;
        use starknet_api::transaction::fields::ValidResourceBounds;

        match &account_transaction.tx {
            AccountTransaction::Declare(tx) => {
                use starknet_api::transaction::DeclareTransaction;
                if let DeclareTransaction::V3(tx) = &tx.tx {
                    match &tx.resource_bounds {
                        ValidResourceBounds::L1Gas(_) => {}
                        ValidResourceBounds::AllResources(all_resource_bounds) => {
                            return all_resource_bounds.l2_gas.max_amount;
                        }
                    }
                }
            }
            AccountTransaction::DeployAccount(tx) => {
                use starknet_api::transaction::DeployAccountTransaction;
                if let DeployAccountTransaction::V3(tx) = &tx.tx {
                    match &tx.resource_bounds {
                        ValidResourceBounds::L1Gas(_) => {}
                        ValidResourceBounds::AllResources(all_resource_bounds) => {
                            return all_resource_bounds.l2_gas.max_amount;
                        }
                    }
                }
            }
            AccountTransaction::Invoke(tx) => {
                use starknet_api::transaction::InvokeTransaction;
                if let InvokeTransaction::V3(tx) = &tx.tx {
                    match &tx.resource_bounds {
                        ValidResourceBounds::L1Gas(_) => {}
                        ValidResourceBounds::AllResources(all_resource_bounds) => {
                            return all_resource_bounds.l2_gas.max_amount;
                        }
                    }
                }
            }
        }
    }

    // This function should only be called with account transaction versions that
    // have L2 gas. It's a pain to set it up through the type system, so we'll
    // just return early in expected cases (see match above) and panic if we get
    // here.
    unreachable!();
}

/// Checks if the transaction failed due to insufficient L2 gas.
fn is_out_of_gas(execution_info: &TransactionExecutionInfo) -> bool {
    let Some(revert_error) = &execution_info.revert_error else {
        return false;
    };
    revert_error.to_string().contains("Out of gas") // TODO: Find a type-safe way to check this
}
