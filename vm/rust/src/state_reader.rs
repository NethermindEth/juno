mod ffi;
mod state_reader;

use std::{
    ffi::{c_char, c_int, c_uchar, c_void, CStr},
    slice,
    str::FromStr,
    sync::Mutex,
};

use blockifier::execution::contract_class::RunnableCompiledClass;
use blockifier::state::errors::StateError;
use blockifier::state::state_api::{StateReader, StateResult};
use cached::{Cached, SizedCache};
use cairo_lang_starknet_classes::casm_contract_class::CasmContractClass;
use once_cell::sync::Lazy;
use serde::Deserialize;
use starknet_api::contract_class::{
    ClassInfo as BlockifierClassInfo, ContractClass, SierraVersion,
};
use starknet_api::core::{ClassHash, CompiledClassHash, ContractAddress, Nonce};
use starknet_api::state::StorageKey;
use starknet_types_core::felt::Felt;

use crate::ffi_entrypoint::BlockInfo;

pub fn felt_to_byte_array(felt: &Felt) -> [u8; 32] {
    felt.to_bytes_be()
}

pub fn ptr_to_felt(bytes: *const c_uchar) -> Felt {
    let slice = unsafe { slice::from_raw_parts(bytes, 32) };
    Felt::from_bytes_be_slice(slice)
}

#[derive(Deserialize)]
pub struct ClassInfo {
    cairo_version: usize,
    contract_class: Box<serde_json::value::RawValue>,
    sierra_program_length: usize,
    abi_length: usize,
    sierra_version: String,
}

pub fn class_info_from_json_str(raw_json: &str) -> Result<BlockifierClassInfo, String> {
    let class_info: ClassInfo = serde_json::from_str(raw_json)
        .map_err(|err| format!("failed parsing class info: {:?}", err))?;

    let class_def = class_info.contract_class.get();
    let sierra_version: SierraVersion;
    let sierra_len;
    let abi_len;
    let class: ContractClass = match class_info.cairo_version {
        0 => {
            sierra_version = SierraVersion::DEPRECATED;
            sierra_len = 0;
            abi_len = 0;
            match parse_deprecated_class_definition(class_def.to_string()) {
                Ok(class) => class,
                Err(err) => return Err(format!("failed parsing deprecated class: {:?}", err)),
            }
        }
        1 => {
            sierra_version = SierraVersion::from_str(&class_info.sierra_version)
                .map_err(|err| format!("failed parsing sierra version: {:?}", err))?;
            sierra_len = class_info.sierra_program_length;
            abi_len = class_info.abi_length;
            match parse_casm_definition(class_def.to_string(), sierra_version.clone()) {
                Ok(class) => class,
                Err(err) => return Err(format!("failed parsing casm class: {:?}", err)),
            }
        }
        _ => {
            return Err(format!(
                "unsupported class version: {}",
                class_info.cairo_version
            ))
        }
    };

    BlockifierClassInfo::new(&class, sierra_len, abi_len, sierra_version)
        .map_err(|err| format!("failed creating BlockifierClassInfo: {:?}", err))
}

fn parse_deprecated_class_definition(
    definition: String,
) -> anyhow::Result<starknet_api::contract_class::ContractClass> {
    let class: starknet_api::deprecated_contract_class::ContractClass =
        serde_json::from_str(&definition)?;

    Ok(starknet_api::contract_class::ContractClass::V0(class))
}

fn parse_casm_definition(
    casm_definition: String,
    sierra_version: starknet_api::contract_class::SierraVersion,
) -> anyhow::Result<starknet_api::contract_class::ContractClass> {
    let class: CasmContractClass = serde_json::from_str(&casm_definition)?;

    Ok(starknet_api::contract_class::ContractClass::V1((
        class,
        sierra_version,
    )))
}
