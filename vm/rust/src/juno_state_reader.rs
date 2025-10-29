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

use crate::BlockInfo;

type StarkFelt = Felt;

extern "C" {
    fn JunoFree(ptr: *const c_void);

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
        class_hash_be32: *const core::ffi::c_void,
    ) -> JunoBytes;
    fn JunoFree(ptr: *mut core::ffi::c_void);
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
        let ptr = unsafe { JunoStateGetCompiledClass(self.handle, class_hash_bytes.as_ptr()) };
        if ptr.is_null() {
            Err(StateError::UndeclaredClassHash(class_hash))
        } else {
            let json_str = unsafe { CStr::from_ptr(ptr) }.to_str().unwrap();
            let class_info_res = class_info_from_json_str(json_str);

            unsafe { JunoFree(ptr as *const c_void) };

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

pub fn class_info_from_pb(mut bytes: &[u8]) -> anyhow::Result<starknet_api::contract_class::ClassInfo> {
    let env = pb::ClassEnvelope::decode(&mut bytes)?;
    match env.class {
        Some(pb::class_envelope::Class::Casm(c)) if env.cairo_version == 1 => {
            // build CasmContractClass (cairo-lang) then ClassInfo::V1
            let casm = casm_from_pb(&c);                      // same helper as before, now felt_from_pb32
            Ok(starknet_api::contract_class::ClassInfo {
                cairo_version: starknet_api::contract_class::CairoVersion::V1(
                    SierraVersion::from(env.sierra_version),
                ),
                contract_class: starknet_api::contract_class::RunnableClass::Casm(casm),
                abi_length: env.abi_length as usize,
                sierra_program_length: env.sierra_program_length as usize,
            })
        }
        Some(pb::class_envelope::Class::Deprecated(d)) if env.cairo_version == 0 => {
            let dep = deprecated_from_pb(&d);                 // maps to starknet_api::deprecated_contract_class::ContractClass
            Ok(starknet_api::contract_class::ClassInfo {
                cairo_version: starknet_api::contract_class::CairoVersion::V0,
                contract_class: starknet_api::contract_class::RunnableClass::Deprecated(ContractClass::from(dep)),
                abi_length: env.abi_length as usize,
                sierra_program_length: env.sierra_program_length as usize,
            })
        }
        _ => anyhow::bail!("unsupported or missing class variant cairo_version={}", env.cairo_version),
    }
}

fn felt_from_pb(f: &pb::Felt) -> starknet_api::hash::StarkFelt {
    use starknet_api::hash::StarkFelt;
    // expects 32-be big-endian. Prost gives Vec<u8>
    let mut be = [0u8; 32];
    let src = f.be32.as_slice();
    let n = src.len().min(32);
    be[32 - n..].copy_from_slice(&src[src.len() - n..]);
    StarkFelt::new(be).unwrap()
}

// Casm conversion into cairo-lang-starknet-classes::CasmContractClass
fn casm_from_pb(c: &pb::CasmClass) -> cairo_lang_starknet_classes::casm_contract_class::CasmContractClass {
    use cairo_lang_starknet_classes::casm_contract_class::{CasmContractClass, EntryPoint, CasmContractEntryPoints};
    use num_bigint::BigUint;

    let bytecode = c.bytecode.iter().map(felt_from_pb).collect::<Vec<_>>();

    let to_entries = |src: &Vec<pb::CompiledEntryPoint>| -> Vec<EntryPoint> {
        src.iter().map(|e| EntryPoint {
            selector: felt_from_pb(&e.selector),
            offset: e.offset as usize,
            builtin: e.builtins.clone(),
        }).collect()
    };

    let entry_points = CasmContractEntryPoints {
        external: to_entries(&c.external),
        l1_handler: to_entries(&c.l1_handler),
        constructor: to_entries(&c.constructor),
    };

    // Segment tree conversion
    fn seg_from_pb(n: &pb::SegmentLengths) -> cairo_lang_starknet_classes::casm_contract_class::SegmentLengths {
        use cairo_lang_starknet_classes::casm_contract_class::SegmentLengths as S;
        match &n.node {
            Some(pb::segment_lengths::Node::Length(len)) => S { length: *len as usize, children: vec![] },
            Some(pb::segment_lengths::Node::Children(ch)) => S {
                length: 0,
                children: ch.items.iter().map(seg_from_pb).collect(),
            },
            None => S { length: 0, children: vec![] },
        }
    }

    CasmContractClass {
        bytecode,
        hints: String::from_utf8_lossy(&c.hints_json).to_string(),
        pythonic_hints: String::from_utf8_lossy(&c.pythonic_hints_json).to_string(),
        compiler_version: c.compiler_version.clone(),
        bytecode_segment_lengths: seg_from_pb(&c.bytecode_segment_lengths),
        entry_points_by_type: entry_points,
    }
}

// Deprecated conversion into starknet_api::deprecated_contract_class::ContractClass
fn deprecated_from_pb(d: &pb::DeprecatedCairoClass) -> starknet_api::deprecated_contract_class::ContractClass {
    use starknet_api::deprecated_contract_class::{ContractClass, EntryPointType, EntryPoint, ContractEntryPoints};
    use serde_json::Value;

    let to_ep = |v: &Vec<pb::DeprecatedEntryPoint>| -> Vec<EntryPoint> {
        v.iter().map(|e| EntryPoint {
            selector: felt_from_pb(&e.selector),
            offset: felt_from_pb(&e.offset), // matches your Go shape (offset as felt)
        }).collect()
    };

    let abi: Value = serde_json::from_slice(&d.abi_json).unwrap_or(serde_json::Value::Null);

    ContractClass {
        abi: Some(abi),
        entry_points_by_type: ContractEntryPoints {
            external: to_ep(&d.externals),
            l1_handler: to_ep(&d.l1_handlers),
            constructor: to_ep(&d.constructors),
        },
        program: String::from_utf8_lossy(&d.program_b64).to_string(),
    }
}