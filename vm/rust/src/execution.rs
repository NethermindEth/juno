use crate::error::ExecutionError;
use crate::juno_state_reader::JunoStateReader;
use anyhow::{anyhow, Error};
use blockifier::execution::contract_class::TrackedResource;
use blockifier::state::state_api::{StateReader, StateResult, UpdatableState};
use blockifier::transaction::account_transaction::ExecutionFlags;
use blockifier::transaction::objects::HasRelatedFeeType;
use blockifier::transaction::transaction_execution::Transaction;
use blockifier::transaction::transactions::ExecutableTransaction;
use blockifier::{
    context::BlockContext,
    state::cached_state::{CachedState, TransactionalState},
    transaction::{errors::TransactionExecutionError, objects::TransactionExecutionInfo},
};
use starknet_api::core::ClassHash;
use starknet_api::executable_transaction::AccountTransaction;
use starknet_api::execution_resources::GasAmount;
use starknet_api::transaction::fields::{
    AllResourceBounds, GasVectorComputationMode, ValidResourceBounds,
};
use starknet_api::transaction::{
    DeclareTransaction, DeclareTransactionV3, DeployAccountTransaction, DeployAccountTransactionV3,
    InvokeTransaction, InvokeTransactionV3,
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

pub(crate) fn execute_transaction<S>(
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

fn determine_gas_vector_mode(transaction: &Transaction) -> GasVectorComputationMode {
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

/// The margin used in binary search for finding the minimal L2 gas limit.
const L2_GAS_SEARCH_MARGIN: GasAmount = GasAmount(1_000_000);
enum SimulationError {
    OutOfGas,
    ExecutionError(ExecutionError),
}

/// Gets the default maximum Sierra gas limit from blockifier's versioned constants.
fn get_default_max_sierra_gas_limit(block_context: &BlockContext) -> GasAmount {
    let max_sierra_gas_limit = block_context
        .versioned_constants()
        .os_constants
        .execute_max_sierra_gas
        .0;
    GasAmount::from(max_sierra_gas_limit)
}

/// Determines the optimal L2 gas limit required for a transaction to execute successfully.
/// If the required gas exceeds the initial limit, the transaction is reverted.
fn execute_transaction_with_binary_search<S>(
    transaction: &mut Transaction,
    state: &mut S,
    block_context: &blockifier::context::BlockContext,
    error_on_revert: bool,
) -> Result<TransactionExecutionInfo, ExecutionError>
where
    S: UpdatableState,
{
    let initial_resource_bounds = extract_resource_bounds(transaction)?;
    let initial_gas_limit = initial_resource_bounds.l2_gas.max_amount;
    
    // Use balance-dependent limit only when charge_fee is enabled,
    // otherwise use blockifier's default limit to avoid the account balance issue
    let max_l2_gas_limit = if get_execution_flags(transaction).charge_fee {
        calculate_max_l2_gas_covered(transaction, block_context, state)?
    } else {
        get_default_max_sierra_gas_limit(block_context)
    };

    // Simulate transaction execution with maximum possible gas to get actual gas usage.
    set_l2_gas_limit(transaction, max_l2_gas_limit)?;

    let simulation_result =
        match simulate_execution(transaction, state, block_context, error_on_revert) {
            Ok((tx_info, _)) => tx_info,
            Err(SimulationError::ExecutionError(error)) => return Err(error),
            Err(SimulationError::OutOfGas) => {
                return Err(ExecutionError::Custom(
                    "Transaction ran out of gas during simulation even with MAX gas limit"
                        .to_string(),
                ));
            }
        };

    let GasAmount(gas_used) = simulation_result.receipt.gas.l2_gas;

    // Add a 10% buffer to the actual gas usage to prevent underestimation.
    let l2_gas_adjusted = GasAmount(gas_used.saturating_add(gas_used / 10));
    set_l2_gas_limit(transaction, l2_gas_adjusted)?;

    let (l2_gas_limit, _, tx_state) =
        match simulate_execution(transaction, state, block_context, error_on_revert) {
            Ok((tx_info, tx_state)) => {
                // If 110% of the actual transaction gas fee is enough, we use that
                // as the estimate and skip the binary search.
                (l2_gas_adjusted, tx_info, tx_state)
            }
            Err(SimulationError::OutOfGas) => {
                let mut lower_bound = GasAmount(gas_used);
                let mut upper_bound = GasAmount::MAX;
                let mut current_l2_gas_limit = calculate_midpoint(lower_bound, upper_bound);

                // Run a binary search to find the minimal gas limit that still allows the
                // transaction to execute without running out of L2 gas.
                let (tx_info, tx_state) = loop {
                    set_l2_gas_limit(transaction, current_l2_gas_limit)?;

                    // Special case where the search would get stuck if `current_l2_gas_limit ==
                    // lower_bound` but the required amount is equal to the upper bound.
                    let bounds_diff = upper_bound
                        .checked_sub(lower_bound)
                        .expect("Upper bound >= lower bound");
                    if bounds_diff == GasAmount(1) && current_l2_gas_limit == lower_bound {
                        lower_bound = upper_bound;
                        current_l2_gas_limit = upper_bound;
                    }

                    match simulate_execution(transaction, state, block_context, error_on_revert) {
                        Ok((tx_info, tx_state)) => {
                            if is_search_complete(lower_bound, upper_bound, L2_GAS_SEARCH_MARGIN) {
                                break (tx_info, tx_state);
                            }

                            upper_bound = current_l2_gas_limit;
                            current_l2_gas_limit = calculate_midpoint(lower_bound, upper_bound);
                        }
                        Err(SimulationError::OutOfGas) => {
                            lower_bound = current_l2_gas_limit;
                            current_l2_gas_limit = calculate_midpoint(lower_bound, upper_bound);
                        }
                        Err(SimulationError::ExecutionError(error)) => return Err(error),
                    }
                };
                (current_l2_gas_limit, tx_info, tx_state)
            }
            Err(SimulationError::ExecutionError(error)) => return Err(error),
        };

    // If the computed gas limit exceeds the initial limit, revert the transaction.
    // The L2 gas limit is set to zero to prevent the transaction execution from succeeding
    // in the case where the user defined gas limit is less than the required gas limit
    if get_execution_flags(transaction).charge_fee && l2_gas_limit > initial_gas_limit {
        tx_state.abort();
        set_l2_gas_limit(transaction, initial_gas_limit)?;
        return execute_transaction(&transaction, state, block_context, error_on_revert);
    }

    let mut exec_info = execute_transaction(&transaction, state, block_context, error_on_revert)?;

    // Execute the transaction with the determined gas limit and update the estimate.
    exec_info.receipt.gas.l2_gas = l2_gas_limit;

    Ok(exec_info)
}

fn get_execution_flags(tx: &Transaction) -> ExecutionFlags {
    match tx {
        Transaction::Account(account_transaction) => account_transaction.execution_flags.clone(),
        Transaction::L1Handler(_) => Default::default(),
    }
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

fn simulate_execution<'a, S>(
    tx: &Transaction,
    state: &'a mut S,
    block_context: &blockifier::context::BlockContext,
    error_on_revert: bool,
) -> Result<(TransactionExecutionInfo, TransactionalState<'a, S>), SimulationError>
where
    S: UpdatableState,
{
    let mut tx_state = CachedState::<_>::create_transactional(state);

    match tx.execute(&mut tx_state, block_context) {
        Ok(tx_info) if is_out_of_gas(&tx_info) => Err(SimulationError::OutOfGas),
        Ok(tx_info) => {
            if tx_info.is_reverted() && error_on_revert {
                if let Some(revert_error) = tx_info.revert_error {
                    let revert_string = revert_error.to_string();
                    return Err(SimulationError::ExecutionError(
                        ExecutionError::ExecutionError {
                            error: revert_string,
                            error_stack: revert_error.into(),
                        },
                    ));
                }
            }

            Ok((tx_info, tx_state))
        }
        Err(error) => Err(SimulationError::ExecutionError(ExecutionError::new(error))),
    }
}

fn set_l2_gas_limit(transaction: &mut Transaction, gas_limit: GasAmount) -> Result<(), Error> {
    if let Transaction::Account(ref mut account_transaction) = transaction {
        match &mut account_transaction.tx {
            AccountTransaction::Declare(ref mut tx) => {
                if let DeclareTransaction::V3(ref mut tx) = &mut tx.tx {
                    if let ValidResourceBounds::AllResources(ref mut all_resource_bounds) =
                        &mut tx.resource_bounds
                    {
                        all_resource_bounds.l2_gas.max_amount = gas_limit;
                        return Ok(());
                    }
                }
            }
            AccountTransaction::DeployAccount(ref mut tx) => {
                if let DeployAccountTransaction::V3(ref mut tx) = &mut tx.tx {
                    if let ValidResourceBounds::AllResources(ref mut all_resource_bounds) =
                        &mut tx.resource_bounds
                    {
                        all_resource_bounds.l2_gas.max_amount = gas_limit;
                        return Ok(());
                    }
                }
            }
            AccountTransaction::Invoke(ref mut tx) => {
                if let InvokeTransaction::V3(ref mut tx) = &mut tx.tx {
                    if let ValidResourceBounds::AllResources(ref mut all_resource_bounds) =
                        &mut tx.resource_bounds
                    {
                        all_resource_bounds.l2_gas.max_amount = gas_limit;
                        return Ok(());
                    }
                }
            }
        }
    }
    Err(anyhow!("Failed to set L2 gas limit"))
}

/// Retrieves the resource bounds for a given transaction.
fn extract_resource_bounds(tx: &Transaction) -> Result<AllResourceBounds, Error> {
    match tx {
        Transaction::Account(account_tx) => match &account_tx.tx {
            AccountTransaction::Declare(declare_tx) => match &declare_tx.tx {
                DeclareTransaction::V3(DeclareTransactionV3 {
                    resource_bounds: ValidResourceBounds::AllResources(all_resources),
                    ..
                }) => Ok(*all_resources),
                _ => Err(anyhow!("Unsupported Declare transaction version")),
            },
            AccountTransaction::DeployAccount(deploy_tx) => match &deploy_tx.tx {
                DeployAccountTransaction::V3(DeployAccountTransactionV3 {
                    resource_bounds: ValidResourceBounds::AllResources(all_resources),
                    ..
                }) => Ok(*all_resources),
                _ => Err(anyhow!("Unsupported DeployAccount transaction version")),
            },
            AccountTransaction::Invoke(invoke_tx) => match &invoke_tx.tx {
                InvokeTransaction::V3(InvokeTransactionV3 {
                    resource_bounds: ValidResourceBounds::AllResources(all_resources),
                    ..
                }) => Ok(*all_resources),
                _ => Err(anyhow!("Unsupported Invoke transaction version")),
            },
        },
        _ => Err(anyhow!("Unsupported transaction type")),
    }
}

/// Calculates the maximum amount of L2 gas that can be covered by the balance.
fn calculate_max_l2_gas_covered<S>(
    tx: &Transaction,
    block_context: &blockifier::context::BlockContext,
    state: &mut S,
) -> Result<GasAmount, Error>
where
    S: UpdatableState,
{
    // Extract initial resource bounds from the transaction.
    let initial_resource_bounds = extract_resource_bounds(tx)?;

    // Create resource bounds without L2 gas.
    let resource_bounds_without_l2_gas = AllResourceBounds {
        l2_gas: Default::default(),
        ..initial_resource_bounds
    };

    // Calculate the maximum possible fee without L2 gas.
    let max_fee_without_l2_gas =
        ValidResourceBounds::AllResources(resource_bounds_without_l2_gas).max_possible_fee();

    match tx {
        Transaction::Account(account_transaction) => {
            // DEPLOY_ACCOUNT: Normal logic calculates max gas based on account balance, but account doesn't exist yet (balance = 0).
            // This would set gas limit to zero, causing immediate "out of gas". Use max gas limit from versioned constants instead.
            if is_deploy_account_transaction(tx) {
                return Ok(get_default_max_sierra_gas_limit(block_context));
            }

            // Retrieve the fee token address.
            let fee_token_address = block_context
                .chain_info()
                .fee_token_address(&account_transaction.fee_type());

            // Get the balance of the fee token.
            let balance = state
                .get_fee_token_balance(account_transaction.sender_address(), fee_token_address)?;
            let balance = (balance.1.to_biguint() << 128) + balance.0.to_biguint();

            if balance > max_fee_without_l2_gas.0.into() {
                // Calculate the maximum amount of L2 gas that can be bought with the balance.
                let max_l2_gas = (balance - max_fee_without_l2_gas.0)
                    / initial_resource_bounds
                        .l2_gas
                        .max_price_per_unit
                        .0
                        .max(1u64.into());
                Ok(u64::try_from(max_l2_gas).unwrap_or(GasAmount::MAX.0).into())
            } else {
                // Balance is less than committed L1 gas and L1 data gas, transaction will fail.
                // Let it pass through here so that execution returns a detailed error.
                Ok(GasAmount::ZERO)
            }
        }
        Transaction::L1Handler(_) => {
            // L1 handler transactions don't have L2 gas.
            Err(anyhow!("L1 handler transactions don't have L2 gas"))
        }
    }
}

fn is_out_of_gas(execution_info: &TransactionExecutionInfo) -> bool {
    if let Some(revert_error) = &execution_info.revert_error {
        revert_error.to_string().contains("Out of gas")
    } else {
        false
    }
}
