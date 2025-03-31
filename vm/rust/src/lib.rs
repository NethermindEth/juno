pub mod error;
pub mod error_stack;
pub mod execution;
pub mod jsonrpc;
mod juno_state_reader;

use crate::juno_state_reader::{ptr_to_felt, BlockHeight, JunoStateReader};
use error::{CallError, ExecutionError};
use error_stack::{ErrorStack, Frame};
use execution::process_transaction;
use serde::Deserialize;
use serde_json::json;
use std::{
    collections::BTreeMap,
    ffi::{c_char, c_longlong, c_uchar, c_ulonglong, c_void, CStr, CString},
    fs::File,
    io::Read,
    path::Path,
    slice,
    sync::Arc,
};

use anyhow::Result;
use blockifier::bouncer::BouncerConfig;
use blockifier::fee::{fee_utils, gas_usage};
use blockifier::{
    abi::constants::STORED_BLOCK_HASH_BUFFER,
    transaction::account_transaction::ExecutionFlags as AccountExecutionFlags,
};
use blockifier::{
    blockifier::block::pre_process_block, execution::entry_point::SierraGasRevertTracker,
};
use blockifier::{
    context::{BlockContext, ChainInfo, FeeTokenAddresses, TransactionContext},
    execution::entry_point::{CallEntryPoint, CallType, EntryPointExecutionContext},
    state::{cached_state::CachedState, state_api::State},
    transaction::{
        objects::{DeprecatedTransactionInfo, HasRelatedFeeType, TransactionInfo},
        transaction_execution::Transaction,
    },
    versioned_constants::VersionedConstants,
};
use juno_state_reader::{class_info_from_json_str, felt_to_byte_array};
use starknet_api::{
    block::{BlockHash, GasPrice, StarknetVersion},
    contract_class::{ClassInfo, EntryPointType, SierraVersion},
    core::PatriciaKey,
    executable_transaction::AccountTransaction,
    execution_resources::GasVector,
    transaction::{
        fields::{Calldata, Fee, GasVectorComputationMode},
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
use starknet_api::{
    core::{ChainId, ClassHash, ContractAddress},
    hash::StarkHash,
};
use starknet_types_core::felt::Felt;
use std::str::FromStr;
type StarkFelt = Felt;
use anyhow::Context;
use once_cell::sync::Lazy;

// Allow users to call CONSTRUCTOR entry point type which has fixed entry_point_felt "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194"
pub static CONSTRUCTOR_ENTRY_POINT_FELT: Lazy<StarkFelt> = Lazy::new(|| {
    StarkFelt::from_hex("0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194")
        .expect("Invalid hex string")
});

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

#[repr(C)]
#[derive(Clone, Copy)]
pub struct CallInfo {
    pub contract_address: [c_uchar; 32],
    pub class_hash: [c_uchar; 32],
    pub entry_point_selector: [c_uchar; 32],
    pub calldata: *const *const c_uchar,
    pub len_calldata: usize,
}

#[repr(C)]
#[derive(Clone, Copy)]
pub struct BlockInfo {
    pub block_number: c_ulonglong,
    pub block_timestamp: c_ulonglong,
    pub is_pending: c_uchar,
    pub sequencer_address: [c_uchar; 32],
    pub l1_gas_price_wei: [c_uchar; 32],
    pub l1_gas_price_fri: [c_uchar; 32],
    pub version: *const c_char,
    pub block_hash_to_be_revealed: [c_uchar; 32],
    pub l1_data_gas_price_wei: [c_uchar; 32],
    pub l1_data_gas_price_fri: [c_uchar; 32],
    pub use_blob_data: c_uchar,
    pub l2_gas_price_wei: [c_uchar; 32],
    pub l2_gas_price_fri: [c_uchar; 32],
}

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn cairoVMCall(
    call_info_ptr: *const CallInfo,
    block_info_ptr: *const BlockInfo,
    reader_handle: usize,
    chain_id: *const c_char,
    max_steps: c_ulonglong,
    concurrency_mode: c_uchar,
    sierra_version: *const c_char,
    err_stack: c_uchar,
    return_state_diff: c_uchar,
) {
    let block_info = unsafe { *block_info_ptr };
    let call_info = unsafe { *call_info_ptr };
    let mut writer_buffer = Vec::with_capacity(10_000);

    let reader = JunoStateReader::new(reader_handle, BlockHeight::from_block_info(&block_info));
    let contract_addr_felt = StarkFelt::from_bytes_be(&call_info.contract_address);
    let class_hash = if call_info.class_hash == [0; 32] {
        None
    } else {
        Some(ClassHash(StarkFelt::from_bytes_be(&call_info.class_hash)))
    };
    let entry_point_selector_felt = StarkFelt::from_bytes_be(&call_info.entry_point_selector);
    let chain_id_str = unsafe { CStr::from_ptr(chain_id) }.to_str().unwrap();

    let mut calldata_vec: Vec<StarkFelt> = Vec::with_capacity(call_info.len_calldata);
    if call_info.len_calldata > 0 {
        let calldata_slice =
            unsafe { slice::from_raw_parts(call_info.calldata, call_info.len_calldata) };
        for ptr in calldata_slice {
            let data = ptr_to_felt(ptr.cast());
            calldata_vec.push(data);
        }
    }

    let version_constants = get_versioned_constants(block_info.version);
    let sierra_version_str = unsafe { CStr::from_ptr(sierra_version) }.to_str().unwrap();
    let sierra_version = SierraVersion::from_str(sierra_version_str).unwrap();
    let initial_gas: u64 = if sierra_version < version_constants.min_sierra_version_for_sierra_gas {
        version_constants.infinite_gas_for_vm_mode()
    } else {
        version_constants.os_constants.validate_max_sierra_gas.0
    };
    let contract_address =
        starknet_api::core::ContractAddress(PatriciaKey::try_from(contract_addr_felt).unwrap());
    let entry_point_selector = starknet_api::core::EntryPointSelector(entry_point_selector_felt);

    let mut entry_point = CallEntryPoint {
        entry_point_type: EntryPointType::External,
        entry_point_selector,
        calldata: Calldata(calldata_vec.into()),
        storage_address: contract_address,
        call_type: CallType::Call,
        class_hash,
        initial_gas,
        ..Default::default()
    };

    if CONSTRUCTOR_ENTRY_POINT_FELT.eq(&entry_point_selector_felt) {
        entry_point.entry_point_type = EntryPointType::Constructor
    }

    let mut state = CachedState::new(reader);
    let concurrency_mode = concurrency_mode == 1;
    let structured_err_stack = err_stack == 1;
    let mut context = EntryPointExecutionContext::new_invoke(
        Arc::new(TransactionContext {
            block_context: build_block_context(
                &mut state,
                &block_info,
                chain_id_str,
                Some(max_steps),
                concurrency_mode,
            )
            .unwrap(),
            tx_info: TransactionInfo::Deprecated(DeprecatedTransactionInfo::default()),
        }),
        false,
        SierraGasRevertTracker::new(GasAmount::from(initial_gas)),
    );
    let mut remaining_gas = entry_point.initial_gas;
    let call_info = entry_point
        .execute(&mut state, &mut context, &mut remaining_gas)
        .map_err(|e| {
            CallError::from_entry_point_execution_error(
                e,
                contract_address,
                class_hash.unwrap_or(ClassHash::default()),
                entry_point_selector,
            )
        });

    match call_info {
        Err(CallError::ContractError(revert_error, error_stack)) => {
            let err_string = if structured_err_stack {
                error_stack_frames_to_json(error_stack).to_string()
            } else {
                json!(revert_error).to_string()
            };
            report_error(reader_handle, err_string.as_str(), -1, 0);
        }
        Err(CallError::Internal(e)) | Err(CallError::Custom(e)) => {
            report_error(reader_handle, &e, -1, 0);
        }
        Ok(call_info) => {
            if call_info.execution.failed {
                report_error(reader_handle, "execution failed", -1, 1);
            }
            for data in call_info.execution.retdata.0 {
                unsafe {
                    JunoAppendResponse(reader_handle, felt_to_byte_array(&data).as_ptr());
                }
            }
            // We only need to return the state_diff when creating the genesis state_diff.
            // Calling this for all other RPC requests is a waste of resources.
            if return_state_diff == 1 {
                match state.to_state_diff() {
                    Ok(state_diff) => {
                        let json_state_diff = jsonrpc::StateDiff::from(state_diff.state_maps);
                        append_state_diff(reader_handle, &json_state_diff, &mut writer_buffer);
                    }
                    Err(_) => {
                        report_error(reader_handle, "failed to convert state diff", -1, 0);
                    }
                }
            }
        }
    };
}

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
    err_stack: c_uchar,
) {
    let block_info = unsafe { *block_info_ptr };
    let reader = JunoStateReader::new(reader_handle, BlockHeight::from_block_info(&block_info));
    let chain_id_str = unsafe { CStr::from_ptr(chain_id) }.to_str().unwrap();
    let txn_json_str = unsafe { CStr::from_ptr(txns_json) }.to_str().unwrap();
    let txns_and_query_bits: Result<Vec<TxnAndQueryBit>, serde_json::Error> =
        serde_json::from_str(txn_json_str);
    if let Err(e) = txns_and_query_bits {
        report_error(reader_handle, e.to_string().as_str(), -1, 0);
        return;
    }

    let mut classes: Result<Vec<Box<serde_json::value::RawValue>>, serde_json::Error> = Ok(vec![]);
    if !classes_json.is_null() {
        let classes_json_str = unsafe { CStr::from_ptr(classes_json) }.to_str().unwrap();
        classes = serde_json::from_str(classes_json_str);
    }
    if let Err(e) = classes {
        report_error(reader_handle, e.to_string().as_str(), -1, 0);
        return;
    }

    let paid_fees_on_l1_json_str = unsafe { CStr::from_ptr(paid_fees_on_l1_json) }
        .to_str()
        .unwrap();
    let mut paid_fees_on_l1: Vec<Box<Fee>> = match serde_json::from_str(paid_fees_on_l1_json_str) {
        Ok(f) => f,
        Err(e) => {
            report_error(reader_handle, e.to_string().as_str(), -1, 0);
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
    let err_stack = err_stack == 1;
    let err_on_revert = err_on_revert == 1;

    let mut writer_buffer = Vec::with_capacity(10_000);

    for (txn_index, txn_and_query_bit) in txns_and_query_bits.iter().enumerate() {
        let class_info = match txn_and_query_bit.txn.clone() {
            StarknetApiTransaction::Declare(_) => {
                if classes.is_empty() {
                    report_error(reader_handle, "missing declared class", txn_index as i64, 0);
                    return;
                }
                let class_json_str = classes.remove(0);

                let maybe_cc = class_info_from_json_str(class_json_str.get());
                if let Err(e) = maybe_cc {
                    report_error(reader_handle, e.to_string().as_str(), txn_index as i64, 0);
                    return;
                }
                Some(maybe_cc.unwrap())
            }
            _ => None,
        };

        let paid_fee_on_l1: Option<Fee> = match txn_and_query_bit.txn.clone() {
            StarknetApiTransaction::L1Handler(_) => {
                if paid_fees_on_l1.is_empty() {
                    report_error(
                        reader_handle,
                        "missing fee paid on l1b",
                        txn_index as i64,
                        0,
                    );
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
            report_error(reader_handle, e.to_string().as_str(), txn_index as i64, 0);
            return;
        }

        let is_l1_handler_txn = false;
        let mut txn_state = CachedState::create_transactional(&mut state);
        let mut txn = txn.unwrap();
        let gas_vector_computation_mode = determine_gas_vector_mode(&txn);

        let (minimal_gas_vector, fee_type) = match &txn {
            Transaction::Account(t) => (
                Some(gas_usage::estimate_minimal_gas_vector(
                    &block_context,
                    &t,
                    &gas_vector_computation_mode,
                )),
                t.fee_type(),
            ),
            Transaction::L1Handler(t) => (None, t.fee_type()),
        };

        match process_transaction(&mut txn, &mut txn_state, &block_context, err_on_revert) {
            Err(e) => match e {
                ExecutionError::ExecutionError { error, error_stack } => {
                    let err_string = if err_stack {
                        error_stack_frames_to_json(error_stack).to_string()
                    } else {
                        json!(error).to_string()
                    };
                    report_error(reader_handle, err_string.as_str(), txn_index as i64, 0);
                }
                ExecutionError::Internal(e) | ExecutionError::Custom(e) => {
                    report_error(
                        reader_handle,
                        json!(e).to_string().as_str(),
                        txn_index as i64,
                        0,
                    );
                }
            },
            Ok(mut tx_execution_info) => {
                // we are estimating fee, override actual fee calculation
                if tx_execution_info.receipt.fee.0 == 0 && !is_l1_handler_txn {
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

                let transaction_receipt = jsonrpc::TransactionReceipt {
                    gas: tx_execution_info.receipt.gas,
                    da_gas: tx_execution_info.receipt.da_gas,
                    fee: tx_execution_info.receipt.fee,
                };
                append_receipt(reader_handle, &transaction_receipt, &mut writer_buffer);
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
                        0,
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
                append_trace(reader_handle, trace.as_ref().unwrap(), &mut writer_buffer);
            }
        }
        txn_state.commit();
    }
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

fn felt_to_u128(felt: StarkFelt) -> u128 {
    // todo find Into<u128> trait or similar
    let bytes = felt.to_bytes_be();
    let mut arr = [0u8; 16];
    arr.copy_from_slice(&bytes[16..32]);

    // felts are encoded in big-endian order
    u128::from_be_bytes(arr)
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

fn append_trace(
    reader_handle: usize,
    trace: &jsonrpc::TransactionTrace,
    writer_buffer: &mut Vec<u8>,
) {
    writer_buffer.clear();
    serde_json::to_writer(&mut *writer_buffer, trace).unwrap();

    let ptr = writer_buffer.as_ptr();
    let len = writer_buffer.len();

    unsafe {
        JunoAppendTrace(reader_handle, ptr as *const c_void, len);
    };
}
fn append_state_diff(
    reader_handle: usize,
    state_diff: &jsonrpc::StateDiff,
    writer_buffer: &mut Vec<u8>,
) {
    writer_buffer.clear();
    serde_json::to_writer(&mut *writer_buffer, state_diff).unwrap();

    let ptr = writer_buffer.as_ptr();
    let len = writer_buffer.len();

    unsafe {
        JunoAppendStateDiff(reader_handle, ptr as *const c_void, len);
    };
}
fn append_receipt(
    reader_handle: usize,
    trace: &jsonrpc::TransactionReceipt,
    writer_buffer: &mut Vec<u8>,
) {
    writer_buffer.clear();
    serde_json::to_writer(&mut *writer_buffer, trace).unwrap();

    let ptr = writer_buffer.as_ptr();
    let len = writer_buffer.len();

    unsafe {
        JunoAppendReceipt(reader_handle, ptr as *const c_void, len);
    };
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

fn report_error(reader_handle: usize, msg: &str, txn_index: i64, execution_failed: usize) {
    let err_msg = CString::new(msg).unwrap();
    unsafe {
        JunoReportError(reader_handle, txn_index, err_msg.as_ptr(), execution_failed);
    };
}

// NonzeroGasPrice must be greater than zero to successfully execute transaction.
fn gas_price_from_bytes_bonded(bytes: &[c_uchar; 32]) -> Result<NonzeroGasPrice, anyhow::Error> {
    let u128_val = felt_to_u128(StarkFelt::from_bytes_be(bytes));
    Ok(NonzeroGasPrice::new(GasPrice(if u128_val == 0 {
        1
    } else {
        u128_val
    }))?)
}

fn build_block_context(
    state: &mut dyn State,
    block_info: &BlockInfo,
    chain_id_str: &str,
    max_steps: Option<c_ulonglong>,
    _concurrency_mode: bool,
) -> Result<BlockContext> {
    let sequencer_addr = StarkFelt::from_bytes_be(&block_info.sequencer_address);
    let l1_gas_price_eth = gas_price_from_bytes_bonded(&block_info.l1_gas_price_wei)?;
    let l1_gas_price_strk = gas_price_from_bytes_bonded(&block_info.l1_gas_price_fri)?;
    let l1_data_gas_price_eth = gas_price_from_bytes_bonded(&block_info.l1_data_gas_price_wei)?;
    let l1_data_gas_price_strk = gas_price_from_bytes_bonded(&block_info.l1_data_gas_price_fri)?;
    let l2_gas_price_eth = gas_price_from_bytes_bonded(&block_info.l2_gas_price_wei)?;
    let l2_gas_price_strk = gas_price_from_bytes_bonded(&block_info.l2_gas_price_fri)?;

    let mut old_block_number_and_hash: Option<BlockHashAndNumber> = None;
    // STORED_BLOCK_HASH_BUFFER const is 10 for now
    if block_info.block_number >= STORED_BLOCK_HASH_BUFFER {
        old_block_number_and_hash = Some(BlockHashAndNumber {
            number: starknet_api::block::BlockNumber(
                block_info.block_number - STORED_BLOCK_HASH_BUFFER,
            ),
            hash: BlockHash(StarkFelt::from_bytes_be(
                &block_info.block_hash_to_be_revealed,
            )),
        })
    }
    let mut constants = get_versioned_constants(block_info.version);
    if let Some(max_steps) = max_steps {
        constants.invoke_tx_max_n_steps = max_steps as u32;
    }

    let block_info = BlockifierBlockInfo {
        block_number: starknet_api::block::BlockNumber(block_info.block_number),
        block_timestamp: starknet_api::block::BlockTimestamp(block_info.block_timestamp),
        sequencer_address: ContractAddress(PatriciaKey::try_from(sequencer_addr).unwrap()),
        gas_prices: GasPrices {
            eth_gas_prices: GasPriceVector {
                l1_gas_price: l1_gas_price_eth,
                l1_data_gas_price: l1_data_gas_price_eth,
                l2_gas_price: l2_gas_price_eth,
            },
            strk_gas_prices: GasPriceVector {
                l1_gas_price: l1_gas_price_strk,
                l1_data_gas_price: l1_data_gas_price_strk,
                l2_gas_price: l2_gas_price_strk,
            },
        },
        use_kzg_da: block_info.use_blob_data == 1,
    };
    let chain_info = ChainInfo {
        chain_id: ChainId::from(chain_id_str.to_string()),
        fee_token_addresses: FeeTokenAddresses {
            // Both addresses are the same for all networks
            eth_fee_token_address: ContractAddress::try_from(
                StarkHash::from_hex(
                    "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
                )
                .unwrap(),
            )
            .unwrap(),
            strk_fee_token_address: ContractAddress::try_from(
                StarkHash::from_hex(
                    "0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d",
                )
                .unwrap(),
            )
            .unwrap(),
        },
    };

    pre_process_block(
        state,
        old_block_number_and_hash,
        block_info.block_number,
        constants.os_constants.as_ref(),
    )
    .unwrap();

    Ok(BlockContext::new(
        block_info,
        chain_info,
        constants,
        BouncerConfig::max(),
    ))
}

#[allow(static_mut_refs)]
fn get_versioned_constants(version: *const c_char) -> VersionedConstants {
    let starknet_version = unsafe { CStr::from_ptr(version) }
        .to_str()
        .ok()
        .and_then(|version_str| StarknetVersion::try_from(version_str).ok());

    if let (Some(custom_constants), Some(version)) =
        (unsafe { &CUSTOM_VERSIONED_CONSTANTS }, starknet_version)
    {
        if let Some(constants) = custom_constants.0.get(&version) {
            return constants.clone();
        }
    }

    starknet_version
        .and_then(|version| VersionedConstants::get(&version).ok())
        .unwrap_or(VersionedConstants::latest_constants())
        .to_owned()
}

#[derive(Debug)]
pub struct VersionedConstantsMap(pub BTreeMap<StarknetVersion, VersionedConstants>);

impl VersionedConstantsMap {
    pub fn from_file(version_with_path: BTreeMap<String, String>) -> Result<Self> {
        let mut result = BTreeMap::new();

        for (version, path) in version_with_path {
            let mut file = File::open(Path::new(&path))
                .with_context(|| format!("Failed to open file: {}", path))?;

            let mut contents = String::new();
            file.read_to_string(&mut contents)
                .with_context(|| format!("Failed to read contents of file: {}", path))?;

            let constants: VersionedConstants = serde_json::from_str(&contents)
                .with_context(|| format!("Failed to parse JSON in file: {}", path))?;

            let parsed_version = StarknetVersion::try_from(version.as_str())
                .with_context(|| format!("Failed to parse version string: {}", version))?;

            result.insert(parsed_version, constants);
        }

        Ok(VersionedConstantsMap(result))
    }
}

static mut CUSTOM_VERSIONED_CONSTANTS: Option<VersionedConstantsMap> = None;

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
                return CString::new("").unwrap().into_raw();
            },
            Err(e) => {
                return CString::new(format!(
                    "Failed to load versioned constants from paths: {}",
                    e
                ))
                .unwrap()
                .into_raw();
            }
        }
    } else {
        return CString::new("Failed to parse JSON").unwrap().into_raw();
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
