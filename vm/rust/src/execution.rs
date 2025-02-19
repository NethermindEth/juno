use crate::juno_state_reader::JunoStateReader;
use blockifier::execution::contract_class::TrackedResource;
use blockifier::fee::fee_checks::FeeCheckError;
use blockifier::state::state_api::{StateReader, StateResult, UpdatableState};
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
        Ok(true) => get_gas_vector_computation_mode(txn, txn_state, block_context),
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

enum SimulationError {
    OutOfGas(GasAmount),
    ExecutionError(TransactionExecutionError),
}

/// The margin used in binary search for finding the minimal L2 gas limit.
const L2_GAS_SEARCH_MARGIN: GasAmount = GasAmount(1_000_000);

/// Determines the optimal L2 gas limit required for a transaction to execute successfully.
/// If the required gas exceeds the initial limit, the transaction is reverted.
fn get_gas_vector_computation_mode<S>(
    transaction: &mut Transaction,
    state: &mut S,
    block_context: &blockifier::context::BlockContext,
) -> Result<TransactionExecutionInfo, TransactionExecutionError>
where
    S: UpdatableState,
{
    let original_tx = transaction.clone();
    let initial_resource_bounds = get_resource_bounds(transaction)?;
    let initial_gas_limit = initial_resource_bounds.l2_gas.max_amount;
    let max_l2_gas_limit =
        get_max_l2_gas_amount_covered_by_balance(transaction, block_context, state)?;

    // Simulate transaction execution with maximum possible gas to get actual gas usage.
    set_l2_gas_limit(transaction, max_l2_gas_limit);
    let (simulation_result, _) = match simulate_execution(transaction, state, block_context) {
        Ok(info) => info,
        Err(SimulationError::ExecutionError(error)) => {
            return Err(error);
        }
        Err(SimulationError::OutOfGas(gas)) => {
            return Err(TransactionExecutionError::FeeCheckError(
                FeeCheckError::MaxGasAmountExceeded {
                    resource: starknet_api::transaction::fields::Resource::L2Gas,
                    max_amount: GasAmount::MAX,
                    actual_amount: gas,
                },
            ));
        }
    };

    let GasAmount(gas_used) = simulation_result.receipt.gas.l2_gas;

    // Add a 10% buffer to the actual gas usage to prevent underestimation.
    let l2_gas_adjusted = GasAmount(gas_used.saturating_add(gas_used / 10));
    set_l2_gas_limit(transaction, l2_gas_adjusted);

    let (l2_gas_limit, mut execution_info, tx_state) =
        match simulate_execution(transaction, state, block_context) {
            Ok((tx_info, tx_state)) => {
                // If 110% of the actual transaction gas fee is enough, we use that
                // as the estimate and skip the binary search.
                (l2_gas_adjusted, tx_info, tx_state)
            }
            Err(SimulationError::OutOfGas(_)) => {
                let mut lower_bound = GasAmount(gas_used);
                let mut upper_bound = max_l2_gas_limit;
                let mut current_l2_gas_limit = calculate_midpoint(lower_bound, upper_bound);

                // Run a binary search to find the minimal gas limit that still allows the
                // transaction to execute without running out of L2 gas.
                let (tx_info, tx_state) = loop {
                    set_l2_gas_limit(transaction, current_l2_gas_limit);

                    // Special case where the search would get stuck if `current_l2_gas_limit ==
                    // lower_bound` but the required amount is equal to the upper bound.
                    let bounds_diff = upper_bound
                        .checked_sub(lower_bound)
                        .expect("Upper bound >= lower bound");
                    if bounds_diff == GasAmount(1) && current_l2_gas_limit == lower_bound {
                        lower_bound = upper_bound;
                        current_l2_gas_limit = upper_bound;
                    }

                    match simulate_execution(transaction, state, block_context) {
                        Ok((tx_info, tx_state)) => {
                            if is_search_complete(lower_bound, upper_bound, L2_GAS_SEARCH_MARGIN) {
                                break (tx_info, tx_state);
                            }

                            upper_bound = current_l2_gas_limit;
                            current_l2_gas_limit = calculate_midpoint(lower_bound, upper_bound);
                        }
                        Err(SimulationError::OutOfGas(_)) => {
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
    if l2_gas_limit > initial_gas_limit {
        return original_tx.execute(state, block_context);
    }

    // Execute the transaction with the determined gas limit and update the estimate.
    tx_state.commit();
    execution_info.receipt.gas.l2_gas = l2_gas_limit;

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

fn simulate_execution<'a, S>(
    transaction: &Transaction,
    state: &'a mut S,
    block_context: &BlockContext,
) -> Result<(TransactionExecutionInfo, TransactionalState<'a, S>), SimulationError>
where
    S: UpdatableState,
{
    let mut simulated_state = CachedState::<_>::create_transactional(state);
    match transaction.execute(&mut simulated_state, block_context) {
        Ok(info) if is_out_of_gas(&info) => Err(SimulationError::OutOfGas(info.receipt.gas.l2_gas)),
        Ok(info) => Ok((info, simulated_state)),
        Err(error) => Err(SimulationError::ExecutionError(error)),
    }
}

fn set_l2_gas_limit(transaction: &mut Transaction, gas_limit: GasAmount) {
    if let Transaction::Account(ref mut account_transaction) = transaction {
        match &mut account_transaction.tx {
            AccountTransaction::Declare(ref mut tx) => {
                if let DeclareTransaction::V3(ref mut tx) = &mut tx.tx {
                    if let ValidResourceBounds::AllResources(ref mut all_resource_bounds) =
                        &mut tx.resource_bounds
                    {
                        all_resource_bounds.l2_gas.max_amount = gas_limit;
                        return;
                    }
                }
            }
            AccountTransaction::DeployAccount(ref mut tx) => {
                if let DeployAccountTransaction::V3(ref mut tx) = &mut tx.tx {
                    if let ValidResourceBounds::AllResources(ref mut all_resource_bounds) =
                        &mut tx.resource_bounds
                    {
                        all_resource_bounds.l2_gas.max_amount = gas_limit;
                        return;
                    }
                }
            }
            AccountTransaction::Invoke(ref mut tx) => {
                if let InvokeTransaction::V3(ref mut tx) = &mut tx.tx {
                    if let ValidResourceBounds::AllResources(ref mut all_resource_bounds) =
                        &mut tx.resource_bounds
                    {
                        all_resource_bounds.l2_gas.max_amount = gas_limit;
                        return;
                    }
                }
            }
        }
    }
    unreachable!();
}

fn get_resource_bounds(tx: &Transaction) -> Result<AllResourceBounds, TransactionExecutionError> {
    match tx {
        Transaction::Account(
            blockifier::transaction::account_transaction::AccountTransaction {
                tx:
                    starknet_api::executable_transaction::AccountTransaction::Declare(
                        starknet_api::executable_transaction::DeclareTransaction {
                            tx:
                                DeclareTransaction::V3(DeclareTransactionV3 {
                                    resource_bounds:
                                        ValidResourceBounds::AllResources(all_resources),
                                    ..
                                }),
                            ..
                        },
                    ),
                ..
            },
        ) => Ok(*all_resources),
        Transaction::Account(
            blockifier::transaction::account_transaction::AccountTransaction {
                tx:
                    starknet_api::executable_transaction::AccountTransaction::DeployAccount(
                        starknet_api::executable_transaction::DeployAccountTransaction {
                            tx:
                                DeployAccountTransaction::V3(DeployAccountTransactionV3 {
                                    resource_bounds:
                                        ValidResourceBounds::AllResources(all_resources),
                                    ..
                                }),
                            ..
                        },
                    ),
                ..
            },
        ) => Ok(*all_resources),
        Transaction::Account(
            blockifier::transaction::account_transaction::AccountTransaction {
                tx:
                    starknet_api::executable_transaction::AccountTransaction::Invoke(
                        starknet_api::executable_transaction::InvokeTransaction {
                            tx:
                                InvokeTransaction::V3(InvokeTransactionV3 {
                                    resource_bounds:
                                        ValidResourceBounds::AllResources(all_resources),
                                    ..
                                }),
                            ..
                        },
                    ),
                ..
            },
        ) => Ok(*all_resources),
        _ => unreachable!(),
    }
}

fn get_max_l2_gas_amount_covered_by_balance<S>(
    tx: &Transaction,
    block_context: &blockifier::context::BlockContext,
    state: &mut S,
) -> Result<GasAmount, TransactionExecutionError>
where
    S: UpdatableState,
{
    let initial_resource_bounds = get_resource_bounds(tx)?;
    let resource_bounds_without_l2_gas = AllResourceBounds {
        l2_gas: Default::default(),
        ..initial_resource_bounds
    };
    let max_possible_fee_without_l2_gas =
        ValidResourceBounds::AllResources(resource_bounds_without_l2_gas).max_possible_fee();

    match tx {
        Transaction::Account(account_transaction) => {
            let fee_token_address = block_context
                .chain_info()
                .fee_token_address(&account_transaction.fee_type());
            let balance = state
                .get_fee_token_balance(account_transaction.sender_address(), fee_token_address)?;
            let balance = (balance.1.to_biguint() << 128) + balance.0.to_biguint();

            if balance > max_possible_fee_without_l2_gas.0.into() {
                // The maximum amount of L2 gas that can be bought with the balance.
                let max_amount = (balance - max_possible_fee_without_l2_gas.0)
                    / initial_resource_bounds.l2_gas.max_price_per_unit.0;
                Ok(u64::try_from(max_amount).unwrap_or(u64::MAX).into())
            } else {
                // Balance is less than committed L1 gas and L1 data gas, tx will fail anyway.
                // Let it pass through here so that execution returns a detailed error.
                Ok(GasAmount::ZERO)
            }
        }
        Transaction::L1Handler(_) => {
            // L1 handler transactions don't have L2 gas.
            unreachable!();
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
