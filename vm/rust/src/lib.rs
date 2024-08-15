pub mod jsonrpc;
mod juno_state_reader;

#[macro_use]
extern crate lazy_static;

use crate::juno_state_reader::{ptr_to_felt, JunoStateReader};
use blockifier::bouncer::BouncerConfig;
use blockifier::fee::{fee_utils, gas_usage};
use blockifier::state::cached_state::TransactionalState;
use blockifier::transaction::errors::TransactionExecutionError::{
    ContractConstructorExecutionFailed, ExecutionError, ValidateTransactionError,
};
use blockifier::transaction::objects::{GasVector, TransactionExecutionInfo};
use blockifier::{
    blockifier::block::{
        pre_process_block, BlockInfo as BlockifierBlockInfo, BlockNumberHashPair, GasPrices,
    },
    context::{BlockContext, ChainInfo, FeeTokenAddresses, TransactionContext},
    execution::{
        contract_class::ClassInfo,
        entry_point::{CallEntryPoint, CallType, EntryPointExecutionContext},
    },
    state::{cached_state::CachedState, state_api::State},
    transaction::{
        objects::{DeprecatedTransactionInfo, HasRelatedFeeType, TransactionInfo},
        transaction_execution::Transaction,
        transactions::ExecutableTransaction,
    },
    versioned_constants::VersionedConstants,
};
use cairo_vm::vm::runners::cairo_runner::ExecutionResources;
use juno_state_reader::{class_info_from_json_str, felt_to_byte_array};
use serde::Deserialize;
use starknet_api::block::BlockHash;
use starknet_api::core::{ChainId, ClassHash, ContractAddress, EntryPointSelector, PatriciaKey};
use starknet_api::transaction::Transaction as StarknetApiTransaction;
use starknet_api::transaction::{Calldata, Fee, TransactionHash};
use starknet_types_core::felt::Felt;
use std::str::FromStr;
use std::{
    collections::HashMap,
    ffi::{c_char, c_longlong, c_uchar, c_ulonglong, c_void, CStr, CString},
    num::NonZeroU128,
    slice,
    sync::Arc,
};

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
    let contract_addr_felt = Felt::from_bytes_be(&call_info.contract_address);
    let class_hash = if call_info.class_hash == [0; 32] {
        None
    } else {
        Some(ClassHash(Felt::from_bytes_be(&call_info.class_hash)))
    };
    let entry_point_selector_felt = Felt::from_bytes_be(&call_info.entry_point_selector);
    let chain_id_str = unsafe { CStr::from_ptr(chain_id) }.to_str().unwrap();

    let mut calldata_vec: Vec<Felt> = Vec::with_capacity(call_info.len_calldata);
    if call_info.len_calldata > 0 {
        let calldata_slice =
            unsafe { slice::from_raw_parts(call_info.calldata, call_info.len_calldata) };
        for ptr in calldata_slice {
            let data = ptr_to_felt(ptr.cast());
            calldata_vec.push(data);
        }
    }

    let entry_point = CallEntryPoint {
        entry_point_type: starknet_api::deprecated_contract_class::EntryPointType::External,
        entry_point_selector: EntryPointSelector(entry_point_selector_felt),
        calldata: Calldata(calldata_vec.into()),
        storage_address: contract_addr_felt.try_into().unwrap(),
        call_type: CallType::Call,
        class_hash,
        code_address: None,
        caller_address: ContractAddress::default(),
        initial_gas: get_versioned_constants(block_info.version)
            .os_constants
            .gas_costs
            .initial_gas_cost,
    };

    let mut state = CachedState::new(reader);
    let mut resources = ExecutionResources::default();
    let context = EntryPointExecutionContext::new_invoke(
        Arc::new(TransactionContext {
            block_context: build_block_context(
                &mut state,
                &block_info,
                chain_id_str,
                Some(max_steps),
            ),
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
    err_on_revert: c_uchar,
) {
    let txn_json_str = unsafe { CStr::from_ptr(txns_json) }.to_str().unwrap();
    let txns_and_query_bits: Result<Vec<TxnAndQueryBit>, serde_json::Error> =
        serde_json::from_str(txn_json_str);
    if let Err(e) = txns_and_query_bits {
        report_error(reader_handle, e.to_string().as_str(), -1);
        return;
    }
    let txns_and_query_bits = txns_and_query_bits.unwrap();

    let block_info = unsafe { *block_info_ptr };

    let chain_id_str = unsafe { CStr::from_ptr(chain_id) }.to_str().unwrap();

    let mut classes: Result<Vec<Box<serde_json::value::RawValue>>, serde_json::Error> = Ok(vec![]);

    if !classes_json.is_null() {
        let classes_json_str = unsafe { CStr::from_ptr(classes_json) }.to_str().unwrap();
        classes = serde_json::from_str(classes_json_str);
    }
    if let Err(e) = classes {
        report_error(reader_handle, e.to_string().as_str(), -1);
        return;
    }

    let classes = classes.unwrap();

    let charge_fee = skip_charge_fee == 0;
    let validate = skip_validate == 0;

    let paid_fees_on_l1_json_str = unsafe { CStr::from_ptr(paid_fees_on_l1_json) }
        .to_str()
        .unwrap();

    let paid_fees_on_l1: Vec<Fee> = match serde_json::from_str(paid_fees_on_l1_json_str) {
        Ok(f) => f,
        Err(e) => {
            report_error(reader_handle, e.to_string().as_str(), -1);
            return;
        }
    };

    let reader = JunoStateReader::new(reader_handle, block_info.block_number);

    let err_on_revert = err_on_revert != 0;

    let res = cairo_vm_execute(
        reader,
        txns_and_query_bits,
        classes,
        paid_fees_on_l1,
        block_info,
        reader_handle,
        chain_id_str,
        charge_fee,
        validate,
        err_on_revert,
    );

    if let Err(ReportError { txn_index, error }) = res {
        report_error(reader_handle, error.as_str(), txn_index as i64);
    }
}

#[derive(Debug)]
struct ReportError {
    txn_index: usize,
    error: String,
}

fn cairo_vm_execute(
    reader: JunoStateReader,
    txns_and_query_bits: Vec<TxnAndQueryBit>,
    mut classes: Vec<Box<serde_json::value::RawValue>>,
    mut paid_fees_on_l1: Vec<Fee>,
    block_info: BlockInfo,
    reader_handle: usize,
    chain_id: &str,
    charge_fee: bool,
    validate: bool,
    err_on_revert: bool,
) -> Result<(), ReportError> {
    let mut state = CachedState::new(reader);

    let block_context: BlockContext = build_block_context(&mut state, &block_info, chain_id, None);

    let mut trace_buffer = Vec::with_capacity(10_000);

    for (txn_index, txn_and_query_bit) in txns_and_query_bits.iter().enumerate() {
        let mut txn_state: TransactionalState<CachedState<JunoStateReader>> =
            CachedState::create_transactional(&mut state);

        println!(
            "\n\nJuno: `cairoVMExecute`: executing transaction ({}/{}) {}",
            txn_index,
            txns_and_query_bits.len(),
            txn_and_query_bit.txn_hash
        );

        let transaction_execution_info = execute_transaction(
            txn_and_query_bit,
            &mut txn_state,
            &mut classes,
            &mut paid_fees_on_l1,
            &block_context,
            charge_fee,
            validate,
            err_on_revert,
        )
        .map_err(|err| ReportError {
            txn_index,
            error: err,
        })?;

        let actual_fee = transaction_execution_info.transaction_receipt.fee.0.into();
        let data_gas_consumed = transaction_execution_info
            .transaction_receipt
            .da_gas
            .l1_data_gas
            .into();

        let trace = jsonrpc::new_transaction_trace(
            &txn_and_query_bit.txn,
            transaction_execution_info,
            &mut txn_state,
        )
        .map_err(|err| ReportError {
            txn_index,
            error: format!("failed building txn state diff reason: {:?}", err),
        })?;

        // With maybe an iterator that performance an action before going to the next.
        // What I'm looking for sounds like a generator.
        // Important is that you don't yet have the outcomes
        unsafe {
            JunoAppendActualFee(reader_handle, felt_to_byte_array(&actual_fee).as_ptr());
            JunoAppendDataGasConsumed(
                reader_handle,
                felt_to_byte_array(&data_gas_consumed).as_ptr(),
            );
        }
        append_trace(reader_handle, &trace, &mut trace_buffer);
        txn_state.commit();
    }

    Ok(())
}

fn execute_transaction(
    txn_and_query_bit: &TxnAndQueryBit,
    txn_state: &mut TransactionalState<CachedState<JunoStateReader>>,
    classes: &mut Vec<Box<serde_json::value::RawValue>>,
    paid_fees_on_l1: &mut Vec<Fee>,
    block_context: &BlockContext,
    charge_fee: bool,
    validate: bool,
    err_on_revert: bool,
) -> Result<TransactionExecutionInfo, String> {
    let class_info = match txn_and_query_bit.txn.clone() {
        StarknetApiTransaction::Declare(declare_transaction) => {
            if classes.is_empty() {
                Err("missing declared class".to_string())?
            }
            let class_json_str = classes.remove(0);

            let maybe_cc =
                class_info_from_json_str(class_json_str.get(), declare_transaction.class_hash());

            // todo(xrvdg) should be able to clean this up now
            if let Err(e) = &maybe_cc {
                Err(e.to_owned())?
            }
            Some(maybe_cc.unwrap())
        }
        _ => None,
    };
    let paid_fee_on_l1: Option<Fee> = match txn_and_query_bit.txn {
        StarknetApiTransaction::L1Handler(_) => {
            if paid_fees_on_l1.is_empty() {
                Err("missing fee paid on l1b".to_string())?
            }

            Some(paid_fees_on_l1.remove(0))
        }
        _ => None,
    };
    let txn = transaction_from_api(
        txn_and_query_bit.txn.clone(),
        txn_and_query_bit.txn_hash,
        class_info,
        paid_fee_on_l1,
        txn_and_query_bit.query_bit,
    )?;

    let fee_type;
    let minimal_l1_gas_amount_vector: Option<GasVector>;
    let res = match txn {
        Transaction::AccountTransaction(t) => {
            fee_type = t.fee_type();
            minimal_l1_gas_amount_vector =
                Some(gas_usage::estimate_minimal_gas_vector(block_context, &t).unwrap());
            t.execute(txn_state, block_context, charge_fee, validate)
        }
        Transaction::L1HandlerTransaction(t) => {
            fee_type = t.fee_type();
            minimal_l1_gas_amount_vector = None;
            t.execute(txn_state, block_context, charge_fee, validate)
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
            Err(format!(
                "failed txn {} reason: {}",
                txn_and_query_bit.txn_hash, err_string,
            ))
        }

        Ok(mut t) => {
            match &t.revert_error {
                Some(err) if err_on_revert => Err(format!("reverted: {}", err))?,
                _ => (),
            }

            // we are estimating fee, override actual fee calculation
            if t.transaction_receipt.fee.0 == 0 {
                let minimal_l1_gas_amount_vector = minimal_l1_gas_amount_vector.unwrap_or_default();
                let gas_consumed = t
                    .transaction_receipt
                    .gas
                    .l1_gas
                    .max(minimal_l1_gas_amount_vector.l1_gas);
                let data_gas_consumed = t
                    .transaction_receipt
                    .gas
                    .l1_data_gas
                    .max(minimal_l1_gas_amount_vector.l1_data_gas);

                t.transaction_receipt.fee = fee_utils::get_fee_by_gas_vector(
                    block_context.block_info(),
                    GasVector {
                        l1_data_gas: data_gas_consumed,
                        l1_gas: gas_consumed,
                    },
                    &fee_type,
                )
            }

            Ok(t)
        }
    }
}

fn felt_to_u128(felt: Felt) -> u128 {
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
    let sequencer_addr = Felt::from_bytes_be(&block_info.sequencer_address);
    let gas_price_wei_felt = Felt::from_bytes_be(&block_info.gas_price_wei);
    let gas_price_fri_felt = Felt::from_bytes_be(&block_info.gas_price_fri);
    let data_gas_price_wei_felt = Felt::from_bytes_be(&block_info.data_gas_price_wei);
    let data_gas_price_fri_felt = Felt::from_bytes_be(&block_info.data_gas_price_fri);
    let default_gas_price = NonZeroU128::new(1).unwrap();

    let mut old_block_number_and_hash: Option<BlockNumberHashPair> = None;
    if block_info.block_number >= 10 {
        old_block_number_and_hash = Some(BlockNumberHashPair {
            number: starknet_api::block::BlockNumber(block_info.block_number - 10),
            hash: BlockHash(Felt::from_bytes_be(&block_info.block_hash_to_be_revealed)),
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
            eth_l1_gas_price: NonZeroU128::new(felt_to_u128(gas_price_wei_felt))
                .unwrap_or(default_gas_price),
            strk_l1_gas_price: NonZeroU128::new(felt_to_u128(gas_price_fri_felt))
                .unwrap_or(default_gas_price),
            eth_l1_data_gas_price: NonZeroU128::new(felt_to_u128(data_gas_price_wei_felt))
                .unwrap_or(default_gas_price),
            strk_l1_data_gas_price: NonZeroU128::new(felt_to_u128(data_gas_price_fri_felt))
                .unwrap_or(default_gas_price),
        },
        use_kzg_da: block_info.use_blob_data == 1,
    };

    let chain_info = ChainInfo {
        chain_id: ChainId::from(chain_id_str.to_string()),
        fee_token_addresses: FeeTokenAddresses {
            // both addresses are the same for all networks
            eth_fee_token_address: ContractAddress::try_from(
                Felt::from_hex(
                    "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
                )
                .unwrap(),
            )
            .unwrap(),
            strk_fee_token_address: ContractAddress::try_from(
                Felt::from_hex(
                    "0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d",
                )
                .unwrap(),
            )
            .unwrap(),
        },
    };

    pre_process_block(state, old_block_number_and_hash, block_info.block_number).unwrap();

    BlockContext::new(block_info, chain_info, constants, BouncerConfig::max())
}

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
        m
    };
}

fn get_versioned_constants(version: *const c_char) -> VersionedConstants {
    let version_str = unsafe { CStr::from_ptr(version) }.to_str().unwrap();
    let version = StarknetVersion::from_str(&version_str)
        .unwrap_or(StarknetVersion::from_str(&"0.0.0").unwrap());

    if version < StarknetVersion::from_str(&"0.13.0").unwrap() {
        CONSTANTS.get(&"0.13.0".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str(&"0.13.1").unwrap() {
        CONSTANTS.get(&"0.13.1".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str(&"0.13.1.1").unwrap() {
        CONSTANTS.get(&"0.13.1.1".to_string()).unwrap().to_owned()
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
