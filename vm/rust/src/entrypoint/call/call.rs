use std::{
    ffi::{c_uchar, c_ulonglong},
    slice,
    sync::Arc,
};

use blockifier::{
    context::TransactionContext,
    execution::entry_point::{
        CallEntryPoint, CallType, EntryPointExecutionContext, SierraGasRevertTracker,
    },
    state::cached_state::CachedState,
    transaction::objects::{DeprecatedTransactionInfo, TransactionInfo},
};
use once_cell::sync::Lazy;
use starknet_api::{
    contract_class::EntryPointType,
    core::{ClassHash, ContractAddress},
    execution_resources::GasAmount,
    transaction::fields::Calldata,
};
use starknet_types_core::felt::Felt;

use crate::{
    entrypoint::{
        call::{ffi::JunoAppendResponse, utils::append_state_diff},
        utils::build_block_context,
    },
    error::{call::CallError, juno::JunoError, stack::error_stack_frames_to_json},
    ffi_entrypoint::{BlockInfo, CallInfo, ChainInfo},
    ffi_type::state_diff::StateDiff,
    state_reader::{
        state_reader::{felt_to_byte_array, ptr_to_felt, BlockHeight},
        JunoStateReader,
    },
};

// Allow users to call CONSTRUCTOR entry point type which has fixed entry_point_felt
// "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194"
static CONSTRUCTOR_ENTRY_POINT_FELT: Lazy<Felt> = Lazy::new(|| {
    Felt::from_hex("0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194")
        .expect("Invalid hex string")
});

#[allow(clippy::too_many_arguments)]
pub fn cairo_vm_call(
    call_info_ptr: *const CallInfo,
    block_info_ptr: *const BlockInfo,
    chain_info_ptr: *const ChainInfo,
    reader_handle: usize,
    max_steps: c_ulonglong,
    initial_gas: c_ulonglong,
    concurrency_mode: c_uchar,
    err_stack: c_uchar,
    return_state_diff: c_uchar,
) -> Result<(), JunoError> {
    let block_info = unsafe { *block_info_ptr };
    let call_info = unsafe { *call_info_ptr };
    let chain_info = unsafe { *chain_info_ptr };
    let mut writer_buffer = Vec::with_capacity(10_000);

    let reader = JunoStateReader::new(reader_handle, BlockHeight::from_block_info(&block_info));
    let contract_addr_felt = Felt::from_bytes_be(&call_info.contract_address);
    let class_hash = if call_info.class_hash == [0; 32] {
        None
    } else {
        Some(ClassHash(Felt::from_bytes_be(&call_info.class_hash)))
    };
    let entry_point_selector_felt = Felt::from_bytes_be(&call_info.entry_point_selector);

    let mut calldata_vec: Vec<Felt> = Vec::with_capacity(call_info.len_calldata);
    if call_info.len_calldata > 0 {
        let calldata_slice =
            unsafe { slice::from_raw_parts(call_info.calldata, call_info.len_calldata) };
        for ptr in calldata_slice {
            let data = ptr_to_felt(ptr.cast());
            calldata_vec.push(data);
        }
    }

    let contract_address =
        ContractAddress::try_from(contract_addr_felt).map_err(JunoError::block_error)?;
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
            block_context: Arc::new(
                build_block_context(
                    &mut state,
                    &block_info,
                    &chain_info,
                    Some(max_steps),
                    concurrency_mode,
                )
                .map_err(JunoError::block_error)?,
            ),
            tx_info: TransactionInfo::Deprecated(DeprecatedTransactionInfo::default()),
        }),
        false,
        SierraGasRevertTracker::new(GasAmount::from(initial_gas)),
    );
    let mut remaining_gas = entry_point.initial_gas;
    let call_info = entry_point
        .execute(&mut state, &mut context, &mut remaining_gas)
        .map_err(|e| {
            let e = CallError::from_entry_point_execution_error(
                e,
                contract_address,
                class_hash.unwrap_or(ClassHash::default()),
                entry_point_selector,
            );
            match e {
                CallError::ContractError(revert_error, error_stack) => {
                    if structured_err_stack {
                        JunoError::json_error(error_stack_frames_to_json(error_stack), None)
                    } else {
                        JunoError::block_error(revert_error)
                    }
                }
                CallError::Internal(e) | CallError::Custom(e) => JunoError::block_error(e),
            }
        })?;

    for data in call_info.execution.retdata.0 {
        unsafe {
            JunoAppendResponse(reader_handle, felt_to_byte_array(&data).as_ptr());
        }
    }
    if call_info.execution.failed {
        Err(JunoError {
            msg: "execution failed".to_string(),
            txn_index: -1,
            execution_failed: true,
        })
    } else {
        // We only need to return the state_diff when creating the genesis state_diff.
        // Calling this for all other RPC requests is a waste of resources.
        if return_state_diff == 1 {
            let state_diff = state
                .to_state_diff()
                .map_err(|_| JunoError::block_error("failed to convert state diff"))?;
            let json_state_diff = StateDiff::from(state_diff.state_maps);
            append_state_diff(reader_handle, &json_state_diff, &mut writer_buffer)
                .map_err(|err| JunoError::block_error(err.to_string()))?;
        }
        Ok(())
    }
}
