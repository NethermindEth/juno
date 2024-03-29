pub mod jsonrpc;
mod juno_state_reader;

#[macro_use]
extern crate lazy_static;

use crate::juno_state_reader::{ptr_to_felt, JunoStateReader};
use std::{
    collections::HashMap, ffi::{c_char, c_longlong, c_uchar, c_ulonglong, c_void, CStr, CString}, num::NonZeroU128, slice, sync::Arc
};

use blockifier::{
    block::{pre_process_block, BlockInfo as BlockifierBlockInfo, BlockNumberHashPair, GasPrices}, context::{BlockContext, ChainInfo, FeeTokenAddresses, TransactionContext}, execution::{
        contract_class::ClassInfo,
        entry_point::{CallEntryPoint, CallType, EntryPointExecutionContext},
    }, fee::fee_utils::calculate_tx_fee, state::{cached_state::{CachedState, GlobalContractCache}, state_api::State}, transaction::{
        errors::TransactionExecutionError::{
            ContractConstructorExecutionFailed,
            ExecutionError,
            ValidateTransactionError,
        }, objects::{DeprecatedTransactionInfo, HasRelatedFeeType, TransactionInfo}, transaction_execution::Transaction, transactions::ExecutableTransaction
    }, versioned_constants::VersionedConstants
};
use cairo_vm::vm::runners::cairo_runner::ExecutionResources;
use juno_state_reader::{class_info_from_json_str, felt_to_byte_array};
use serde::Deserialize;
use starknet_api::{block::BlockHash, core::PatriciaKey, transaction::{Calldata, Transaction as StarknetApiTransaction, TransactionHash}};
use starknet_api::{
    deprecated_contract_class::EntryPointType,
    hash::StarkFelt,
    transaction::Fee,
};
use starknet_api::{
    core::{ChainId, ClassHash, ContractAddress, EntryPointSelector},
    hash::StarkHash,
};
use std::str::FromStr;

extern "C" {
    fn JunoReportError(reader_handle: usize, txnIndex: c_longlong, err: *const c_char);
    fn JunoAppendTrace(reader_handle: usize, json_trace: *const c_void, len: usize);
    fn JunoAppendResponse(reader_handle: usize, ptr: *const c_uchar);
    fn JunoAppendActualFee(reader_handle: usize, ptr: *const c_uchar);
    fn JunoAppendDataGasConsumed(reader_handle: usize, ptr: *const c_uchar);
}

#[repr(C)]
#[derive(Clone, Copy)]
pub struct CallInfo {
    pub contract_address: [c_uchar; 32],
    pub class_hash: [c_uchar; 32],
    pub entry_point_selector: [c_uchar; 32],
    pub calldata: *const *const c_uchar,
    pub len_calldata: usize
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
pub extern "C" fn cairoVMCall(
    call_info_ptr: *const CallInfo,
    block_info_ptr: *const BlockInfo,
    reader_handle: usize,
    chain_id: *const c_char,
    max_steps: c_ulonglong,
) {
    let block_info = unsafe { *block_info_ptr };
    let call_info = unsafe { *call_info_ptr };

    let reader = JunoStateReader::new(reader_handle, block_info.block_number);
    let contract_addr_felt = StarkFelt::new(call_info.contract_address).unwrap();
    let class_hash = if call_info.class_hash == [0; 32] {
        None
    } else {
        Some(ClassHash(StarkFelt::new(call_info.class_hash).unwrap()))
    };
    let entry_point_selector_felt = StarkFelt::new(call_info.entry_point_selector).unwrap();
    let chain_id_str = unsafe { CStr::from_ptr(chain_id) }.to_str().unwrap();

    let mut calldata_vec: Vec<StarkFelt> = Vec::with_capacity(call_info.len_calldata);
    if call_info.len_calldata > 0 {
        let calldata_slice = unsafe { slice::from_raw_parts(call_info.calldata, call_info.len_calldata) };
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
        initial_gas: get_versioned_constants(block_info.version).gas_cost("initial_gas_cost"),
    };

    let mut state = CachedState::new(reader, GlobalContractCache::new(1));
    let mut resources = ExecutionResources::default();
    let context = EntryPointExecutionContext::new_invoke(
        Arc::new(TransactionContext {
            block_context: build_block_context(&mut state, &block_info, chain_id_str, Some(max_steps)),
            tx_info: TransactionInfo::Deprecated(DeprecatedTransactionInfo::default()),
        }),
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
    paid_fees_on_l1_json: *const c_char,
    block_info_ptr: *const BlockInfo,
    reader_handle: usize,
    chain_id: *const c_char,
    skip_charge_fee: c_uchar,
    skip_validate: c_uchar,
    err_on_revert: c_uchar
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

    let mut state = CachedState::new(reader, GlobalContractCache::new(1));
    let txns_and_query_bits = txns_and_query_bits.unwrap();
    let mut classes = classes.unwrap();
    let block_context: BlockContext = build_block_context(&mut state, &block_info, chain_id_str, None);
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
                if  t.actual_fee.0 == 0 {
                    t.actual_fee = calculate_tx_fee(&t.actual_resources, &block_context, &fee_type).unwrap();
                }

                let actual_fee = t.actual_fee.0.into();
                let data_gas_consumed = t.da_gas.l1_data_gas.into();

                let trace =
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
                    JunoAppendDataGasConsumed(reader_handle, felt_to_byte_array(&data_gas_consumed).as_ptr());
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
    class_info: Option<ClassInfo>,
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
        StarknetApiTransaction::Declare(_) if class_info.is_none() => {
            return Err(format!(
                "Declare transaction must be created with a ContractClass (transaction_hash={})",
                tx_hash,
            ))
        }
        _ => {} // all ok
    };

    Transaction::from_api(tx, tx_hash, class_info, paid_fee_on_l1, None, query_bit)
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
) -> BlockContext {
    let sequencer_addr =  StarkFelt::new(block_info.sequencer_address).unwrap();
    let gas_price_wei_felt = StarkFelt::new(block_info.gas_price_wei).unwrap();
    let gas_price_fri_felt = StarkFelt::new(block_info.gas_price_fri).unwrap();
    let data_gas_price_wei_felt = StarkFelt::new(block_info.data_gas_price_wei).unwrap();
    let data_gas_price_fri_felt = StarkFelt::new(block_info.data_gas_price_fri).unwrap();
    let default_gas_price = NonZeroU128::new(1).unwrap();

    let mut old_block_number_and_hash: Option<BlockNumberHashPair> = None;
    if block_info.block_number >= 10 {
        old_block_number_and_hash = Some(BlockNumberHashPair{
            number: starknet_api::block::BlockNumber(block_info.block_number - 10),
            hash: BlockHash(StarkFelt::new(block_info.block_hash_to_be_revealed).unwrap()),
        })
    }
    let mut constants = get_versioned_constants(block_info.version);
    if let Some(max_steps) = max_steps {
        constants.invoke_tx_max_n_steps = max_steps as u32;
    }

    pre_process_block(state, old_block_number_and_hash, BlockifierBlockInfo{
        block_number: starknet_api::block::BlockNumber(block_info.block_number),
        block_timestamp: starknet_api::block::BlockTimestamp(block_info.block_timestamp),
        sequencer_address: ContractAddress(PatriciaKey::try_from(sequencer_addr).unwrap()),
        gas_prices: GasPrices {
            eth_l1_gas_price: NonZeroU128::new(felt_to_u128(gas_price_wei_felt)).unwrap_or(default_gas_price),
            strk_l1_gas_price: NonZeroU128::new(felt_to_u128(gas_price_fri_felt)).unwrap_or(default_gas_price),
            eth_l1_data_gas_price: NonZeroU128::new(felt_to_u128(data_gas_price_wei_felt)).unwrap_or(default_gas_price),
            strk_l1_data_gas_price: NonZeroU128::new(felt_to_u128(data_gas_price_fri_felt)).unwrap_or(default_gas_price),
        },
        use_kzg_da: block_info.use_blob_data == 1,
    }, ChainInfo{
        chain_id: ChainId(chain_id_str.to_string()),
        fee_token_addresses: FeeTokenAddresses {
            // both addresses are the same for all networks
            eth_fee_token_address: ContractAddress::try_from(StarkHash::try_from("0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7").unwrap()).unwrap(),
            strk_fee_token_address: ContractAddress::try_from(StarkHash::try_from("0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d").unwrap()).unwrap(),
        },
    }, constants).unwrap()
}




lazy_static! {
    static ref CONSTANTS: HashMap<String, VersionedConstants> = {
        let mut m = HashMap::new();
        m.insert("0.13.0".to_string(), serde_json::from_slice(include_bytes!("../versioned_constants_13_0.json")).unwrap());
        m.insert("0.13.1".to_string(), serde_json::from_slice(include_bytes!("../versioned_constants_13_1.json")).unwrap());
        m
    };
}

fn get_versioned_constants(version: *const c_char) -> VersionedConstants {
    let version_str = unsafe { CStr::from_ptr(version) }.to_str().unwrap();
    let version = StarknetVersion::from_str(&version_str).unwrap_or(StarknetVersion::from_str(&"0.0.0").unwrap());

    if version < StarknetVersion::from_str(&"0.13.1").unwrap() {
        CONSTANTS.get(&"0.13.0".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str(&"0.13.1.1").unwrap() {
        CONSTANTS.get(&"0.13.1".to_string()).unwrap().to_owned()
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