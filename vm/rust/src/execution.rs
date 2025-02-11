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
use blockifier::{
    context::BlockContext,
    fee::{fee_utils::get_fee_by_gas_vector, gas_usage},
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
use starknet_types_core::felt::Felt;

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
    let txn_json_str = unsafe { CStr::from_ptr(txns_json) }.to_str().unwrap();
    let txns_and_query_bits: Result<Vec<TxnAndQueryBit>, serde_json::Error> =
        serde_json::from_str(txn_json_str);
    if let Err(e) = txns_and_query_bits {
        report_error(reader_handle, e.to_string().as_str(), -1);
        return;
    }

    let mut classes: Result<Vec<Box<serde_json::value::RawValue>>, serde_json::Error> = Ok(vec![]);
    if !classes_json.is_null() {
        let classes_json_str = unsafe { CStr::from_ptr(classes_json) }.to_str().unwrap();
        classes = serde_json::from_str(classes_json_str);
    }
    if let Err(e) = classes {
        report_error(reader_handle, e.to_string().as_str(), -1);
        return;
    }

    let paid_fees_on_l1_json_str = unsafe { CStr::from_ptr(paid_fees_on_l1_json) }
        .to_str()
        .unwrap();
    let mut paid_fees_on_l1: Vec<Box<Fee>> = match serde_json::from_str(paid_fees_on_l1_json_str) {
        Ok(f) => f,
        Err(e) => {
            report_error(reader_handle, e.to_string().as_str(), -1);
            return;
        }
    };

    let mut state = CachedState::new(reader);
    let txns_and_query_bits = txns_and_query_bits.unwrap();
    let mut classes = classes.unwrap();
    let concurrency_mode = concurrency_mode == 1;
    let block_context: BlockContext = build_block_context(
        &mut state,
        &block_info,
        chain_id_str,
        None,
        concurrency_mode,
    )
    .unwrap();
    let charge_fee = skip_charge_fee == 0;
    let validate = skip_validate == 0;

    let mut trace_buffer = Vec::with_capacity(10_000);

    for (txn_index, txn_and_query_bit) in txns_and_query_bits.iter().enumerate() {
        let class_info = match txn_and_query_bit.txn.clone() {
            StarknetApiTransaction::Declare(_) => {
                if classes.is_empty() {
                    report_error(reader_handle, "missing declared class", txn_index as i64);
                    return;
                }
                let class_json_str = classes.remove(0);

                let maybe_cc = class_info_from_json_str(class_json_str.get());
                if let Err(e) = maybe_cc {
                    report_error(reader_handle, e.to_string().as_str(), txn_index as i64);
                    return;
                }
                Some(maybe_cc.unwrap())
            }
            _ => None,
        };

        let paid_fee_on_l1: Option<Fee> = match txn_and_query_bit.txn.clone() {
            StarknetApiTransaction::L1Handler(_) => {
                if paid_fees_on_l1.is_empty() {
                    report_error(reader_handle, "missing fee paid on l1b", txn_index as i64);
                    return;
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
        );
        if let Err(e) = txn {
            report_error(reader_handle, e.to_string().as_str(), txn_index as i64);
            return;
        }

        let mut txn_state = CachedState::create_transactional(&mut state);
        let minimal_gas_vector: Option<GasVector>;
        let fee_type;
        let txn = txn.unwrap();
        let gas_vector_computation_mode = determine_gas_vector_mode(&txn);

        let res = match txn {
            Transaction::Account(t) => {
                minimal_gas_vector = Some(gas_usage::estimate_minimal_gas_vector(
                    &block_context,
                    &t,
                    &gas_vector_computation_mode,
                ));
                fee_type = t.fee_type();
                t.execute(&mut txn_state, &block_context)
            }
            Transaction::L1Handler(t) => {
                minimal_gas_vector = None;
                fee_type = t.fee_type();
                t.execute(&mut txn_state, &block_context)
            }
        };
        match res {
            Err(error) => {
                let err_string = match &error {
                    ContractConstructorExecutionFailed(e) => format!("{error} {e}"),
                    ExecutionError { error: e, .. } | ValidateTransactionError { error: e, .. } => {
                        format!("{error} {e}")
                    }
                    other => other.to_string(),
                };
                report_error(
                    reader_handle,
                    format!(
                        "failed txn {} reason: {}",
                        txn_and_query_bit.txn_hash, err_string,
                    )
                    .as_str(),
                    txn_index as i64,
                );
                return;
            }
            Ok(mut tx_execution_info) => {
                if let Some(call_info) = &tx_execution_info.execute_call_info {
                    if call_info.execution.failed {
                        report_error(
                            reader_handle,
                            format!("failed call info {}", txn_and_query_bit.txn_hash).as_str(),
                            txn_index as i64,
                        );
                        return;
                    }
                }
                if tx_execution_info.is_reverted() && err_on_revert != 0 {
                    report_error(
                        reader_handle,
                        format!("reverted: {}", tx_execution_info.revert_error.unwrap()).as_str(),
                        txn_index as i64,
                    );
                    return;
                }

                // we are estimating fee, override actual fee calculation
                if tx_execution_info.receipt.fee.0 == 0 {
                    let minimal_gas_vector = minimal_gas_vector.unwrap_or_default();
                    let l1_gas_consumed = tx_execution_info
                        .receipt
                        .gas
                        .l1_gas
                        .max(minimal_gas_vector.l1_gas);
                    let l1_data_gas_consumed = tx_execution_info
                        .receipt
                        .gas
                        .l1_data_gas
                        .max(minimal_gas_vector.l1_data_gas);
                    let l2_gas_consumed = tx_execution_info
                        .receipt
                        .gas
                        .l2_gas
                        .max(minimal_gas_vector.l2_gas);

                    tx_execution_info.receipt.fee = fee_utils::get_fee_by_gas_vector(
                        block_context.block_info(),
                        GasVector {
                            l1_data_gas: l1_data_gas_consumed,
                            l1_gas: l1_gas_consumed,
                            l2_gas: l2_gas_consumed,
                        },
                        &fee_type,
                    )
                }

                let actual_fee: Felt = tx_execution_info.receipt.fee.0.into();
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

                let trace = jsonrpc::new_transaction_trace(
                    &txn_and_query_bit.txn,
                    tx_execution_info,
                    &mut txn_state,
                );
                if let Err(e) = trace {
                    report_error(
                        reader_handle,
                        format!("failed building txn state diff reason: {:?}", e).as_str(),
                        txn_index as i64,
                    );
                    return;
                }

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
                    JunoAddExecutionSteps(reader_handle, execution_steps)
                }
                append_trace(reader_handle, trace.as_ref().unwrap(), &mut trace_buffer);
            }
        }
        txn_state.commit();
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
