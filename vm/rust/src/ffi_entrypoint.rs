use std::ffi::{c_char, c_longlong, c_uchar, c_ulonglong, CString};

use crate::error::juno::JunoError;

extern "C" {
    fn JunoReportError(
        reader_handle: usize,
        txnIndex: c_longlong,
        err: *const c_char,
        execution_failed: usize,
    );
}

fn report_error(reader_handle: usize, err: JunoError) {
    let err_msg = CString::new(err.msg).unwrap();
    let execution_failed = if err.execution_failed { 1 } else { 0 };
    unsafe {
        JunoReportError(
            reader_handle,
            err.txn_index,
            err_msg.as_ptr(),
            execution_failed,
        );
    };
}

#[repr(C)]
#[derive(Clone, Copy)]
pub struct ChainInfo {
    pub chain_id: *const c_char,
    pub eth_fee_token_address: [c_uchar; 32],
    pub strk_fee_token_address: [c_uchar; 32],
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
    chain_info_ptr: *const ChainInfo,
    reader_handle: usize,
    max_steps: c_ulonglong,
    initial_gas: c_ulonglong,
    concurrency_mode: c_uchar,
    err_stack: c_uchar,
    return_state_diff: c_uchar,
) {
    cairo_vm_call(
        call_info_ptr,
        block_info_ptr,
        chain_info_ptr,
        reader_handle,
        max_steps,
        initial_gas,
        concurrency_mode,
        err_stack,
        return_state_diff,
    )
    .unwrap_or_else(|err| report_error(reader_handle, err));
}

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn cairoVMExecute(
    txns_json: *const c_char,
    classes_json: *const c_char,
    paid_fees_on_l1_json: *const c_char,
    block_info_ptr: *const BlockInfo,
    chain_info_ptr: *const ChainInfo,
    reader_handle: usize,
    skip_charge_fee: c_uchar,
    skip_validate: c_uchar,
    err_on_revert: c_uchar,
    concurrency_mode: c_uchar,
    err_stack: c_uchar,
    allow_binary_search: c_uchar,
) {
    cairo_vm_execute(
        txns_json,
        classes_json,
        paid_fees_on_l1_json,
        block_info_ptr,
        chain_info_ptr,
        reader_handle,
        skip_charge_fee,
        skip_validate,
        err_on_revert,
        concurrency_mode,
        err_stack,
        allow_binary_search,
    )
    .unwrap_or_else(|err| report_error(reader_handle, err));
}
