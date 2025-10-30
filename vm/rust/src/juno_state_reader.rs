use std::{
    collections::HashMap,
    ffi::{c_int, c_uchar, c_void},
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
    ClassInfo as BlockifierClassInfo, ContractClass, SierraVersion, EntryPointType
};
use starknet_api::core::{ClassHash, CompiledClassHash, ContractAddress, Nonce};
use starknet_api::state::StorageKey;
use starknet_types_core::felt::Felt;
use crate::BlockInfo;
use starknet_api::deprecated_contract_class::{ContractClass as DeprecatedClassDefinition, EntryPointV0};
mod compiled_class_pb {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/src/protobuf/compiled_class.pb.rs"));
}
use prost::Message;
use num_traits::Num;

type StarkFelt = Felt;

extern "C" {
    fn JunoStateGetStorageAt(
        reader_handle: usize,
        contract_address: *const c_uchar,
        storage_location: *const c_uchar,
        buffer: *mut c_uchar,
    ) -> c_int;
    fn JunoStateGetNonceAt(
        reader_handle: usize,
        contract_address: *const c_uchar,
        buffer: *mut c_uchar,
    ) -> c_int;
    fn JunoStateGetClassHashAt(
        reader_handle: usize,
        contract_address: *const c_uchar,
        buffer: *mut c_uchar,
    ) -> c_int;
    fn JunoStateGetCompiledClass(
        reader_handle: usize,
        class_hash: *const c_uchar,
        out_len: *mut usize,
    )
        -> *const c_uchar;
    fn JunoFree(p: *const c_void) -> ();
}

struct CachedRunnableCompiledClass {
    pub definition: RunnableCompiledClass,
    pub cached_on_height: u64,
}

static CLASS_CACHE: Lazy<Mutex<SizedCache<ClassHash, CachedRunnableCompiledClass>>> =
    Lazy::new(|| Mutex::new(SizedCache::with_size(128)));

pub enum BlockHeight {
    Height(u64),
    Pending,
}

impl BlockHeight {
    pub fn is_after(&self, target: u64) -> bool {
        match self {
            BlockHeight::Height(height) => *height > target,
            BlockHeight::Pending => true,
        }
    }

    pub fn from_block_info(block_info: &BlockInfo) -> Self {
        if block_info.is_pending == 1 {
            Self::Pending
        } else {
            Self::Height(block_info.block_number)
        }
    }
}

pub struct JunoStateReader {
    pub handle: usize, // uintptr_t equivalent
    pub height: BlockHeight,
}

impl JunoStateReader {
    pub fn new(handle: usize, height: BlockHeight) -> Self {
        Self { handle, height }
    }
}

impl StateReader for JunoStateReader {
    fn get_storage_at(
        &self,
        contract_address: ContractAddress,
        key: StorageKey,
    ) -> StateResult<StarkFelt> {
        let addr = felt_to_byte_array(contract_address.0.key());
        let storage_key = felt_to_byte_array(key.0.key());
        let mut buffer: [u8; 32] = [0; 32];
        let wrote = unsafe {
            JunoStateGetStorageAt(
                self.handle,
                addr.as_ptr(),
                storage_key.as_ptr(),
                buffer.as_mut_ptr(),
            )
        };
        if wrote == 0 {
            Err(StateError::StateReadError(format!(
                "failed to read location {} at address {}",
                key.0.key(),
                contract_address.0.key()
            )))
        } else {
            assert!(wrote == 32, "Juno didn't write 32 bytes");
            Ok(StarkFelt::from_bytes_be(&buffer))
        }
    }

    /// Returns the nonce of the given contract instance.
    /// Default: 0 for an uninitialized contract address.
    fn get_nonce_at(&self, contract_address: ContractAddress) -> StateResult<Nonce> {
        let addr = felt_to_byte_array(contract_address.0.key());
        let mut buffer: [u8; 32] = [0; 32];
        let wrote = unsafe { JunoStateGetNonceAt(self.handle, addr.as_ptr(), buffer.as_mut_ptr()) };
        if wrote == 0 {
            Err(StateError::StateReadError(format!(
                "failed to read nonce of address {}",
                contract_address.0.key()
            )))
        } else {
            assert!(wrote == 32, "Juno didn't write 32 bytes");
            Ok(Nonce(StarkFelt::from_bytes_be(&buffer)))
        }
    }

    /// Returns the class hash of the contract class at the given contract instance.
    /// Default: 0 (uninitialized class hash) for an uninitialized contract address.
    fn get_class_hash_at(&self, contract_address: ContractAddress) -> StateResult<ClassHash> {
        let addr = felt_to_byte_array(contract_address.0.key());
        let mut buffer: [u8; 32] = [0; 32];
        let wrote =
            unsafe { JunoStateGetClassHashAt(self.handle, addr.as_ptr(), buffer.as_mut_ptr()) };
        if wrote == 0 {
            Err(StateError::StateReadError(format!(
                "failed to read class hash of address {}",
                contract_address.0.key()
            )))
        } else {
            assert!(wrote == 32, "Juno didn't write 32 bytes");
            Ok(ClassHash(StarkFelt::from_bytes_be(&buffer)))
        }
    }

    /// Returns the contract class of the given class hash.
    fn get_compiled_class(&self, class_hash: ClassHash) -> StateResult<RunnableCompiledClass> {
        if let Some(cached_class) = CLASS_CACHE.lock().unwrap().cache_get(&class_hash) {
            // skip the cache if it comes from a height higher than ours. Class might be undefined on the height
            // that we are reading from right now.
            //
            // About the changes in the second attempt at making class cache behave as expected;
            //
            // The initial assumption here was that `self.height` uniquely identifies and strictly orders the underlying state
            // instances. The first assumption doesn't necessarily hold, because we can pass different state instaces with the
            // same height. This most commonly happens with call/estimate/simulate and trace flows. Trace flow calls the VM
            // for block number N with the state at the beginning of the block, while call/estimate/simulate flows call the VM
            // with the same block number but with the state at the end of that block. That is why, we cannot use classes from cache
            // if they are cached on the same height that we are executing on. Because they might be cached using a state instance that
            // is in the future compared to the state that we are currently executing on, even tho they have the same height.
            if self.height.is_after(cached_class.cached_on_height) {
                return Ok(cached_class.definition.clone());
            }
        }

        let class_hash_bytes = felt_to_byte_array(&class_hash.0);
        let mut out_len: usize = 0;
        let ptr = unsafe { JunoStateGetCompiledClass(self.handle, class_hash_bytes.as_ptr(), &mut out_len) };
        if ptr.is_null() || out_len == 0 {
            Err(StateError::UndeclaredClassHash(class_hash))
        } else {
            let bytes = unsafe { std::slice::from_raw_parts(ptr, out_len) };
            unsafe { JunoFree(ptr as *const c_void) };
            let class_info_res = class_info_from_pb(bytes);

            match class_info_res {
                Ok(class) => {
                    let runnable_compiled_class =
                        RunnableCompiledClass::try_from(class.contract_class).unwrap();
                    if let BlockHeight::Height(height) = self.height {
                        CLASS_CACHE.lock().unwrap().cache_set(
                            class_hash,
                            CachedRunnableCompiledClass {
                                // This clone is cheap, it is just a reference copy in the underlying
                                // RunnableCompiledClass implementation
                                definition: runnable_compiled_class.clone(),
                                cached_on_height: height,
                            },
                        );
                    }
                    Ok(runnable_compiled_class)
                }
                Err(e) => Err(StateError::StateReadError(format!(
                    "parsing JSON string for class hash {}: {}",
                    class_hash.0, e
                ))),
            }
        }
    }

    /// Returns the compiled class hash of the given class hash.
    fn get_compiled_class_hash(&self, _class_hash: ClassHash) -> StateResult<CompiledClassHash> {
        unimplemented!()
    }
}

pub fn felt_to_byte_array(felt: &StarkFelt) -> [u8; 32] {
    felt.to_bytes_be()
}

pub fn ptr_to_felt(bytes: *const c_uchar) -> StarkFelt {
    let slice = unsafe { slice::from_raw_parts(bytes, 32) };
    StarkFelt::from_bytes_be_slice(slice)
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

pub fn class_info_from_pb(mut bytes: &[u8]) -> Result<BlockifierClassInfo, String> {
    let class_info = compiled_class_pb::CompiledClass::decode(&mut bytes)
        .map_err(|e| e.to_string())?;
    let sierra_version: SierraVersion;
    let sierra_len: usize;
    let abi_len: usize;

    let class: ContractClass = match class_info.class {
        Some(compiled_class_pb::compiled_class::Class::Deprecated(dep)) => {
            sierra_version = SierraVersion::DEPRECATED;
            sierra_len = 0;
            abi_len = 0;
            match deprecated_class_from_pb(&dep) {
                Ok(class) => class,
                Err(err) => return Err(format!("failed parsing deprecated class: {:?}", err)),
            }
        }
        Some(compiled_class_pb::compiled_class::Class::Casm(casm)) => {
            sierra_version = SierraVersion::from_str(&class_info.sierra_version)
                .map_err(|err| format!("failed parsing sierra version: {:?}", err))?;
            sierra_len = class_info.sierra_program_length as usize;
            abi_len = class_info.abi_length as usize;
            match casm_definition_from_pb(&casm, sierra_version.clone()) {
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
    let class: DeprecatedClassDefinition =
        serde_json::from_str(&definition)?;

    Ok(starknet_api::contract_class::ContractClass::V0(class))
}

fn deprecated_class_from_pb(d: &compiled_class_pb::DeprecatedCairoClass) -> anyhow::Result<starknet_api::contract_class::ContractClass> {
    let to_ep = |v: &Vec<compiled_class_pb::DeprecatedEntryPoint>| -> Vec<EntryPointV0> {
        v.iter().map(|e| EntryPointV0 {
            selector: starknet_api::core::EntryPointSelector(
                felt_from_pb(e.selector.as_ref().unwrap()),
            ),
            offset: starknet_api::deprecated_contract_class::EntryPointOffset(
                pb_felt_to_usize(e.offset.as_ref().unwrap()),
            ),
        }).collect()
    };

    let abi: Option<Vec<starknet_api::deprecated_contract_class::ContractClassAbiEntry>> = serde_json::from_slice(&d.abi_json).ok();
    let mut entry_points_by_type: HashMap<EntryPointType, Vec<EntryPointV0>> = HashMap::new();
    entry_points_by_type.insert(EntryPointType::External, to_ep(&d.externals));
    entry_points_by_type.insert(EntryPointType::L1Handler, to_ep(&d.l1_handlers));
    entry_points_by_type.insert(EntryPointType::Constructor, to_ep(&d.constructors));
    let program: starknet_api::deprecated_contract_class::Program = serde_json::from_slice(&d.program_b64)?; // TODO: HMMMMMM
    Ok(starknet_api::contract_class::ContractClass::V0(
        DeprecatedClassDefinition {
            abi: abi,
            entry_points_by_type: entry_points_by_type,
            program: program,
        }
    ))
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

fn casm_definition_from_pb(
    c: &compiled_class_pb::CasmClass,
    sierra_version: starknet_api::contract_class::SierraVersion,
) -> anyhow::Result<starknet_api::contract_class::ContractClass> {
    use cairo_lang_starknet_classes::casm_contract_class::{CasmContractClass, CasmContractEntryPoint, CasmContractEntryPoints};

    let bytecode = c
        .bytecode
        .iter()
        .map(|f| cairo_lang_utils::bigint::BigUintAsHex::from(biguint_from_pb(f)))
        .collect::<Vec<_>>();

    let to_entries = |src: &Vec<compiled_class_pb::CompiledEntryPoint>| -> Vec<CasmContractEntryPoint> {
        src.iter().map(|e| CasmContractEntryPoint {
            selector: biguint_from_pb(e.selector.as_ref().unwrap()),
            offset: e.offset as usize,
            builtins: e.builtins.clone(),
        }).collect()
    };

    let entry_points = CasmContractEntryPoints {
        external: to_entries(&c.external),
        l1_handler: to_entries(&c.l1_handler),
        constructor: to_entries(&c.constructor),
    };

    let hints: Vec<(usize, Vec<cairo_lang_casm::hints::Hint>)> = serde_json::from_slice(&c.hints_json).unwrap_or_default();
    let pythonic_hints: Vec<(usize, Vec<String>)> = serde_json::from_slice(&c.pythonic_hints_json).unwrap_or_default();
    let bytecode_segment_lengths = serde_json::from_slice(&c.bytecode_segment_lengths).ok();

    let prime = num_bigint::BigUint::from_str_radix(&c.prime.strip_prefix("0x").unwrap_or(&c.prime), 16)
        .map_err(|e| anyhow::anyhow!("failed to parse prime: {}", e))?;
    
    Ok(starknet_api::contract_class::ContractClass::V1((
        CasmContractClass {
            prime: prime,
            bytecode: bytecode,
            hints: hints,
            pythonic_hints: Some(pythonic_hints),
            compiler_version: c.compiler_version.clone(),
            bytecode_segment_lengths: bytecode_segment_lengths,
            entry_points_by_type: entry_points,
        },
        sierra_version,
    )))
}

fn felt_from_pb(f: &compiled_class_pb::Felt) -> StarkFelt {
    use StarkFelt;
    let mut be = [0u8; 32];
    let src = f.be32.as_slice();
    let n = src.len().min(32);
    be[32 - n..].copy_from_slice(&src[src.len() - n..]);
    StarkFelt::from_bytes_be(&be)
}

fn biguint_from_pb(f: &compiled_class_pb::Felt) -> num_bigint::BigUint {
    num_bigint::BigUint::from_bytes_be(f.be32.as_slice())
}

fn pb_felt_to_usize(f: &compiled_class_pb::Felt) -> usize {
    let src = f.be32.as_slice();
    let mut arr = [0u8; 8];
    let n = src.len().min(8);
    arr[8 - n..].copy_from_slice(&src[src.len() - n..]);
    u64::from_be_bytes(arr) as usize
}
