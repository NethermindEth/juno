use std::{
    collections::BTreeMap,
    ffi::{c_char, c_longlong, c_uchar, c_ulonglong, CStr, CString},
    panic,
};

use crate::{
    entrypoint::{cairo_vm_call, cairo_vm_execute},
    error::juno::JunoError,
    versioned_constants::{VersionedConstantsMap, CUSTOM_VERSIONED_CONSTANTS},
};

extern "C" {
    fn JunoReportError(
        reader_handle: usize,
        txnIndex: c_longlong,
        err: *const c_char,
        execution_failed: usize,
    );
}

fn format_panic(payload: Box<dyn std::any::Any + Send>) -> String {
    payload
        .downcast_ref::<&str>()
        .map(|s| s.to_string())
        .unwrap_or_else(|| {
            payload
                .downcast_ref::<String>()
                .map(|s| s.clone())
                .unwrap_or("Unknown panic payload".into())
        })
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
    panic::catch_unwind(|| {
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
    })
    .map_or_else(
        |err| report_error(reader_handle, JunoError::block_error(format_panic(err))),
        |res| res.unwrap_or_else(|err| report_error(reader_handle, err)),
    )
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
    is_estimate_fee: c_uchar,
) {
    panic::catch_unwind(|| {
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
            is_estimate_fee,
        )
    })
    .map_or_else(
        |err| report_error(reader_handle, JunoError::block_error(format_panic(err))),
        |res| res.unwrap_or_else(|err| report_error(reader_handle, err)),
    )
}

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
                "Failed to load versioned constants from paths: {e}",
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
