// todo(rdr): This module exist because refactoring it is still not perfect, the preliminary steps of cairo_vm_call and cairo_vm_execute are mostly the same. They should be abstracted away into their own function for "adapting" from the ffi types to the blockifier types

use std::ffi::{c_char, c_uchar, c_ulonglong, CStr};

use anyhow::Result;
use blockifier::{
    abi::constants::STORED_BLOCK_HASH_BUFFER,
    blockifier::block::pre_process_block,
    blockifier_versioned_constants::VersionedConstants,
    bouncer::BouncerConfig,
    context::{BlockContext, ChainInfo as BlockifierChainInfo, FeeTokenAddresses},
    state::state_api::State,
};
use starknet_api::{
    block::{
        BlockHash, BlockHashAndNumber, BlockInfo as BlockifierBlockInfo, GasPrice, GasPriceVector,
        GasPrices, NonzeroGasPrice, StarknetVersion,
    },
    core::{ChainId, ContractAddress},
};
use starknet_types_core::felt::Felt;

use crate::{
    ffi_entrypoint::{BlockInfo, ChainInfo},
    versioned_constants::CUSTOM_VERSIONED_CONSTANTS,
};

fn felt_to_u128(felt: Felt) -> u128 {
    // todo find Into<u128> trait or similar
    let bytes = felt.to_bytes_be();
    let mut arr = [0u8; 16];
    arr.copy_from_slice(&bytes[16..32]);

    // felts are encoded in big-endian order
    u128::from_be_bytes(arr)
}

fn gas_price_from_bytes_bonded(bytes: &[c_uchar; 32]) -> Result<NonzeroGasPrice, anyhow::Error> {
    let u128_val = felt_to_u128(Felt::from_bytes_be(bytes));
    Ok(NonzeroGasPrice::new(GasPrice(if u128_val == 0 {
        1
    } else {
        u128_val
    }))?)
}

#[allow(static_mut_refs)]
fn get_versioned_constants(version: *const c_char) -> VersionedConstants {
    let starknet_version = unsafe { CStr::from_ptr(version) }
        .to_str()
        .ok()
        .and_then(|version_str| StarknetVersion::try_from(version_str).ok());

    if let (Some(custom_constants), Some(version)) =
        (unsafe { &CUSTOM_VERSIONED_CONSTANTS }, starknet_version)
    {
        if let Some(constants) = custom_constants.0.get(&version) {
            return constants.clone();
        }
    }

    starknet_version
        .and_then(|version| VersionedConstants::get(&version).ok())
        .unwrap_or(VersionedConstants::latest_constants())
        .to_owned()
}

pub fn build_block_context(
    state: &mut dyn State,
    block_info: &BlockInfo,
    chain_info: &ChainInfo,
    max_steps: Option<c_ulonglong>,
    _concurrency_mode: bool,
) -> Result<BlockContext> {
    let sequencer_addr = Felt::from_bytes_be(&block_info.sequencer_address);
    let l1_gas_price_eth = gas_price_from_bytes_bonded(&block_info.l1_gas_price_wei)?;
    let l1_gas_price_strk = gas_price_from_bytes_bonded(&block_info.l1_gas_price_fri)?;
    let l1_data_gas_price_eth = gas_price_from_bytes_bonded(&block_info.l1_data_gas_price_wei)?;
    let l1_data_gas_price_strk = gas_price_from_bytes_bonded(&block_info.l1_data_gas_price_fri)?;
    let l2_gas_price_eth = gas_price_from_bytes_bonded(&block_info.l2_gas_price_wei)?;
    let l2_gas_price_strk = gas_price_from_bytes_bonded(&block_info.l2_gas_price_fri)?;

    let mut old_block_number_and_hash: Option<BlockHashAndNumber> = None;
    // STORED_BLOCK_HASH_BUFFER const is 10 for now
    if block_info.block_number >= STORED_BLOCK_HASH_BUFFER {
        old_block_number_and_hash = Some(BlockHashAndNumber {
            number: starknet_api::block::BlockNumber(
                block_info.block_number - STORED_BLOCK_HASH_BUFFER,
            ),
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
        sequencer_address: ContractAddress::try_from(sequencer_addr)?,
        gas_prices: GasPrices {
            eth_gas_prices: GasPriceVector {
                l1_gas_price: l1_gas_price_eth,
                l1_data_gas_price: l1_data_gas_price_eth,
                l2_gas_price: l2_gas_price_eth,
            },
            strk_gas_prices: GasPriceVector {
                l1_gas_price: l1_gas_price_strk,
                l1_data_gas_price: l1_data_gas_price_strk,
                l2_gas_price: l2_gas_price_strk,
            },
        },
        use_kzg_da: block_info.use_blob_data == 1,
    };
    let chain_id_str = unsafe { CStr::from_ptr(chain_info.chain_id) }.to_str()?;
    let eth_fee_token_felt = Felt::from_bytes_be(&chain_info.eth_fee_token_address);
    let strk_fee_token_felt = Felt::from_bytes_be(&chain_info.strk_fee_token_address);

    let chain_info = BlockifierChainInfo {
        chain_id: ChainId::from(chain_id_str.to_string()),
        fee_token_addresses: FeeTokenAddresses {
            eth_fee_token_address: ContractAddress::try_from(eth_fee_token_felt)?,
            strk_fee_token_address: ContractAddress::try_from(strk_fee_token_felt)?,
        },
        // TODO(Ege): make this configurable
        is_l3: false,
    };

    pre_process_block(
        state,
        old_block_number_and_hash,
        block_info.block_number,
        constants.os_constants.as_ref(),
    )?;

    Ok(BlockContext::new(
        block_info,
        chain_info,
        constants,
        BouncerConfig::max(),
    ))
}
