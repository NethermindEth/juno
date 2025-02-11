use crate::block_context::build_block_context;
use crate::errors::report_error;
use crate::ffi::JunoAppendResponse;
use crate::juno_state_reader::{felt_to_byte_array, ptr_to_felt, JunoStateReader};
use crate::types::{BlockInfo, CallInfo, StarkFelt};
use crate::versioned_constants::get_versioned_constants;
use blockifier::execution::entry_point::SierraGasRevertTracker;
use blockifier::{
    context::TransactionContext,
    execution::entry_point::{CallEntryPoint, CallType, EntryPointExecutionContext},
    state::cached_state::CachedState,
    transaction::objects::{DeprecatedTransactionInfo, TransactionInfo},
};
use starknet_api::contract_class::{EntryPointType, SierraVersion};
use starknet_api::core::{ClassHash, EntryPointSelector, PatriciaKey};
use starknet_api::execution_resources::GasAmount;
use starknet_api::transaction::fields::Calldata;
use std::str::FromStr;
use std::{
    ffi::{c_char, c_uchar, c_ulonglong, CStr},
    slice,
    sync::Arc,
};

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
) {
    let block_info = unsafe { *block_info_ptr };
    let call_info = unsafe { *call_info_ptr };

    let reader = JunoStateReader::new(reader_handle, block_info.block_number);
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
    let sierra_version =
        SierraVersion::from_str(sierra_version_str).unwrap_or(SierraVersion::DEPRECATED);
    let initial_gas: u64 = if sierra_version < SierraVersion::new(1, 7, 0) {
        version_constants.infinite_gas_for_vm_mode()
    } else {
        version_constants.os_constants.validate_max_sierra_gas.0
    };
    let contract_address =
        starknet_api::core::ContractAddress(PatriciaKey::try_from(contract_addr_felt).unwrap());

    let entry_point = CallEntryPoint {
        entry_point_type: EntryPointType::External,
        entry_point_selector: EntryPointSelector(entry_point_selector_felt),
        calldata: Calldata(calldata_vec.into()),
        storage_address: contract_address,
        call_type: CallType::Call,
        class_hash,
        initial_gas,
        ..Default::default()
    };

    let concurrency_mode = concurrency_mode == 1;
    let mut state = CachedState::new(reader);
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
    match entry_point.execute(&mut state, &mut context, &mut remaining_gas) {
        Err(e) => report_error(reader_handle, e.to_string().as_str(), -1),
        Ok(call_info) => {
            if call_info.execution.failed {
                report_error(reader_handle, "execution failed", -1);
                return;
            }
            for data in call_info.execution.retdata.0 {
                unsafe {
                    JunoAppendResponse(reader_handle, felt_to_byte_array(&data).as_ptr());
                };
            }
        }
    }
}
