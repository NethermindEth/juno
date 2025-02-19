use crate::juno_state_reader::JunoStateReader;
use blockifier::execution::contract_class::TrackedResource;
use blockifier::state::state_api::{StateReader, StateResult, UpdatableState};
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
use starknet_api::transaction::fields::{GasVectorComputationMode, ValidResourceBounds};
use starknet_api::transaction::{DeclareTransaction, DeployAccountTransaction, InvokeTransaction};
use std::fmt;

#[derive(Debug)]
pub enum ExecutionError {
    TransactionExecutionError(TransactionExecutionError),
    CustomError(anyhow::Error),
}

impl fmt::Display for ExecutionError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ExecutionError::TransactionExecutionError(err) => {
                write!(f, "Transaction execution error: {}", err)
            }
            ExecutionError::CustomError(err) => write!(f, "Custom error: {}", err),
        }
    }
}

impl From<TransactionExecutionError> for ExecutionError {
    fn from(err: TransactionExecutionError) -> Self {
        ExecutionError::TransactionExecutionError(err)
    }
}

impl From<anyhow::Error> for ExecutionError {
    fn from(err: anyhow::Error) -> Self {
        ExecutionError::CustomError(err)
    }
}

#[derive(Debug)]
enum SimulationError {
    OutOfGas(GasAmount),
    ExecutionError(TransactionExecutionError),
}

impl fmt::Display for SimulationError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            SimulationError::OutOfGas(gas) => write!(f, "Out of gas: {}", gas),
            SimulationError::ExecutionError(err) => write!(f, "Execution error: {}", err),
        }
    }
}

impl From<TransactionExecutionError> for SimulationError {
    fn from(err: TransactionExecutionError) -> Self {
        SimulationError::ExecutionError(err)
    }
}

pub fn execute_transaction(
    txn: &mut Transaction,
    txn_state: &mut TransactionalState<'_, CachedState<JunoStateReader>>,
    block_context: &BlockContext,
) -> Result<TransactionExecutionInfo, ExecutionError> {
    match is_l2_gas_accounting_enabled(
        txn,
        txn_state,
        block_context,
        &determine_gas_vector_mode(txn),
    ) {
        Ok(true) => get_gas_vector_computation_mode(txn, txn_state, block_context),
        Ok(false) => txn
            .execute(txn_state, block_context)
            .map_err(ExecutionError::from),
        Err(error) => Err(ExecutionError::TransactionExecutionError(
            TransactionExecutionError::StateError(error),
        )),
    }
}

pub fn is_l2_gas_accounting_enabled(
    transaction: &Transaction,
    state: &mut TransactionalState<'_, CachedState<JunoStateReader>>,
    block_context: &BlockContext,
    gas_computation_mode: &GasVectorComputationMode,
) -> StateResult<bool> {
    let sender_class_hash = state.get_class_hash_at(transaction.sender_address())?;

    if sender_class_hash == ClassHash::default() {
        return Ok(false);
    }

    let min_sierra_version = &block_context
        .versioned_constants()
        .min_sierra_version_for_sierra_gas;
    let sender_tracked_resource = state
        .get_compiled_class(sender_class_hash)?
        .tracked_resource(min_sierra_version, None);

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

const L2_GAS_SEARCH_MARGIN: GasAmount = GasAmount(1_000_000);

fn get_gas_vector_computation_mode<S>(
    transaction: &mut Transaction,
    state: &mut S,
    block_context: &blockifier::context::BlockContext,
) -> Result<TransactionExecutionInfo, ExecutionError>
where
    S: UpdatableState,
{
    let initial_gas_limit = extract_l2_gas_limit(transaction)?;
    let mut original_transaction = transaction.clone();

    set_l2_gas_limit(transaction, GasAmount::MAX)?;
    if let Transaction::Account(account_transaction) = transaction {
        account_transaction.execution_flags.charge_fee = false;
        account_transaction.execution_flags.validate = false;
    }

    let simulation_result = match simulate_execution(transaction, state, block_context) {
        Ok((tx_info, _)) => tx_info,
        Err(SimulationError::ExecutionError(error)) => {
            return Err(ExecutionError::TransactionExecutionError(error))
        }
        Err(SimulationError::OutOfGas(gas)) => {
            return Err(ExecutionError::CustomError(anyhow::anyhow!(
                "Transaction ran out of gas during simulation: {}",
                gas
            )));
        }
    };

    let GasAmount(gas_used) = simulation_result.receipt.gas.l2_gas;
    let l2_gas_adjusted = GasAmount(gas_used.saturating_add(gas_used / 10));
    set_l2_gas_limit(transaction, l2_gas_adjusted)?;

    let (l2_gas_limit, _, tx_state) = match simulate_execution(transaction, state, block_context) {
        Ok((tx_info, tx_state)) => {
            // If 110% of the actual transaction gas fee is enough, we use that
            // as the estimate and skip the binary search.
            (l2_gas_adjusted, tx_info, tx_state)
        }
        Err(SimulationError::OutOfGas(_)) => {
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
                    Err(SimulationError::ExecutionError(error)) => {
                        return Err(ExecutionError::TransactionExecutionError(error))
                    }
                }
            };

            (current_l2_gas_limit, tx_info, tx_state)
        }
        Err(SimulationError::ExecutionError(error)) => {
            return Err(ExecutionError::TransactionExecutionError(error))
        }
    };
    tx_state.abort();

    if l2_gas_limit > initial_gas_limit {
        set_l2_gas_limit(&mut original_transaction, GasAmount(0))?;
        return original_transaction
            .execute(state, block_context)
            .map_err(ExecutionError::from);
    }

    set_l2_gas_limit(&mut original_transaction, initial_gas_limit)?;
    let mut exec_info = original_transaction.execute(state, block_context)?;

    // Execute the transaction with the determined gas limit and update the estimate.
    exec_info.receipt.gas.l2_gas = l2_gas_limit;

    Ok(exec_info)
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

fn set_l2_gas_limit(
    transaction: &mut Transaction,
    gas_limit: GasAmount,
) -> Result<(), anyhow::Error> {
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
    Err(anyhow::anyhow!("Failed to set L2 gas limit"))
}

fn extract_l2_gas_limit(transaction: &Transaction) -> Result<GasAmount, anyhow::Error> {
    if let Transaction::Account(account_transaction) = transaction {
        match &account_transaction.tx {
            AccountTransaction::Declare(tx) => {
                if let DeclareTransaction::V3(tx) = &tx.tx {
                    if let ValidResourceBounds::AllResources(all_resource_bounds) =
                        &tx.resource_bounds
                    {
                        return Ok(all_resource_bounds.l2_gas.max_amount);
                    }
                }
            }
            AccountTransaction::DeployAccount(tx) => {
                if let DeployAccountTransaction::V3(tx) = &tx.tx {
                    if let ValidResourceBounds::AllResources(all_resource_bounds) =
                        &tx.resource_bounds
                    {
                        return Ok(all_resource_bounds.l2_gas.max_amount);
                    }
                }
            }
            AccountTransaction::Invoke(tx) => {
                if let InvokeTransaction::V3(tx) = &tx.tx {
                    if let ValidResourceBounds::AllResources(all_resource_bounds) =
                        &tx.resource_bounds
                    {
                        return Ok(all_resource_bounds.l2_gas.max_amount);
                    }
                }
            }
        }
    }
    Err(anyhow::anyhow!("Failed to extract L2 gas limit"))
}

fn is_out_of_gas(execution_info: &TransactionExecutionInfo) -> bool {
    if let Some(revert_error) = &execution_info.revert_error {
        revert_error.to_string().contains("Out of gas")
    } else {
        false
    }
}
