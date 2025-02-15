use std::ffi::{c_uchar, c_ulonglong};

use blockifier::{
    abi::constants::STORED_BLOCK_HASH_BUFFER,
    blockifier::block::pre_process_block,
    bouncer::BouncerConfig,
    context::{BlockContext, ChainInfo, FeeTokenAddresses},
    state::state_api::State,
};
use starknet_api::{
    block::{
        BlockHash, BlockHashAndNumber, BlockInfo as BlockifierBlockInfo, GasPrice, GasPriceVector,
        GasPrices, NonzeroGasPrice,
    },
    core::{ChainId, ContractAddress, PatriciaKey},
    hash::StarkHash,
};
use starknet_types_core::felt::Felt;

use crate::types::{BlockInfo, StarkFelt};
use crate::versioned_constants::get_versioned_constants;

fn felt_to_u128(felt: StarkFelt) -> u128 {
    // todo find Into<u128> trait or similar
    let bytes = felt.to_bytes_be();
    let mut arr = [0u8; 16];
    arr.copy_from_slice(&bytes[16..32]);

    // felts are encoded in big-endian order
    u128::from_be_bytes(arr)
}

// NonzeroGasPrice must be greater than zero to successfully execute transaction.
fn gas_price_from_bytes_bonded(bytes: &[c_uchar; 32]) -> Result<NonzeroGasPrice, anyhow::Error> {
    let u128_val = felt_to_u128(StarkFelt::from_bytes_be(bytes));
    Ok(NonzeroGasPrice::new(GasPrice(if u128_val == 0 {
        1
    } else {
        u128_val
    }))?)
}

pub fn build_block_context(
    state: &mut dyn State,
    block_info: &BlockInfo,
    chain_id_str: &str,
    max_steps: Option<c_ulonglong>,
    _concurrency_mode: bool,
) -> Result<BlockContext, anyhow::Error> {
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
        sequencer_address: ContractAddress(PatriciaKey::try_from(sequencer_addr).unwrap()),
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
    let chain_info = ChainInfo {
        chain_id: ChainId::from(chain_id_str.to_string()),
        fee_token_addresses: FeeTokenAddresses {
            // Both addresses are the same for all networks
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
