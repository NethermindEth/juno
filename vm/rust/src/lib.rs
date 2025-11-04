pub mod entrypoint;
pub mod error;
pub mod execution;
pub mod ffi_entrypoint;
pub mod ffi_type;

mod juno_state_reader;

use crate::{
    ffi_type::{
        state_diff::StateDiff,
        transaction_receipt::TransactionReceipt,
        transaction_trace::{new_transaction_trace, TransactionTrace},
    },
    juno_state_reader::{ptr_to_felt, BlockHeight, JunoStateReader},
};
use error::{
    call::CallError,
    execution::ExecutionError,
    juno::JunoError,
    stack::{ErrorStack, Frame},
};
use execution::process_transaction;
use serde::Deserialize;
use serde_json::json;
use std::{
    collections::{BTreeMap, VecDeque},
    ffi::{c_char, c_longlong, c_uchar, c_ulonglong, c_void, CStr, CString},
    slice,
    sync::Arc,
};

use blockifier::fee::{fee_utils, gas_usage};
use blockifier::{
    abi::constants::STORED_BLOCK_HASH_BUFFER,
    transaction::account_transaction::ExecutionFlags as AccountExecutionFlags,
};
use blockifier::{
    blockifier::block::pre_process_block, execution::entry_point::SierraGasRevertTracker,
};
use blockifier::{bouncer::BouncerConfig, transaction::objects::TransactionExecutionInfo};
use blockifier::{
    context::{
        BlockContext, ChainInfo as BlockifierChainInfo, FeeTokenAddresses, TransactionContext,
    },
    execution::entry_point::{CallEntryPoint, CallType, EntryPointExecutionContext},
    state::{cached_state::CachedState, state_api::State},
    transaction::{
        objects::{DeprecatedTransactionInfo, HasRelatedFeeType, TransactionInfo},
        transaction_execution::Transaction,
    },
};
use juno_state_reader::{class_info_from_json_str, felt_to_byte_array};
use starknet_api::core::{ChainId, ClassHash, ContractAddress};
use starknet_api::{
    block::{BlockHash, GasPrice, StarknetVersion},
    contract_class::{ClassInfo, EntryPointType},
    executable_transaction::AccountTransaction,
    execution_resources::GasVector,
    transaction::{
        fields::{Calldata, Fee, GasVectorComputationMode, Tip},
        DeclareTransaction, DeployAccountTransaction, InvokeTransaction,
        Transaction as StarknetApiTransaction, TransactionHash,
    },
};
use starknet_api::{
    block::{
        BlockHashAndNumber, BlockInfo as BlockifierBlockInfo, GasPriceVector, GasPrices,
        NonzeroGasPrice,
    },
    execution_resources::GasAmount,
};
use starknet_types_core::felt::Felt;

extern "C" {
    fn JunoReportError(
        reader_handle: usize,
        txnIndex: c_longlong,
        err: *const c_char,
        execution_failed: usize,
    );
    fn JunoAppendTrace(reader_handle: usize, json_trace: *const c_void, len: usize);
    fn JunoAppendStateDiff(reader_handle: usize, json_state_diff: *const c_void, len: usize);
    fn JunoAppendReceipt(reader_handle: usize, json_receipt: *const c_void, len: usize);
    fn JunoAppendResponse(reader_handle: usize, ptr: *const c_uchar);
    fn JunoAppendActualFee(reader_handle: usize, ptr: *const c_uchar);
    fn JunoAppendDAGas(reader_handle: usize, ptr: *const c_uchar, ptr: *const c_uchar);
    fn JunoAppendGasConsumed(
        reader_handle: usize,
        ptr: *const c_uchar,
        ptr: *const c_uchar,
        ptr: *const c_uchar,
    );
    fn JunoAddExecutionSteps(reader_handle: usize, execSteps: c_ulonglong);
}

fn adjust_fee_calculation_result(
    tx_execution_info: &mut TransactionExecutionInfo,
    txn: &Transaction,
    gas_vector_computation_mode: &GasVectorComputationMode,
    block_context: &BlockContext,
    txn_index: usize,
) -> Result<(), JunoError> {
    let (minimal_gas_vector, fee_type) = match &txn {
        Transaction::Account(t) => (
            Some(gas_usage::estimate_minimal_gas_vector(
                &block_context,
                t,
                &gas_vector_computation_mode,
            )),
            t.fee_type(),
        ),
        Transaction::L1Handler(t) => (None, t.fee_type()),
    };

    let minimal_gas_vector = minimal_gas_vector.unwrap_or_default();
    let mut adjusted_l1_gas_consumed = tx_execution_info
        .receipt
        .gas
        .l1_gas
        .max(minimal_gas_vector.l1_gas);

    let mut adjusted_l1_data_gas_consumed = tx_execution_info
        .receipt
        .gas
        .l1_data_gas
        .max(minimal_gas_vector.l1_data_gas);

    let mut adjusted_l2_gas_consumed = tx_execution_info
        .receipt
        .gas
        .l2_gas
        .max(minimal_gas_vector.l2_gas);

    match gas_vector_computation_mode {
        GasVectorComputationMode::NoL2Gas => {
            adjusted_l1_gas_consumed = adjusted_l1_gas_consumed
                .checked_add(
                    block_context
                        .versioned_constants()
                        .sierra_gas_to_l1_gas_amount_round_up(adjusted_l2_gas_consumed),
                )
                .ok_or_else(|| {
                    JunoError::tx_non_execution_error(
                        format!(
                            "addition of L2 gas ({}) to L1 gas ({}) conversion overflowed.",
                            adjusted_l2_gas_consumed, adjusted_l1_gas_consumed,
                        ),
                        txn_index,
                    )
                })?;

            adjusted_l1_data_gas_consumed = adjusted_l1_data_gas_consumed;
            adjusted_l2_gas_consumed = GasAmount(0);
        }
        _ => {}
    };
    let tip = if block_context.versioned_constants().enable_tip {
        match txn {
            Transaction::Account(txn) => txn.tip(),
            Transaction::L1Handler(_) => Tip(0),
        }
    } else {
        starknet_api::transaction::fields::Tip(0)
    };
    tx_execution_info.receipt.gas.l1_gas = adjusted_l1_gas_consumed;
    tx_execution_info.receipt.gas.l1_data_gas = adjusted_l1_data_gas_consumed;
    tx_execution_info.receipt.gas.l2_gas = adjusted_l2_gas_consumed;
    tx_execution_info.receipt.fee = fee_utils::get_fee_by_gas_vector(
        block_context.block_info(),
        GasVector {
            l1_data_gas: adjusted_l1_data_gas_consumed,
            l1_gas: adjusted_l1_gas_consumed,
            l2_gas: adjusted_l2_gas_consumed,
        },
        &fee_type,
        tip,
    );
    Ok(())
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

fn append_gas_and_fee(tx_execution_info: &TransactionExecutionInfo, reader_handle: usize) {
    let actual_fee: Felt = tx_execution_info.receipt.fee.0.into();
    let da_gas_l1_gas = tx_execution_info.receipt.da_gas.l1_gas.into();
    let da_gas_l1_data_gas = tx_execution_info.receipt.da_gas.l1_data_gas.into();
    let execution_steps = tx_execution_info
        .receipt
        .resources
        .computation
        .total_vm_resources()
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
        JunoAddExecutionSteps(reader_handle, execution_steps)
    }
}

fn append_trace(
    reader_handle: usize,
    trace: &TransactionTrace,
    writer_buffer: &mut Vec<u8>,
) -> Result<(), serde_json::Error> {
    writer_buffer.clear();
    serde_json::to_writer(&mut *writer_buffer, trace)?;

    let ptr = writer_buffer.as_ptr();
    let len = writer_buffer.len();

    unsafe {
        JunoAppendTrace(reader_handle, ptr as *const c_void, len);
    };
    Ok(())
}

fn append_state_diff(
    reader_handle: usize,
    state_diff: &StateDiff,
    writer_buffer: &mut Vec<u8>,
) -> Result<(), serde_json::Error> {
    writer_buffer.clear();
    serde_json::to_writer(&mut *writer_buffer, state_diff)?;

    let ptr = writer_buffer.as_ptr();
    let len = writer_buffer.len();

    unsafe {
        JunoAppendStateDiff(reader_handle, ptr as *const c_void, len);
    };
    Ok(())
}

fn append_receipt(
    reader_handle: usize,
    trace: &TransactionReceipt,
    writer_buffer: &mut Vec<u8>,
) -> Result<(), serde_json::Error> {
    writer_buffer.clear();
    serde_json::to_writer(&mut *writer_buffer, trace)?;

    let ptr = writer_buffer.as_ptr();
    let len = writer_buffer.len();

    unsafe {
        JunoAppendReceipt(reader_handle, ptr as *const c_void, len);
    };
    Ok(())
}

fn error_stack_frames_to_json(err_stack: ErrorStack) -> serde_json::Value {
    let frames = err_stack.0;

    // We are assuming they will be only one of these
    let string_frame = frames
        .iter()
        .rev()
        .find_map(|frame| match frame {
            Frame::StringFrame(string) => Some(string.clone()),
            _ => None,
        })
        .unwrap_or_else(|| "Unknown error, no string frame available.".to_string());

    // Start building the error from the ground up, starting from the string frame
    // into the parent call frame and so on ...
    frames
        .into_iter()
        .filter_map(|frame| match frame {
            Frame::CallFrame(call_frame) => Some(call_frame),
            _ => None,
        })
        .rev()
        .fold(json!(string_frame), |prev_err, frame| {
            json!({
                "contract_address": frame.storage_address,
                "class_hash": frame.class_hash,
                "selector": frame.selector,
                "error": prev_err,
            })
        })
}

// NonzeroGasPrice must be greater than zero to successfully execute transaction.

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn setVersionedConstants(json_bytes: *const c_char) -> *const c_char {
    let json_str = unsafe {
        match CStr::from_ptr(json_bytes).to_str() {
            Ok(s) => s,
            Err(_) => {
                return CString::new("Failed to convert JSON bytes to string")
                    .unwrap()
                    .into_raw()
            }
        }
    };

    let versioned_constants_files_paths: Result<BTreeMap<String, String>, _> =
        serde_json::from_str(json_str);
    if let Ok(paths) = versioned_constants_files_paths {
        match VersionedConstantsMap::from_file(paths) {
            Ok(custom_constants) => unsafe {
                CUSTOM_VERSIONED_CONSTANTS = Some(custom_constants);
                CString::new("").unwrap().into_raw()
            },
            Err(e) => CString::new(format!(
                "Failed to load versioned constants from paths: {}",
                e
            ))
            .unwrap()
            .into_raw(),
        }
    } else {
        CString::new("Failed to parse JSON").unwrap().into_raw()
    }
}

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn freeString(s: *mut c_char) {
    if !s.is_null() {
        unsafe {
            // Convert the raw C string pointer back to a CString. This operation
            // takes ownership of the memory again and ensures it gets deallocated
            // when drop function returns.
            drop(CString::from_raw(s));
        }
    }
}
