pub mod jsonrpc;
mod juno_state_reader;
mod juno_versioned_constants;
use std::{borrow::BorrowMut, num::NonZeroU128};
use crate::juno_state_reader::{ptr_to_felt,felt_ptr_to_u128, convert_to_c_uchar,JunoStateReader};
use crate::juno_versioned_constants::{versioned_constants,StarknetVersion};
use std::{
    ffi::{c_char, c_uchar, c_ulonglong, c_void, c_longlong, CStr, CString},
    slice,
};
use std::sync::Arc;
use cairo_vm::vm::runners::cairo_runner::ExecutionResources;
use blockifier::{
    block::{BlockInfo,GasPrices,pre_process_block,BlockNumberHashPair}, 
    context::{ ChainInfo, FeeTokenAddresses, TransactionContext}, 
    execution::{
        common_hints::ExecutionMode,
        entry_point::{CallEntryPoint, CallType, EntryPointExecutionContext},
        contract_class::ClassInfo,
    }, 
    fee::fee_utils::calculate_tx_fee, 
    state::cached_state::{CachedState, GlobalContractCache}, 
    transaction::{
        errors::TransactionExecutionError::{
            ContractConstructorExecutionFailed,
            ExecutionError,
            ValidateTransactionError,
        }, objects::{DeprecatedTransactionInfo, HasRelatedFeeType, TransactionInfo}, 
        transaction_execution::Transaction, 
        transactions::ExecutableTransaction
    },
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
use lazy_static::lazy_static;
use std::convert::TryInto;

extern "C" {
    fn JunoReportError(reader_handle: usize, txnIndex: c_longlong, err: *const c_char);
    fn JunoAppendTrace(reader_handle: usize, json_trace: *const c_void, len: usize);
    fn JunoAppendResponse(reader_handle: usize, ptr: *const c_uchar);
    fn JunoAppendActualFee(reader_handle: usize, ptr: *const c_uchar);
}

const GLOBAL_CONTRACT_CACHE_SIZE: usize= 100; // Todo ? default used to set this to 100.

lazy_static! {
    pub static ref FEE_TOKEN_ADDRESSES: FeeTokenAddresses = {
        let eth_fee_token_address = StarkHash::try_from("0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7")
            .and_then(|hash| ContractAddress::try_from(hash))
            .unwrap();

        let strk_fee_token_address = StarkHash::try_from("0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d")
            .and_then(|hash| ContractAddress::try_from(hash))
            .unwrap();

        FeeTokenAddresses {
            eth_fee_token_address,
            strk_fee_token_address,
        }
    };
}



#[no_mangle]
pub extern "C" fn cairoVMCall(
    block_info_json: *const c_char,
    contract_address: *const c_uchar,
    class_hash: *const c_uchar,
    entry_point_selector: *const c_uchar,
    calldata: *const *const c_uchar,
    len_calldata: usize,
    reader_handle: usize,
    chain_id: *const c_char,
    max_steps: c_ulonglong,
) {
    
    let block_info_json_str = unsafe { CStr::from_ptr(block_info_json) }.to_str().unwrap();
    let block_info_json: Result<BlockInfoJuno, serde_json::Error> =
    serde_json::from_str(block_info_json_str);
    if let Err(e) = block_info_json {
        report_error(reader_handle, e.to_string().as_str(), -1);
        return;
    }
    let block_info_juno = block_info_json.unwrap();

    let block_version_str : &str = &block_info_juno.block_version.unwrap();
    let block_version =StarknetVersion::from_str(block_version_str);
    let mut versioned_constants = versioned_constants::for_version(&block_version).unwrap().clone();

    versioned_constants.invoke_tx_max_n_steps = max_steps as u32; // Todo ?

    let reader = JunoStateReader::new(reader_handle, block_info_juno.block_number);
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
        initial_gas: versioned_constants.gas_cost("initial_gas_cost"),
    };
    
    let mut state = CachedState::new(reader, GlobalContractCache::new(GLOBAL_CONTRACT_CACHE_SIZE));
    let mut resources = ExecutionResources::default();

    let gas_prices: GasPrices = GasPrices {
        eth_l1_gas_price: NonZeroU128::new(1).unwrap(),
        strk_l1_gas_price: NonZeroU128::new(1).unwrap(),
        eth_l1_data_gas_price:NonZeroU128::new(1).unwrap(),
        strk_l1_data_gas_price: NonZeroU128::new(1).unwrap(),
    };

    let use_kzg_da = false;
    let block_info = build_block_info(block_info_juno.block_number, block_info_juno.block_timestamp,StarkFelt::default(),gas_prices,use_kzg_da);
    let chain_info = build_chain_info(chain_id_str, (*FEE_TOKEN_ADDRESSES).clone());

    
    let block_hash_u128 = felt_ptr_to_u128(convert_to_c_uchar(block_info_juno.block_hash));
    let block_hash_and_number = Some(BlockNumberHashPair::new(block_info_juno.block_number, StarkFelt::from_u128(block_hash_u128)));

    let block_context = pre_process_block(state.borrow_mut(),block_hash_and_number,block_info,chain_info,versioned_constants.clone());

    let tx_context = TransactionContext {
        block_context:block_context.unwrap(),
        tx_info:TransactionInfo::Deprecated(DeprecatedTransactionInfo::default()),
    };

    let context = EntryPointExecutionContext::new(
        Arc::new(tx_context),
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

#[derive(Deserialize)]
pub struct ClassInformation {
    pub abi_length :usize,
    pub sierra_program_length: usize,
}

#[derive(Deserialize)]
pub struct BlockInfoJuno {
    block_number: c_ulonglong,
    block_timestamp: c_ulonglong,
    block_version:  Option<String>,
    block_hash:   Option<String>,
}


#[no_mangle]
pub extern "C" fn cairoVMExecute(
    txns_json: *const c_char,
    classes_json: *const c_char,
    contract_info_json: *const c_char,
    reader_handle: usize,
    block_info_json: *const c_char,
    chain_id: *const c_char,
    sequencer_address: *const c_uchar,
    paid_fees_on_l1_json: *const c_char,
    skip_charge_fee: c_uchar,
    skip_validate: c_uchar,
    err_on_revert: c_uchar,
    gas_price_wei: *const c_uchar,
    gas_price_strk: *const c_uchar,
    legacy_json: c_uchar,
    da_gas_price_wei: *const c_uchar,
    da_gas_price_fri: *const c_uchar,
    use_kzg_da: c_uchar 
) {

    let block_info_json_str = unsafe { CStr::from_ptr(block_info_json) }.to_str().unwrap();
    let block_info_juno: Result<BlockInfoJuno, serde_json::Error> =
    serde_json::from_str(block_info_json_str);
    if let Err(e) = block_info_juno {
        report_error(reader_handle, e.to_string().as_str(), -1);
        return;
    }
    let block_info_juno = block_info_juno.unwrap();
    
    let block_version_str : &str = &block_info_juno.block_version.unwrap();
    let block_version =StarknetVersion::from_str(block_version_str);
    let versioned_constants = versioned_constants::for_version(&block_version).unwrap().clone();

    let reader = JunoStateReader::new(reader_handle, block_info_juno.block_number);
    let chain_id_str = unsafe { CStr::from_ptr(chain_id) }.to_str().unwrap();
    let txn_json_str = unsafe { CStr::from_ptr(txns_json) }.to_str().unwrap();
    let contract_info_json_str = unsafe { CStr::from_ptr(contract_info_json) }.to_str().unwrap();
    
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

    let contract_infos: Result<Vec<ClassInformation>, serde_json::Error> =
        serde_json::from_str(contract_info_json_str);
    if let Err(e) = contract_infos {
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
    let mut contract_infos = contract_infos.unwrap();

    let sequencer_address_felt = ptr_to_felt(sequencer_address);

    let gas_prices: GasPrices = GasPrices {
        eth_l1_gas_price: NonZeroU128::new(felt_ptr_to_u128(gas_price_wei)).unwrap(), 
        strk_l1_gas_price: NonZeroU128::new(felt_ptr_to_u128(gas_price_strk)).unwrap(), 
        eth_l1_data_gas_price: NonZeroU128::new(felt_ptr_to_u128(da_gas_price_wei)).unwrap(),
        strk_l1_data_gas_price: NonZeroU128::new(felt_ptr_to_u128(da_gas_price_fri)).unwrap(),
    };

    let use_kzg_da_bool = use_kzg_da != 0;
    let block_info = build_block_info(block_info_juno.block_number,block_info_juno.block_timestamp,sequencer_address_felt,gas_prices, use_kzg_da_bool);
    let chain_info = build_chain_info(chain_id_str,(*FEE_TOKEN_ADDRESSES).clone());

    let mut state: CachedState<JunoStateReader> = CachedState::new(reader, GlobalContractCache::new(GLOBAL_CONTRACT_CACHE_SIZE));

    let block_hash_u128 = felt_ptr_to_u128(convert_to_c_uchar(block_info_juno.block_hash));
    let block_hash_and_number = Some(BlockNumberHashPair::new(block_info_juno.block_number, StarkFelt::from_u128(block_hash_u128)));

    let block_context = pre_process_block(state.borrow_mut(),block_hash_and_number,block_info,chain_info,versioned_constants.clone()).unwrap();

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
        
                let maybe_cc = contract_class_from_json_str(class_json_str.get());
                if let Err(e) = maybe_cc {
                    report_error(reader_handle, e.to_string().as_str(), txn_index as i64);
                    return;
                }
                let contract_class_unwrap = maybe_cc.unwrap();
        
                if contract_infos.is_empty(){
                    report_error(reader_handle, "missing contract info", txn_index as i64);
                    return;
                }
        
                let contract_info = contract_infos.remove(0);
        
                let maybe_class_info = ClassInfo::new(
                    &contract_class_unwrap,
                    contract_info.sierra_program_length,
                    contract_info.abi_length,
                );
                if let Err(e) = maybe_class_info {
                    report_error(reader_handle, e.to_string().as_str(), txn_index as i64);
                    return;
                }
                Some(maybe_class_info.unwrap())
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


fn build_block_info(
    block_number: c_ulonglong,
    block_timestamp: c_ulonglong,
    sequencer_address: StarkFelt,
    gas_prices: GasPrices,
    use_kzg_da: bool,
) -> BlockInfo {
    BlockInfo {
        block_number: BlockNumber(block_number),
        block_timestamp: BlockTimestamp(block_timestamp),
        sequencer_address:ContractAddress::try_from(sequencer_address).unwrap(),
        gas_prices,
        use_kzg_da,
    }
}


fn build_chain_info(
    chain_id: &str,
    fee_token_addresses: FeeTokenAddresses,
) -> ChainInfo {
    ChainInfo {
        chain_id:ChainId(chain_id.to_string()),
        fee_token_addresses,
    }
}
