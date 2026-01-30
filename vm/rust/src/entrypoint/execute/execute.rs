use crate::{
    entrypoint::{
        execute::{
            process::{determine_gas_vector_mode, process_transaction},
            utils::{
                adjust_fee_calculation_result, append_gas_and_fee, append_initial_reads,
                append_receipt, append_trace, parse_json, transaction_from_api,
            },
        },
        utils::build_block_context,
    },
    error::{execution::ExecutionError, juno::JunoError, stack::error_stack_frames_to_json},
    ffi_entrypoint::{BlockInfo, ChainInfo},
    ffi_type::{
        class_info::class_info_from_json_str, initial_reads::InitialReads,
        transaction_receipt::TransactionReceipt, transaction_trace::new_transaction_trace,
    },
    state_reader::{state_reader::BlockHeight, JunoStateReader},
};
use serde::Deserialize;
use std::{
    collections::VecDeque,
    ffi::{c_char, c_uchar},
};

use blockifier::state::cached_state::{CachedState, StateMaps};
use blockifier::transaction::account_transaction::ExecutionFlags as AccountExecutionFlags;
use starknet_api::transaction::{
    fields::Fee, Transaction as StarknetApiTransaction, TransactionHash,
};

pub fn cairo_vm_execute(
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
    return_initial_reads: c_uchar,
) -> Result<(), JunoError> {
    let block_info = unsafe { *block_info_ptr };
    let chain_info = unsafe { *chain_info_ptr };
    let reader = JunoStateReader::new(reader_handle, BlockHeight::from_block_info(&block_info));

    #[derive(Deserialize)]
    pub struct TxnAndQueryBit {
        pub txn: StarknetApiTransaction,
        pub txn_hash: TransactionHash,
        pub query_bit: bool,
    }
    let txns_and_query_bits: Vec<TxnAndQueryBit> = parse_json(txns_json)?;

    let mut classes: VecDeque<Box<serde_json::value::RawValue>> = if !classes_json.is_null() {
        parse_json(classes_json)?
    } else {
        VecDeque::new()
    };

    let mut paid_fees_on_l1: VecDeque<Box<Fee>> = parse_json(paid_fees_on_l1_json)?;

    let mut state = CachedState::new(reader);
    let concurrency_mode = concurrency_mode == 1;
    let block_context =
        build_block_context(&mut state, &block_info, &chain_info, None, concurrency_mode)
            .map_err(JunoError::block_error)?;
    let charge_fee = skip_charge_fee == 0;
    let validate = skip_validate == 0;
    let err_stack = err_stack == 1;
    let err_on_revert = err_on_revert == 1;
    let allow_binary_search = allow_binary_search == 1;
    let is_estimate_fee = is_estimate_fee == 1;
    let return_initial_reads = return_initial_reads == 1;

    let mut writer_buffer = Vec::with_capacity(10_000);
    let mut aggregated_initial_reads: Option<StateMaps> = None;

    for (txn_index, txn_and_query_bit) in txns_and_query_bits.iter().enumerate() {
        let class_info = match txn_and_query_bit.txn.clone() {
            StarknetApiTransaction::Declare(_) => {
                let class_json_str = classes.pop_front().ok_or_else(|| {
                    JunoError::tx_non_execution_error("missing declared class", txn_index)
                })?;

                let class_info = class_info_from_json_str(class_json_str.get())
                    .map_err(|err| JunoError::tx_non_execution_error(err, txn_index))?;

                Some(class_info)
            }
            _ => None,
        };

        let paid_fee_on_l1: Option<Fee> = match txn_and_query_bit.txn.clone() {
            StarknetApiTransaction::L1Handler(_) => {
                let paid_fee_on_l1 = paid_fees_on_l1.pop_front().ok_or_else(|| {
                    JunoError::tx_non_execution_error("missing fee paid on l1", txn_index)
                })?;
                Some(*paid_fee_on_l1)
            }
            _ => None,
        };

        let account_execution_flags = AccountExecutionFlags {
            only_query: txn_and_query_bit.query_bit,
            charge_fee,
            validate,
            // we don't want to be strict with the nonce when not running
            // validation. "strict nonce check" means that if an account
            // has nonce `i`, its next transaction must have nonce `i+1`.
            // When this check is relaxed, the transaction can have any
            // future nonce (i.e. `i + k` with `k > 0`)
            strict_nonce_check: validate,
        };

        let mut txn = transaction_from_api(
            txn_and_query_bit.txn.clone(),
            txn_and_query_bit.txn_hash,
            class_info,
            paid_fee_on_l1,
            account_execution_flags,
        )
        .map_err(|err| JunoError::tx_non_execution_error(err, txn_index))?;

        let is_l1_handler_txn = false;
        let mut txn_state = CachedState::create_transactional(&mut state);
        let gas_vector_computation_mode = determine_gas_vector_mode(&txn);

        let mut tx_execution_info = process_transaction(
            &mut txn,
            &mut txn_state,
            &block_context,
            err_on_revert,
            allow_binary_search,
        )
        .map_err(|e| match e {
            ExecutionError::ExecutionError { error, error_stack } => {
                if err_stack {
                    JunoError::json_error(error_stack_frames_to_json(error_stack), Some(txn_index))
                } else {
                    JunoError::tx_non_execution_error(error, txn_index)
                }
            }
            ExecutionError::Internal(e) | ExecutionError::Custom(e) => {
                JunoError::tx_non_execution_error(e, txn_index)
            }
        })?;

        // we are estimating fee, override actual fee calculation
        if is_estimate_fee && !is_l1_handler_txn {
            adjust_fee_calculation_result(
                &mut tx_execution_info,
                &txn,
                &gas_vector_computation_mode,
                &block_context,
                txn_index,
            )?
        }

        append_gas_and_fee(&tx_execution_info, reader_handle);

        let transaction_receipt = TransactionReceipt {
            gas: tx_execution_info.receipt.gas,
            da_gas: tx_execution_info.receipt.da_gas,
            fee: tx_execution_info.receipt.fee,
        };
        append_receipt(reader_handle, &transaction_receipt, &mut writer_buffer)
            .map_err(|err| JunoError::tx_non_execution_error(err, txn_index))?;

        let trace = new_transaction_trace(
            &txn_and_query_bit.txn,
            tx_execution_info,
            &mut txn_state,
            block_context.versioned_constants(),
            &gas_vector_computation_mode,
        )
        .map_err(|err| {
            JunoError::tx_non_execution_error(
                format!("failed building txn state diff reason: {err:?}"),
                txn_index,
            )
        })?;

        append_trace(reader_handle, &trace, &mut writer_buffer)
            .map_err(|err| JunoError::tx_non_execution_error(err, txn_index))?;

        if return_initial_reads {
            let state_maps = txn_state
                .get_initial_reads()
                .map_err(|err| {
                    JunoError::tx_non_execution_error(
                        format!("failed to get initial reads: {err:?}"),
                        txn_index,
                    )
                })?;
            
            match &mut aggregated_initial_reads {
                Some(aggregated) => {
                    aggregated.storage.extend(state_maps.storage);
                    aggregated.nonces.extend(state_maps.nonces);
                    aggregated.class_hashes.extend(state_maps.class_hashes);
                    aggregated.declared_contracts.extend(state_maps.declared_contracts);
                }
                None => {
                    aggregated_initial_reads = Some(state_maps);
                }
            }
        }

        txn_state.commit();
    }

    if return_initial_reads {
        if let Some(initial_reads) = aggregated_initial_reads {
            let initial_reads_ffi: InitialReads = initial_reads.into();
            append_initial_reads(reader_handle, &initial_reads_ffi, &mut writer_buffer)
                .map_err(|err| JunoError::tx_non_execution_error(err, 0))?;
        }
    }

    Ok(())
}
