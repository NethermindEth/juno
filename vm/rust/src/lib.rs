pub mod jsonrpc;
mod juno_state_reader;

#[macro_use]
extern crate lazy_static;

use crate::juno_state_reader::{ptr_to_felt, JunoStateReader};
use std::{
    collections::HashMap,
    ffi::{c_char, c_longlong, c_uchar, c_ulonglong, c_void, CStr, CString},
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
        errors::TransactionExecutionError::{
            ContractConstructorExecutionFailed, ExecutionError, ValidateTransactionError,
        },
        objects::{DeprecatedTransactionInfo, HasRelatedFeeType, TransactionInfo},
        transaction_execution::Transaction,
        transactions::ExecutableTransaction,
    },
    versioned_constants::VersionedConstants,
};
use juno_state_reader::{class_info_from_json_str, felt_to_byte_array};
use serde::Deserialize;
use starknet_api::{
    block::BlockHash,
    contract_class::{ClassInfo, EntryPointType},
    core::PatriciaKey,
    execution_resources::GasVector,
    transaction::{
        fields::{Calldata, Fee, GasVectorComputationMode},
        Transaction as StarknetApiTransaction, TransactionHash,
    },
};
use starknet_api::{
    block::{
        BlockHashAndNumber, BlockInfo as BlockifierBlockInfo, GasPrice, GasPriceVector, GasPrices,
        NonzeroGasPrice,
    },
    execution_resources::GasAmount,
};
use starknet_api::{
    core::{ChainId, ClassHash, ContractAddress, EntryPointSelector},
    hash::StarkHash,
};
use starknet_types_core::felt::Felt;
use std::str::FromStr;

type StarkFelt = Felt;

extern "C" {
    fn JunoReportError(reader_handle: usize, txnIndex: c_longlong, err: *const c_char);
    fn JunoAppendTrace(reader_handle: usize, json_trace: *const c_void, len: usize);
    fn JunoAppendResponse(reader_handle: usize, ptr: *const c_uchar);
    fn JunoAppendActualFee(reader_handle: usize, ptr: *const c_uchar);
    fn JunoAppendGasConsumed(reader_handle: usize, ptr: *const c_uchar, ptr: *const c_uchar);
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
    pub sequencer_address: [c_uchar; 32],
    pub gas_price_wei: [c_uchar; 32],
    pub gas_price_fri: [c_uchar; 32],
    pub version: *const c_char,
    pub block_hash_to_be_revealed: [c_uchar; 32],
    pub data_gas_price_wei: [c_uchar; 32],
    pub data_gas_price_fri: [c_uchar; 32],
    pub use_blob_data: c_uchar,
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

    // todo(rdr): initial gas should be different based on the execution method used: vm or native!
    // There should be some check.
    let mut initial_gas = get_versioned_constants(block_info.version).infinite_gas_for_vm_mode();

    let entry_point = CallEntryPoint {
        entry_point_type: EntryPointType::External,
        entry_point_selector: EntryPointSelector(entry_point_selector_felt),
        calldata: Calldata(calldata_vec.into()),
        storage_address: contract_addr_felt.try_into().unwrap(),
        call_type: CallType::Call,
        class_hash,
        code_address: None,
        caller_address: ContractAddress::default(),
        initial_gas,
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
    match entry_point.execute(&mut state, &mut context, &mut initial_gas) {
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

pub struct TxnAndQueryBit {
    pub txn: StarknetApiTransaction,
    pub txn_hash: TransactionHash,
    // Only required when deploying a new account
    pub account_execution_flags: Option<AccountExecutionFlags>,
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
) {
    let block_info = unsafe { *block_info_ptr };
    let reader = JunoStateReader::new(reader_handle, block_info.block_number);
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

    let mut trace_buffer = Vec::with_capacity(10_000);

    for (txn_index, txn_and_query_bit) in txns_and_query_bits.iter().enumerate() {
        let class_info = match txn_and_query_bit.txn.clone() {
            StarknetApiTransaction::Declare(_) => {
                if classes.is_empty() {
                    report_error(reader_handle, "missing declared class", txn_index as i64);
                    return;
                }
                let class_json_str = classes.remove(0);

                let maybe_cc = class_info_from_json_str(class_json_str.get());
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
            class_info,
            paid_fee_on_l1,
            txn_and_query_bit.account_execution_flags,
        );
        if let Err(e) = txn {
            report_error(reader_handle, e.to_string().as_str(), txn_index as i64);
            return;
        }

        let mut txn_state = CachedState::create_transactional(&mut state);
        let fee_type;
        let minimal_l1_gas_amount_vector: Option<GasVector>;
        let res = match txn.unwrap() {
            Transaction::Account(t) => {
                fee_type = t.fee_type();
                minimal_l1_gas_amount_vector = Some(gas_usage::estimate_minimal_gas_vector(
                    &block_context,
                    &t,
                    // todo(rdr): Which si the right gas vector, how to select it
                    &GasVectorComputationMode::All,
                ));
                t.execute(&mut txn_state, &block_context)
            }
            Transaction::L1Handler(t) => {
                fee_type = t.fee_type();
                minimal_l1_gas_amount_vector = None;
                t.execute(&mut txn_state, &block_context)
            }
        };

        match res {
            Err(error) => {
                let err_string = match &error {
                    ContractConstructorExecutionFailed(e) => format!("{error} {e}"),
                    ExecutionError { error: e, .. } | ValidateTransactionError { error: e, .. } => {
                        format!("{error} {e}")
                    }
                    other => other.to_string(),
                };
                report_error(
                    reader_handle,
                    format!(
                        "failed txn {} reason: {}",
                        txn_and_query_bit.txn_hash, err_string,
                    )
                    .as_str(),
                    txn_index as i64,
                );
                return;
            }
            Ok(mut t) => {
                if t.is_reverted() && err_on_revert != 0 {
                    report_error(
                        reader_handle,
                        format!("reverted: {}", t.revert_error.unwrap()).as_str(),
                        txn_index as i64,
                    );
                    return;
                }

                // we are estimating fee, override actual fee calculation
                if t.receipt.fee.0 == 0 {
                    let minimal_l1_gas_amount_vector =
                        minimal_l1_gas_amount_vector.unwrap_or_default();
                    let gas_consumed = t
                        .receipt
                        .gas
                        .l1_gas
                        .max(minimal_l1_gas_amount_vector.l1_gas);
                    let data_gas_consumed = t
                        .receipt
                        .gas
                        .l1_data_gas
                        .max(minimal_l1_gas_amount_vector.l1_data_gas);

                    t.receipt.fee = fee_utils::get_fee_by_gas_vector(
                        block_context.block_info(),
                        GasVector {
                            l1_data_gas: data_gas_consumed,
                            l1_gas: gas_consumed,
                            // todo(rdr): what goes here? Random value currently
                            l2_gas: GasAmount(8),
                        },
                        &fee_type,
                    )
                }

                let actual_fee: Felt = t.receipt.fee.0.into();
                let da_gas_l1_gas = t.receipt.da_gas.l1_gas.into();
                let da_gas_l1_data_gas = t.receipt.da_gas.l1_data_gas.into();
                // todo(rdr): Are these resurce usage still valid?
                let execution_steps = t
                    .receipt
                    .resources
                    .computation
                    .vm_resources
                    .n_steps
                    .try_into()
                    .unwrap_or(u64::MAX);

                let trace =
                    jsonrpc::new_transaction_trace(&txn_and_query_bit.txn, t, &mut txn_state);
                if let Err(e) = trace {
                    report_error(
                        reader_handle,
                        format!("failed building txn state diff reason: {:?}", e).as_str(),
                        txn_index as i64,
                    );
                    return;
                }

                unsafe {
                    JunoAppendActualFee(reader_handle, felt_to_byte_array(&actual_fee).as_ptr());
                    JunoAppendGasConsumed(
                        reader_handle,
                        felt_to_byte_array(&da_gas_l1_gas).as_ptr(),
                        felt_to_byte_array(&da_gas_l1_data_gas).as_ptr(),
                    );
                    JunoAddExecutionSteps(reader_handle, execution_steps)
                }
                append_trace(reader_handle, trace.as_ref().unwrap(), &mut trace_buffer);
            }
        }
        txn_state.commit();
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
    execution_flags: ExecutionFlags,
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
    state: &mut dyn State,
    block_info: &BlockInfo,
    chain_id_str: &str,
    max_steps: Option<c_ulonglong>,
    _concurrency_mode: bool,
) -> Result<BlockContext> {
    let sequencer_addr = StarkFelt::from_bytes_be(&block_info.sequencer_address);
    let gas_price_eth = StarkFelt::from_bytes_be(&block_info.gas_price_wei);
    let gas_price_strk = StarkFelt::from_bytes_be(&block_info.gas_price_fri);
    let data_gas_price_eth = StarkFelt::from_bytes_be(&block_info.data_gas_price_wei);
    let data_gas_price_strk = StarkFelt::from_bytes_be(&block_info.data_gas_price_fri);
    let default_gas_price = NonzeroGasPrice::new(GasPrice::from(1_u128)).unwrap();

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
                l1_gas_price: NonzeroGasPrice::new(felt_to_u128(gas_price_eth).into())?,
                l1_data_gas_price: NonzeroGasPrice::new(felt_to_u128(data_gas_price_eth).into())?,
                // todo(rdr): not sure what goes here?
                l2_gas_price: default_gas_price,
            },
            strk_gas_prices: GasPriceVector {
                l1_gas_price: NonzeroGasPrice::new(felt_to_u128(gas_price_strk).into())?,
                l1_data_gas_price: NonzeroGasPrice::new(felt_to_u128(data_gas_price_strk).into())?,
                // todo(rdr): not sure what goes here?
                l2_gas_price: default_gas_price,
            },
        },
        use_kzg_da: block_info.use_blob_data == 1,
    };
    let chain_info = ChainInfo {
        chain_id: ChainId::from(chain_id_str.to_string()),
        fee_token_addresses: FeeTokenAddresses {
            // both addresses are the same for all networks
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

// todo(rdr): Should I add versioned constants 13.4?. Where are 13.3?
lazy_static! {
    static ref CONSTANTS: HashMap<String, VersionedConstants> = {
        let mut m = HashMap::new();
        m.insert(
            "0.13.0".to_string(),
            serde_json::from_slice(include_bytes!("../versioned_constants_13_0.json")).unwrap(),
        );
        m.insert(
            "0.13.1".to_string(),
            serde_json::from_slice(include_bytes!("../versioned_constants_13_1.json")).unwrap(),
        );
        m.insert(
            "0.13.1.1".to_string(),
            serde_json::from_slice(include_bytes!("../versioned_constants_13_1_1.json")).unwrap(),
        );
        m.insert(
            "0.13.2".to_string(),
            serde_json::from_slice(include_bytes!("../versioned_constants_13_2.json")).unwrap(),
        );
        m
    };
}

#[allow(static_mut_refs)]
fn get_versioned_constants(version: *const c_char) -> VersionedConstants {
    let version_str = unsafe { CStr::from_ptr(version) }
        .to_str()
        .unwrap_or("0.0.0");

    let version = match StarknetVersion::from_str(version_str) {
        Ok(v) => v,
        Err(_) => StarknetVersion::from_str("0.0.0").unwrap(),
    };

    if let Some(constants) = unsafe { &CUSTOM_VERSIONED_CONSTANTS } {
        constants.clone()
    } else if version < StarknetVersion::from_str("0.13.1").unwrap() {
        CONSTANTS.get(&"0.13.0".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str("0.13.1.1").unwrap() {
        CONSTANTS.get(&"0.13.1".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str("0.13.2").unwrap() {
        CONSTANTS.get(&"0.13.1.1".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str("0.13.2.1").unwrap() {
        CONSTANTS.get(&"0.13.2".to_string()).unwrap().to_owned()
    } else {
        VersionedConstants::latest_constants().to_owned()
    }
}

#[derive(Default, PartialEq, Eq, PartialOrd, Ord)]
pub struct StarknetVersion(u8, u8, u8, u8);

impl StarknetVersion {
    pub const fn new(a: u8, b: u8, c: u8, d: u8) -> Self {
        StarknetVersion(a, b, c, d)
    }
}

impl FromStr for StarknetVersion {
    type Err = anyhow::Error;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        if s.is_empty() {
            return Ok(StarknetVersion::new(0, 0, 0, 0));
        }

        let parts: Vec<_> = s.split('.').collect();
        anyhow::ensure!(
            parts.len() == 3 || parts.len() == 4,
            "Invalid version string, expected 3 or 4 parts but got {}",
            parts.len()
        );

        let a = parts[0].parse()?;
        let b = parts[1].parse()?;
        let c = parts[2].parse()?;
        let d = parts.get(3).map(|x| x.parse()).transpose()?.unwrap_or(0);

        Ok(StarknetVersion(a, b, c, d))
    }
}

static mut CUSTOM_VERSIONED_CONSTANTS: Option<VersionedConstants> = None;

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

    match serde_json::from_str(json_str) {
        Ok(parsed) => unsafe {
            CUSTOM_VERSIONED_CONSTANTS = Some(parsed);
            CString::new("").unwrap().into_raw() // No error, return an empty string
        },
        Err(_) => CString::new("Failed to parse JSON").unwrap().into_raw(),
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
