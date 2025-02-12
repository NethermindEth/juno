use std::ffi::{c_char, c_uchar, c_void, CStr};

use crate::block_context::build_block_context;
use crate::errors::report_error;
use crate::ffi::{
    JunoAddExecutionSteps, JunoAppendActualFee, JunoAppendDAGas, JunoAppendGasConsumed,
    JunoAppendTrace,
};
use crate::types::BlockInfo;
use crate::{
    jsonrpc::{new_transaction_trace, TransactionTrace},
    juno_state_reader::{class_info_from_json_str, felt_to_byte_array, JunoStateReader},
};
use blockifier::fee::gas_usage::estimate_minimal_gas_vector;
use blockifier::state::cached_state::TransactionalState;
use blockifier::transaction::objects::{TransactionExecutionInfo, TransactionExecutionResult};
use blockifier::{
    context::BlockContext,
    fee::fee_utils::get_fee_by_gas_vector,
    state::cached_state::CachedState,
    transaction::{
        account_transaction::ExecutionFlags as AccountExecutionFlags,
        errors::TransactionExecutionError::{
            ContractConstructorExecutionFailed, ExecutionError, ValidateTransactionError,
        },
        objects::HasRelatedFeeType,
        transaction_execution::Transaction,
        transactions::ExecutableTransaction,
    },
};
use serde::Deserialize;
use starknet_api::block::FeeType;
use starknet_api::{
    contract_class::ClassInfo,
    executable_transaction::AccountTransaction,
    execution_resources::GasVector,
    transaction::{
        fields::{Fee, GasVectorComputationMode},
        DeclareTransaction, DeployAccountTransaction, InvokeTransaction,
        Transaction as StarknetApiTransaction, TransactionHash,
    },
};
use std::result::Result::Ok;

#[derive(Deserialize)]
pub struct TxnAndQueryBit {
    pub txn: StarknetApiTransaction,
    pub txn_hash: TransactionHash,
    pub query_bit: bool,
}

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn cairoVMExecute(
    txns_json: *const c_char,
    classes_json: *const c_char,
    paid_fees_on_l1_json: *const c_char,
    block_info_ptr: *const BlockInfo,
    reader_handle: usize,
    chain_id: *const c_char,
    skip_charge_fee: c_uchar,
    skip_validate: c_uchar,
    err_on_revert: c_uchar,
    concurrency_mode: c_uchar,
) {
    let block_info = unsafe { *block_info_ptr };
    let reader = JunoStateReader::new(reader_handle, block_info.block_number);
    let chain_id_str = unsafe { CStr::from_ptr(chain_id) }.to_str().unwrap();

    let txns_and_query_bits: Vec<TxnAndQueryBit> = match deserialize_json(txns_json) {
        Ok(data) => data,
        Err(e) => return report_error(reader_handle, &e, -1),
    };

    let mut classes: Vec<Box<serde_json::value::RawValue>> = match deserialize_json(classes_json) {
        Ok(data) => data,
        Err(e) => return report_error(reader_handle, &e, -1),
    };

    let mut paid_fees_on_l1: Vec<Box<Fee>> = match deserialize_json(paid_fees_on_l1_json) {
        Ok(data) => data,
        Err(e) => return report_error(reader_handle, &e, -1),
    };

    let mut state = CachedState::new(reader);
    let block_context = match build_block_context(
        &mut state,
        &block_info,
        chain_id_str,
        None,
        concurrency_mode == 1,
    ) {
        Ok(context) => context,
        Err(e) => return report_error(reader_handle, &e.to_string(), -1),
    };

    let charge_fee = skip_charge_fee == 0;
    let validate = skip_validate == 0;
    let mut trace_buffer = Vec::with_capacity(10_000);

    for (txn_index, txn_and_query_bit) in txns_and_query_bits.iter().enumerate() {
        if let Err(e) = process_transaction(
            txn_and_query_bit,
            &mut state,
            &block_context,
            &mut classes,
            &mut paid_fees_on_l1,
            charge_fee,
            validate,
            err_on_revert,
            reader_handle,
            &mut trace_buffer,
        ) {
            report_error(reader_handle, &e, txn_index as i64);
            return;
        }
    }
}

fn deserialize_json<T: serde::de::DeserializeOwned>(ptr: *const c_char) -> Result<T, String> {
    if ptr.is_null() {
        return Err("Null JSON pointer".to_string());
    }
    let json_str = unsafe { CStr::from_ptr(ptr) }
        .to_str()
        .map_err(|e| e.to_string())?;
    serde_json::from_str(json_str).map_err(|e| e.to_string())
}

fn process_transaction(
    txn_and_query_bit: &TxnAndQueryBit,
    state: &mut CachedState<JunoStateReader>,
    block_context: &BlockContext,
    classes: &mut Vec<Box<serde_json::value::RawValue>>,
    paid_fees_on_l1: &mut Vec<Box<Fee>>,
    charge_fee: bool,
    validate: bool,
    err_on_revert: c_uchar,
    reader_handle: usize,
    trace_buffer: &mut Vec<u8>,
) -> Result<(), String> {
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

    let account_execution_flags = AccountExecutionFlags {
        only_query: txn_and_query_bit.query_bit,
        charge_fee,
        validate,
    };

    let txn = transaction_from_api(
        txn_and_query_bit.txn.clone(),
        txn_and_query_bit.txn_hash,
        class_info,
        paid_fee_on_l1,
        account_execution_flags,
    )?;

    let mut txn_state = CachedState::create_transactional(state);
    let res = txn.execute(&mut txn_state, block_context);
    let gas_usage_vector_computation_mode = determine_gas_vector_mode(&txn);
    let (minimal_gas_vector, fee_type) = match txn {
        Transaction::Account(account_tx) => (
            estimate_minimal_gas_vector(
                block_context,
                &account_tx,
                &gas_usage_vector_computation_mode,
            ),
            account_tx.fee_type(),
        ),

        Transaction::L1Handler(l1_handler_tx) => (GasVector::default(), l1_handler_tx.fee_type()),
    };

    handle_execution_result(
        res,
        txn_and_query_bit,
        reader_handle,
        trace_buffer,
        &mut txn_state,
        err_on_revert,
        minimal_gas_vector,
        fee_type,
        block_context,
    )?;

    Ok(txn_state.commit())
}

fn handle_execution_result(
    res: TransactionExecutionResult<TransactionExecutionInfo>,
    txn_and_query_bit: &TxnAndQueryBit,
    reader_handle: usize,
    trace_buffer: &mut Vec<u8>,
    txn_state: &mut TransactionalState<'_, CachedState<JunoStateReader>>,
    err_on_revert: c_uchar,
    minimal_gas_vector: GasVector,
    fee_type: FeeType,
    block_context: &BlockContext,
) -> Result<(), String> {
    match res {
        Err(error) => {
            let err_string = match &error {
                ContractConstructorExecutionFailed(e) => format!("{error} {e}"),
                ExecutionError { error: e, .. } | ValidateTransactionError { error: e, .. } => {
                    format!("{error} {e}")
                }
                other => other.to_string(),
            };
            return Err(format!(
                "failed txn {} reason: {}",
                txn_and_query_bit.txn_hash, err_string,
            ));
        }
        Ok(mut tx_execution_info) => {
            if tx_execution_info
                .execute_call_info
                .as_ref()
                .map_or(false, |info| info.execution.failed)
            {
                return Err(format!("failed call info {}", txn_and_query_bit.txn_hash));
            }

            if tx_execution_info.is_reverted() && err_on_revert != 0 {
                return Err(format!(
                    "reverted: {}",
                    tx_execution_info.revert_error.unwrap()
                ));
            }

            override_fee_calculation(
                &mut tx_execution_info,
                minimal_gas_vector,
                block_context,
                &fee_type,
            );

            append_juno_data(reader_handle, &tx_execution_info);
            let trace =
                match new_transaction_trace(&txn_and_query_bit.txn, tx_execution_info, txn_state) {
                    Ok(trace) => trace,
                    Err(e) => {
                        return Err(format!(
                            "failed building txn trace reason: {:?}",
                            e.to_string()
                        ))
                    }
                };
            append_trace(reader_handle, &trace, trace_buffer);
        }
    };
    Ok(())
}

fn override_fee_calculation(
    tx_execution_info: &mut TransactionExecutionInfo,
    minimal_gas_vector: GasVector,
    block_context: &BlockContext,
    fee_type: &FeeType,
) {
    if tx_execution_info.receipt.fee.0 == 0 {
        let gas_vector = GasVector {
            l1_gas: tx_execution_info
                .receipt
                .gas
                .l1_gas
                .max(minimal_gas_vector.l1_gas),
            l1_data_gas: tx_execution_info
                .receipt
                .gas
                .l1_data_gas
                .max(minimal_gas_vector.l1_data_gas),
            l2_gas: tx_execution_info
                .receipt
                .gas
                .l2_gas
                .max(minimal_gas_vector.l2_gas),
        };
        tx_execution_info.receipt.fee =
            get_fee_by_gas_vector(block_context.block_info(), gas_vector, fee_type);
    }
}

fn append_juno_data(reader_handle: usize, tx_execution_info: &TransactionExecutionInfo) {
    let actual_fee = tx_execution_info.receipt.fee.0.into();
    let da_gas_l1_gas = tx_execution_info.receipt.da_gas.l1_gas.into();
    let da_gas_l1_data_gas = tx_execution_info.receipt.da_gas.l1_data_gas.into();
    let execution_steps = tx_execution_info
        .receipt
        .resources
        .computation
        .vm_resources
        .n_steps
        .try_into()
        .unwrap_or(u64::MAX);
    let l1_gas_consumed = tx_execution_info.receipt.gas.l1_gas.into();
    let l1_data_gas_consumed = tx_execution_info.receipt.gas.l1_data_gas.into();
    let l2_gas_consumed = tx_execution_info.receipt.gas.l2_gas.into();

    unsafe {
        JunoAppendActualFee(reader_handle, felt_to_byte_array(&actual_fee).as_ptr());
        JunoAppendDAGas(
            reader_handle,
            felt_to_byte_array(&da_gas_l1_gas).as_ptr(),
            felt_to_byte_array(&da_gas_l1_data_gas).as_ptr(),
        );
        JunoAppendGasConsumed(
            reader_handle,
            felt_to_byte_array(&l1_gas_consumed).as_ptr(),
            felt_to_byte_array(&l1_data_gas_consumed).as_ptr(),
            felt_to_byte_array(&l2_gas_consumed).as_ptr(),
        );
        JunoAddExecutionSteps(reader_handle, execution_steps);
    }
}

fn append_trace(reader_handle: usize, trace: &TransactionTrace, trace_buffer: &mut Vec<u8>) {
    trace_buffer.clear();
    serde_json::to_writer(&mut *trace_buffer, trace).unwrap();

    let ptr = trace_buffer.as_ptr();
    let len = trace_buffer.len();

    unsafe {
        JunoAppendTrace(reader_handle, ptr as *const c_void, len);
    };
}

fn transaction_from_api(
    tx: StarknetApiTransaction,
    tx_hash: TransactionHash,
    class_info: Option<ClassInfo>,
    paid_fee_on_l1: Option<Fee>,
    execution_flags: AccountExecutionFlags,
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

fn determine_gas_vector_mode(transaction: &Transaction) -> GasVectorComputationMode {
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
