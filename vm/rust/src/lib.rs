mod juno_state_reader;

use crate::juno_state_reader::{ptr_to_felt, JunoStateReader};
use std::{
    collections::HashMap,
    ffi::{c_char, c_uchar, c_ulonglong, CStr, CString},
    slice,
};

use blockifier::{
    abi::constants::{INITIAL_GAS_COST, MAX_STEPS_PER_TX, N_STEPS_RESOURCE},
    block_context::BlockContext,
    execution::entry_point::{
        CallEntryPoint, CallType, EntryPointExecutionContext, ExecutionResources,
    },
    state::cached_state::CachedState,
    transaction::objects::AccountTransactionContext,
};
use cairo_vm::vm::runners::builtin_runner::{
    BITWISE_BUILTIN_NAME, EC_OP_BUILTIN_NAME, HASH_BUILTIN_NAME, KECCAK_BUILTIN_NAME,
    OUTPUT_BUILTIN_NAME, POSEIDON_BUILTIN_NAME, RANGE_CHECK_BUILTIN_NAME,
    SEGMENT_ARENA_BUILTIN_NAME, SIGNATURE_BUILTIN_NAME,
};
use juno_state_reader::felt_to_byte_array;
use starknet_api::transaction::Calldata;
use starknet_api::{
    block::{BlockNumber, BlockTimestamp},
    deprecated_contract_class::EntryPointType,
    hash::StarkFelt,
};
use starknet_api::{
    core::{ChainId, ContractAddress, EntryPointSelector},
    hash::StarkHash,
};

extern "C" {
    fn JunoReportError(reader_handle: usize, err: *const c_char);
    fn JunoAppendResponse(reader_handle: usize, ptr: *const c_uchar);
}

const N_STEPS_FEE_WEIGHT: f64 = 0.01;

#[no_mangle]
pub extern "C" fn cairoVMCall(
    contract_address: *const c_uchar,
    entry_point_selector: *const c_uchar,
    calldata: *const *const c_uchar,
    len_calldata: usize,
    reader_handle: usize,
    block_number: c_ulonglong,
    block_timestamp: c_ulonglong,
    chain_id: *const c_char,
) {
    let reader = JunoStateReader::new(reader_handle);
    let contract_addr_felt = ptr_to_felt(contract_address);
    let entry_point_selector_felt = ptr_to_felt(entry_point_selector);
    let chain_id_str = unsafe { CStr::from_ptr(chain_id) }.to_str().unwrap();

    let mut calldata_vec: Vec<StarkFelt> = vec![];
    if len_calldata > 0 {
        let calldata_slice = unsafe { slice::from_raw_parts(calldata, len_calldata) };
        for ptr in calldata_slice {
            let data = ptr_to_felt(ptr.cast());
            calldata_vec.push(data);
        }
    }

    let entry_point = CallEntryPoint {
        entry_point_type: EntryPointType::External,
        entry_point_selector: EntryPointSelector(entry_point_selector_felt),
        calldata: Calldata(calldata_vec.into()),
        storage_address: contract_addr_felt.try_into().unwrap(),
        call_type: CallType::Call,
        class_hash: None,
        code_address: None,
        caller_address: ContractAddress::default(),
        initial_gas: INITIAL_GAS_COST.into(),
    };

    let mut state = CachedState::new(reader);
    let mut resources = ExecutionResources::default();
    let mut context = EntryPointExecutionContext::new(
        build_block_context(chain_id_str, block_number, block_timestamp),
        AccountTransactionContext::default(),
        MAX_STEPS_PER_TX as u32,
    );
    let call_info = entry_point.execute(&mut state, &mut resources, &mut context);

    match call_info {
        Err(e) => report_error(reader_handle, e.to_string().as_str()),
        Ok(t) => {
            for data in t.execution.retdata.0 {
                unsafe {
                    JunoAppendResponse(reader_handle, felt_to_byte_array(&data).as_ptr());
                };
            }
        }
    }
}

fn report_error(reader_handle: usize, msg: &str) {
    let err_msg = CString::new(msg).unwrap();
    unsafe {
        JunoReportError(reader_handle, err_msg.as_ptr());
    };
}

fn build_block_context(
    chain_id_str: &str,
    block_number: c_ulonglong,
    block_timestamp: c_ulonglong,
) -> BlockContext {
    BlockContext {
        chain_id: ChainId(chain_id_str.into()),
        block_number: BlockNumber(block_number),
        block_timestamp: BlockTimestamp(block_timestamp),

        sequencer_address: ContractAddress::default(),
        // https://github.com/starknet-io/starknet-addresses/blob/df19b17d2c83f11c30e65e2373e8a0c65446f17c/bridged_tokens/mainnet.json
        // fee_token_address is the same for all networks
        fee_token_address: ContractAddress::try_from(
            StarkHash::try_from(
                "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
            )
            .unwrap(),
        )
        .unwrap(),
        gas_price: 1, // fixed gas price, so that we can return "consumed gas" to Go side
        vm_resource_fee_cost: HashMap::from([
            (N_STEPS_RESOURCE.to_string(), N_STEPS_FEE_WEIGHT),
            (OUTPUT_BUILTIN_NAME.to_string(), 0.0),
            (HASH_BUILTIN_NAME.to_string(), N_STEPS_FEE_WEIGHT * 32.0),
            (
                RANGE_CHECK_BUILTIN_NAME.to_string(),
                N_STEPS_FEE_WEIGHT * 16.0,
            ),
            (
                SIGNATURE_BUILTIN_NAME.to_string(),
                N_STEPS_FEE_WEIGHT * 2048.0,
            ),
            (BITWISE_BUILTIN_NAME.to_string(), N_STEPS_FEE_WEIGHT * 64.0),
            (EC_OP_BUILTIN_NAME.to_string(), N_STEPS_FEE_WEIGHT * 1024.0),
            (POSEIDON_BUILTIN_NAME.to_string(), N_STEPS_FEE_WEIGHT * 32.0),
            (
                SEGMENT_ARENA_BUILTIN_NAME.to_string(),
                N_STEPS_FEE_WEIGHT * 10.0,
            ),
            (KECCAK_BUILTIN_NAME.to_string(), 0.0),
        ]),
        invoke_tx_max_n_steps: 1_000_000,
        validate_max_n_steps: 1_000_000,
    }
}
