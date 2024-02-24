pub mod jsonrpc;
mod juno_state_reader;

use crate::juno_state_reader::{ptr_to_felt, JunoStateReader};
use std::{
    collections::HashMap,
    ffi::{c_char, c_uchar, c_ulonglong, c_void, c_longlong, CStr, CString},
    slice,
};

use blockifier::{
    abi::constants::{INITIAL_GAS_COST, N_STEPS_RESOURCE, MAX_STEPS_PER_TX, MAX_VALIDATE_STEPS_PER_TX},
    block_context::{BlockContext, GasPrices, FeeTokenAddresses},
    execution::{
        common_hints::ExecutionMode,
        contract_class::ContractClass,
        entry_point::{CallEntryPoint, CallType, EntryPointExecutionContext, ExecutionResources},
    },
    fee::fee_utils::calculate_tx_fee,
    state::cached_state::{CachedState, GlobalContractCache},
    transaction::{
        objects::{AccountTransactionContext, DeprecatedAccountTransactionContext, HasRelatedFeeType},
        transaction_execution::Transaction,
        transactions::ExecutableTransaction,
        errors::TransactionExecutionError::{
            ContractConstructorExecutionFailed,
            ExecutionError,
            ValidateTransactionError,
        },
    },
};
use cairo_vm::vm::runners::builtin_runner::{
    BITWISE_BUILTIN_NAME, EC_OP_BUILTIN_NAME, HASH_BUILTIN_NAME, KECCAK_BUILTIN_NAME,
    OUTPUT_BUILTIN_NAME, POSEIDON_BUILTIN_NAME, RANGE_CHECK_BUILTIN_NAME,
    SEGMENT_ARENA_BUILTIN_NAME, SIGNATURE_BUILTIN_NAME,
};
use juno_state_reader::{contract_class_from_json_str, felt_to_byte_array};
use serde::Deserialize;
use starknet_api::transaction::{Calldata, Transaction as StarknetApiTransaction, TransactionHash};
use starknet_api::{
    block::{BlockNumber, BlockTimestamp},
    deprecated_contract_class::EntryPointType,
    hash::StarkFelt,
    transaction::Fee,
};
use starknet_api::{
    core::{ChainId, ClassHash, ContractAddress, EntryPointSelector},
    hash::StarkHash,
};

extern "C" {
    fn JunoReportError(reader_handle: usize, txnIndex: c_longlong, err: *const c_char);
    fn JunoAppendTrace(reader_handle: usize, json_trace: *const c_void, len: usize);
    fn JunoAppendResponse(reader_handle: usize, ptr: *const c_uchar);
    fn JunoAppendActualFee(reader_handle: usize, ptr: *const c_uchar);
}

const N_STEPS_FEE_WEIGHT: f64 = 0.005;

#[no_mangle]
pub extern "C" fn cairoVMCall(
    contract_address: *const c_uchar,
    class_hash: *const c_uchar,
    entry_point_selector: *const c_uchar,
    calldata: *const *const c_uchar,
    len_calldata: usize,
    reader_handle: usize,
    block_number: c_ulonglong,
    block_timestamp: c_ulonglong,
    chain_id: *const c_char,
    max_steps: c_ulonglong,
) {
    let reader = JunoStateReader::new(reader_handle, block_number);
    let contract_addr_felt = ptr_to_felt(contract_address);
    let class_hash = if class_hash.is_null() {
        None
    } else {
        Some(ClassHash(ptr_to_felt(class_hash)))
    };
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
        class_hash: class_hash,
        code_address: None,
        caller_address: ContractAddress::default(),
        initial_gas: INITIAL_GAS_COST,
    };

    const GAS_PRICES: GasPrices = GasPrices {
        eth_l1_gas_price: 1,
        strk_l1_gas_price: 1,
    };
    let mut state = CachedState::new(reader, GlobalContractCache::default());
    let mut resources = ExecutionResources::default();
    let context = EntryPointExecutionContext::new(
        &build_block_context(
            chain_id_str,
            block_number,
            block_timestamp,
            StarkFelt::default(),
            GAS_PRICES,
            Some(max_steps),
        ),
        &AccountTransactionContext::Deprecated(DeprecatedAccountTransactionContext::default()),
        ExecutionMode::Execute,
        false,
    );
    if let Err(e) = context {
        report_error(reader_handle, e.to_string().as_str(), -1);
        return;
    }
    match entry_point.execute(&mut state, &mut resources, &mut context.unwrap()) {
        Err(e) => report_error(reader_handle, e.to_string().as_str(), -1),
        Ok(t) => {
            for data in t.execution.retdata.0 {
                unsafe {
                    JunoAppendResponse(reader_handle, felt_to_byte_array(&data).as_ptr());
                };
            }
        }
    }
}

#[derive(Deserialize)]
pub struct TxnAndQueryBit {
    pub txn: StarknetApiTransaction,
    pub txn_hash: TransactionHash,
    pub query_bit: bool,
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
    skip_validate: c_uchar,
    err_on_revert: c_uchar,
    gas_price_wei: *const c_uchar,
    gas_price_strk: *const c_uchar,
    legacy_json: c_uchar,
) {
    let reader = JunoStateReader::new(reader_handle, block_number);
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

    let txns_and_query_bits = txns_and_query_bits.unwrap();
    let mut classes = classes.unwrap();

    let sequencer_address_felt = ptr_to_felt(sequencer_address);
    let gas_price_wei_felt = ptr_to_felt(gas_price_wei);
    let gas_price_strk_felt = ptr_to_felt(gas_price_strk);
    let block_context: BlockContext = build_block_context(
        chain_id_str,
        block_number,
        block_timestamp,
        sequencer_address_felt,
        GasPrices {
            eth_l1_gas_price: felt_to_u128(gas_price_wei_felt),
            strk_l1_gas_price: felt_to_u128(gas_price_strk_felt),
        },
        None
    );
    let mut state = CachedState::new(reader, GlobalContractCache::default());
    let charge_fee = skip_charge_fee == 0;
    let validate = skip_validate == 0;

    let mut trace_buffer = Vec::with_capacity(10_000);

    for (txn_index, txn_and_query_bit) in txns_and_query_bits.iter().enumerate() {
        let contract_class = match txn_and_query_bit.txn.clone() {
            StarknetApiTransaction::Declare(_) => {
                if classes.is_empty() {
                    report_error(reader_handle, "missing declared class", txn_index as i64);
                    return;
                }
                let class_json_str = classes.remove(0);

                let maybe_cc = contract_class_from_json_str(class_json_str.get());
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

        let txn = transaction_from_api(
            txn_and_query_bit.txn.clone(),
            txn_and_query_bit.txn_hash,
            contract_class,
            paid_fee_on_l1,
            txn_and_query_bit.query_bit,
        );
        if let Err(e) = txn {
            report_error(reader_handle, e.to_string().as_str(), txn_index as i64);
            return;
        }

        let mut txn_state = CachedState::create_transactional(&mut state);
        let fee_type;
        let res = match txn.unwrap() {
            Transaction::AccountTransaction(t) => {
                fee_type = t.fee_type();
                t.execute(&mut txn_state, &block_context, charge_fee, validate)
            }
            Transaction::L1HandlerTransaction(t) => {
                fee_type = t.fee_type();
                t.execute(&mut txn_state, &block_context, charge_fee, validate)
            }
        };

        match res {
            Err(error) => {
                let err_string = match &error {
                    ContractConstructorExecutionFailed(e)
                        | ExecutionError(e)
                        | ValidateTransactionError(e) => format!("{error} {e}"),
                    other => other.to_string()
                };
                report_error(
                    reader_handle,
                    format!(
                        "failed txn {} reason: {}",
                        txn_and_query_bit.txn_hash,
                        err_string,
                    )
                    .as_str(),
                    txn_index as i64
                );
                return;
            }
            Ok(mut t) => {
                if t.is_reverted() && err_on_revert != 0 {
                    report_error(
                        reader_handle,
                        format!("reverted: {}", t.revert_error.unwrap())
                        .as_str(),
                        txn_index as i64
                    );
                    return;
                }

                // we are estimating fee, override actual fee calculation
                if !charge_fee {
                    t.actual_fee = calculate_tx_fee(&t.actual_resources, &block_context, &fee_type).unwrap();
                }

                let actual_fee = t.actual_fee.0.into();
                let mut trace =
                    jsonrpc::new_transaction_trace(&txn_and_query_bit.txn, t, &mut txn_state);
                if trace.is_err() {
                    report_error(
                        reader_handle,
                        format!(
                            "failed building txn state diff reason: {:?}",
                            trace.err().unwrap()
                        )
                        .as_str(),
                        txn_index as i64
                    );
                    return;
                }

                unsafe {
                    JunoAppendActualFee(reader_handle, felt_to_byte_array(&actual_fee).as_ptr());
                }
                if legacy_json == 1 {
                    trace.as_mut().unwrap().make_legacy()
                }
                append_trace(reader_handle, trace.as_ref().unwrap(), &mut trace_buffer);
            }
        }
        txn_state.commit();
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
    tx_hash: TransactionHash,
    contract_class: Option<ContractClass>,
    paid_fee_on_l1: Option<Fee>,
    query_bit: bool,
) -> Result<Transaction, String> {
    match tx {
        StarknetApiTransaction::Deploy(_) => {
            return Err(format!(
                "Unsupported deploy transaction in the traced block (transaction_hash={})",
                tx_hash,
            ))
        }
        StarknetApiTransaction::Declare(_) if contract_class.is_none() => {
            return Err(format!(
                "Declare transaction must be created with a ContractClass (transaction_hash={})",
                tx_hash,
            ))
        }
        _ => {} // all ok
    };

    Transaction::from_api(tx, tx_hash, contract_class, paid_fee_on_l1, None, query_bit)
        .map_err(|err| format!("failed to create transaction from api: {:?}", err))
}

fn append_trace(
    reader_handle: usize,
    trace: &jsonrpc::TransactionTrace,
    trace_buffer: &mut Vec<u8>,
) {
    trace_buffer.clear();
    serde_json::to_writer(&mut *trace_buffer, trace).unwrap();

    let ptr = trace_buffer.as_ptr();
    let len = trace_buffer.len();

    unsafe {
        JunoAppendTrace(reader_handle, ptr as *const c_void, len);
    };
}

fn report_error(reader_handle: usize, msg: &str, txn_index: i64) {
    let err_msg = CString::new(msg).unwrap();
    unsafe {
        JunoReportError(reader_handle, txn_index, err_msg.as_ptr());
    };
}

fn build_block_context(
    chain_id_str: &str,
    block_number: c_ulonglong,
    block_timestamp: c_ulonglong,
    sequencer_address: StarkFelt,
    gas_prices: GasPrices,
    max_steps: Option<c_ulonglong>,
) -> BlockContext {
    BlockContext {
        chain_id: ChainId(chain_id_str.into()),
        block_number: BlockNumber(block_number),
        block_timestamp: BlockTimestamp(block_timestamp),

        sequencer_address: ContractAddress::try_from(sequencer_address).unwrap(),
        // https://github.com/starknet-io/starknet-addresses/blob/df19b17d2c83f11c30e65e2373e8a0c65446f17c/bridged_tokens/mainnet.json
        fee_token_addresses: FeeTokenAddresses {
            // both addresses are the same for all networks
            eth_fee_token_address: ContractAddress::try_from(StarkHash::try_from("0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7").unwrap()).unwrap(),
            strk_fee_token_address: ContractAddress::try_from(StarkHash::try_from("0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d").unwrap()).unwrap(),
        },
        gas_prices, // fixed gas price, so that we can return "consumed gas" to Go side
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
        invoke_tx_max_n_steps: max_steps.unwrap_or(MAX_STEPS_PER_TX as u64).try_into().unwrap(),
        validate_max_n_steps: MAX_VALIDATE_STEPS_PER_TX as u32,
        max_recursion_depth: 50,
    }
}
