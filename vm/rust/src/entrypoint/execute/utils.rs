use std::ffi::{c_char, c_void, CStr};

use blockifier::context::BlockContext;
use blockifier::fee::{fee_utils, gas_usage};
use blockifier::transaction::objects::HasRelatedFeeType;
use blockifier::transaction::transaction_execution::Transaction;
use blockifier::transaction::{
    account_transaction::ExecutionFlags as AccountExecutionFlags, objects::TransactionExecutionInfo,
};

use serde::Deserialize;

use starknet_api::execution_resources::{GasAmount, GasVector};
use starknet_api::transaction::fields::Tip;
use starknet_api::{
    contract_class::ClassInfo,
    transaction::{
        fields::{Fee, GasVectorComputationMode},
        Transaction as StarknetApiTransaction, TransactionHash,
    },
};

use starknet_types_core::felt::Felt;

use crate::entrypoint::execute::ffi::{
    JunoAddExecutionSteps, JunoAppendActualFee, JunoAppendDAGas, JunoAppendGasConsumed,
    JunoAppendReceipt, JunoAppendTrace,
};
use crate::error::juno::JunoError;
use crate::ffi_type::transaction_receipt::TransactionReceipt;
use crate::ffi_type::transaction_trace::TransactionTrace;
use crate::state_reader::state_reader::felt_to_byte_array;

pub fn parse_json<'de, T: Deserialize<'de>>(json_ptr: *const c_char) -> Result<T, JunoError> {
    let json_c_str = unsafe { CStr::from_ptr(json_ptr) };
    let json_str = json_c_str.to_str().map_err(JunoError::block_error)?;
    serde_json::from_str(json_str).map_err(JunoError::block_error)
}

pub fn transaction_from_api(
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

pub fn adjust_fee_calculation_result(
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

pub fn append_gas_and_fee(tx_execution_info: &TransactionExecutionInfo, reader_handle: usize) {
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

pub fn append_trace(
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

pub fn append_receipt(
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
