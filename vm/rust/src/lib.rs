pub mod class;
pub mod jsonrpc;
mod juno_state_reader;

use crate::juno_state_reader::{ptr_to_felt, JunoStateReader};
use std::{
    collections::HashMap,
    ffi::{c_char, c_uchar, c_ulonglong, c_void, CStr, CString},
    slice,
};

use blockifier::{
    abi::constants::{INITIAL_GAS_COST, N_STEPS_RESOURCE},
    block_context::BlockContext,
    execution::{
        contract_class::{ContractClass, ContractClassV1},
        entry_point::{CallEntryPoint, CallType, EntryPointExecutionContext, ExecutionResources},
    },
    fee::fee_utils::calculate_tx_fee,
    state::cached_state::CachedState,
    transaction::{
        objects::AccountTransactionContext, transaction_execution::Transaction,
        transactions::ExecutableTransaction,
    },
};
use cairo_lang_starknet::casm_contract_class::CasmContractClass;
use cairo_lang_starknet::contract_class::ContractClass as SierraContractClass;
use cairo_vm::vm::runners::builtin_runner::{
    BITWISE_BUILTIN_NAME, EC_OP_BUILTIN_NAME, HASH_BUILTIN_NAME, KECCAK_BUILTIN_NAME,
    OUTPUT_BUILTIN_NAME, POSEIDON_BUILTIN_NAME, RANGE_CHECK_BUILTIN_NAME,
    SEGMENT_ARENA_BUILTIN_NAME, SIGNATURE_BUILTIN_NAME,
};
use juno_state_reader::{contract_class_from_json_str, felt_to_byte_array};
use starknet_api::transaction::{Calldata, Transaction as StarknetApiTransaction};
use starknet_api::{
    block::{BlockNumber, BlockTimestamp},
    deprecated_contract_class::EntryPointType,
    hash::StarkFelt,
    transaction::Fee,
};
use starknet_api::{
    core::{ChainId, ContractAddress, EntryPointSelector},
    hash::StarkHash,
    transaction::TransactionVersion,
};

extern "C" {
    fn JunoReportError(reader_handle: usize, err: *const c_char);
    fn JunoAppendTrace(reader_handle: usize, json_trace: *const c_void, len: usize);
    fn JunoAppendResponse(reader_handle: usize, ptr: *const c_uchar);
    fn JunoAppendActualFee(reader_handle: usize, ptr: *const c_uchar);
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

    const GAS_PRICE: u128 = 1;
    let mut state = CachedState::new(reader);
    let mut resources = ExecutionResources::default();
    let mut context = EntryPointExecutionContext::new(
        build_block_context(
            chain_id_str,
            block_number,
            block_timestamp,
            StarkFelt::default(),
            GAS_PRICE,
        ),
        AccountTransactionContext::default(),
        4_000_000,
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

#[no_mangle]
pub extern "C" fn cairoVMExecute(
    txns_json: *const c_char,
    classes_json: *const c_char,
    reader_handle: usize,
    block_number: c_ulonglong,
    block_timestamp: c_ulonglong,
    chain_id: *const c_char,
    sequencer_address: *const c_uchar,
    paid_fees_on_l1_json: *const c_char,
    skip_charge_fee: c_uchar,
    gas_price: *const c_uchar,
) {
    let reader = JunoStateReader::new(reader_handle);
    let chain_id_str = unsafe { CStr::from_ptr(chain_id) }.to_str().unwrap();
    let txn_json_str = unsafe { CStr::from_ptr(txns_json) }.to_str().unwrap();
    let sn_api_txns: Result<Vec<StarknetApiTransaction>, serde_json::Error> =
        serde_json::from_str(txn_json_str);
    if sn_api_txns.is_err() {
        report_error(reader_handle, sn_api_txns.unwrap_err().to_string().as_str());
        return;
    }

    let mut classes: Result<Vec<Box<serde_json::value::RawValue>>, serde_json::Error> = Ok(vec![]);
    if !classes_json.is_null() {
        let classes_json_str = unsafe { CStr::from_ptr(classes_json) }.to_str().unwrap();
        classes = serde_json::from_str(classes_json_str);
    }
    if classes.is_err() {
        report_error(reader_handle, classes.unwrap_err().to_string().as_str());
        return;
    }

    let paid_fees_on_l1_json_str = unsafe { CStr::from_ptr(paid_fees_on_l1_json) }
        .to_str()
        .unwrap();
    let mut paid_fees_on_l1: Vec<Box<Fee>> = match serde_json::from_str(paid_fees_on_l1_json_str) {
        Ok(f) => f,
        Err(e) => {
            report_error(reader_handle, e.to_string().as_str());
            return;
        }
    };

    let sn_api_txns = sn_api_txns.unwrap();
    let mut classes = classes.unwrap();

    let sequencer_address_felt = ptr_to_felt(sequencer_address);
    let gas_price_felt = ptr_to_felt(gas_price);
    let block_context: BlockContext = build_block_context(
        chain_id_str,
        block_number,
        block_timestamp,
        sequencer_address_felt,
        felt_to_u128(gas_price_felt),
    );
    let mut state = CachedState::new(reader);
    let charge_fee = skip_charge_fee == 0;

    let mut trace_buffer = Vec::with_capacity(10_000);

    for sn_api_txn in sn_api_txns {
        let contract_class = match sn_api_txn.clone() {
            StarknetApiTransaction::Declare(declare) => {
                if classes.is_empty() {
                    report_error(reader_handle, "missing declared class");
                    return;
                }
                let class_json_str = classes.remove(0);

                let mut maybe_cc = contract_class_from_json_str(class_json_str.get());
                if declare.version() == TransactionVersion(2u32.into()) && maybe_cc.is_err() {
                    // class json could be sierra
                    maybe_cc = contract_class_from_sierra_json(class_json_str.get())
                };

                if maybe_cc.is_err() {
                    report_error(reader_handle, maybe_cc.unwrap_err().to_string().as_str());
                    return;
                }
                Some(maybe_cc.unwrap())
            }
            _ => None,
        };

        let paid_fee_on_l1: Option<Fee> = match sn_api_txn.clone() {
            StarknetApiTransaction::L1Handler(_) => {
                if paid_fees_on_l1.is_empty() {
                    report_error(
                        reader_handle,
                        "missing fee paid on l1b",
                    );
                    return;
                }
                Some(*paid_fees_on_l1.remove(0))
            }
            _ => None,
        };

        let txn = transaction_from_api(sn_api_txn.clone(), contract_class, paid_fee_on_l1);
        if txn.is_err() {
            report_error(reader_handle, txn.unwrap_err().to_string().as_str());
            return;
        }

        let res = match txn.unwrap() {
            Transaction::AccountTransaction(t) => t.execute(&mut state, &block_context, charge_fee),
            Transaction::L1HandlerTransaction(t) => {
                t.execute(&mut state, &block_context, charge_fee)
            }
        };

        match res {
            Err(e) => {
                report_error(
                    reader_handle,
                    format!(
                        "failed txn {:?} reason:{:?}",
                        sn_api_txn.transaction_hash(),
                        e
                    )
                    .as_str(),
                );
                return;
            }
            Ok(mut t) => {
                // we are estimating fee, override actual fee calculation
                if !charge_fee {
                    t.actual_fee = calculate_tx_fee(&t.actual_resources, &block_context).unwrap();
                }

                unsafe {
                    JunoAppendActualFee(
                        reader_handle,
                        felt_to_byte_array(&t.actual_fee.0.into()).as_ptr(),
                    );

                    append_trace(
                        reader_handle,
                        jsonrpc::new_transaction_trace(sn_api_txn, t),
                        &mut trace_buffer,
                    );
                }
            },
        }
    }
}

fn felt_to_u128(felt: StarkFelt) -> u128 {
    let bytes = felt.bytes();
    let mut arr = [0u8; 16];
    arr.copy_from_slice(&bytes[16..32]);

    // felts are encoded in big-endian order
    u128::from_be_bytes(arr)
}

fn transaction_from_api(
    tx: StarknetApiTransaction,
    contract_class: Option<ContractClass>,
    paid_fee_on_l1: Option<Fee>,
) -> Result<Transaction, String> {
    match tx {
        StarknetApiTransaction::Deploy(deploy) => {
            return Err(format!(
                "Unsupported deploy transaction in the traced block (transaction_hash={})",
                deploy.transaction_hash
            ))
        }
        StarknetApiTransaction::Declare(declare) if contract_class.is_none() => {
            return Err(format!(
                "Declare transaction must be created with a ContractClass (transaction_hash={})",
                declare.transaction_hash()
            ))
        }
        _ => {} // all ok
    };

    Transaction::from_api(tx, contract_class, paid_fee_on_l1)
        .map_err(|err| format!("failed to create transaction from api: {:?}", err))
}

fn append_trace(reader_handle: usize, trace: jsonrpc::TransactionTrace, trace_buffer: &mut Vec<u8>) {
    trace_buffer.clear();
    serde_json::to_writer(&mut *trace_buffer, &trace).unwrap();

    let ptr = trace_buffer.as_ptr();
    let len = trace_buffer.len();

    unsafe {
        JunoAppendTrace(reader_handle, ptr as *const c_void, len);
    };
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
    sequencer_address: StarkFelt,
    gas_price: u128,
) -> BlockContext {
    BlockContext {
        chain_id: ChainId(chain_id_str.into()),
        block_number: BlockNumber(block_number),
        block_timestamp: BlockTimestamp(block_timestamp),

        sequencer_address: ContractAddress::try_from(sequencer_address).unwrap(),
        // https://github.com/starknet-io/starknet-addresses/blob/df19b17d2c83f11c30e65e2373e8a0c65446f17c/bridged_tokens/mainnet.json
        // fee_token_address is the same for all networks
        fee_token_address: ContractAddress::try_from(
            StarkHash::try_from(
                "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
            )
            .unwrap(),
        )
        .unwrap(),
        gas_price: gas_price, // fixed gas price, so that we can return "consumed gas" to Go side
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
            (KECCAK_BUILTIN_NAME.to_string(), N_STEPS_FEE_WEIGHT * 2048.0),
        ])
        .into(),
        invoke_tx_max_n_steps: 1_000_000,
        validate_max_n_steps: 1_000_000,
        max_recursion_depth: 50,
    }
}

fn contract_class_from_sierra_json(sierra_json: &str) -> Result<ContractClass, String> {
    let sierra_class: SierraContractClass =
        serde_json::from_str(sierra_json).map_err(|err| err.to_string())?;
    let casm_class = CasmContractClass::from_contract_class(sierra_class, true)
        .map_err(|err| err.to_string())?;
    let contract_class_v1 = ContractClassV1::try_from(casm_class).map_err(|err| err.to_string())?;

    Ok(contract_class_v1.into())
}
