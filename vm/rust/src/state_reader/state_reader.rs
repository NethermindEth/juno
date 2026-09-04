use std::{
    ffi::{c_uchar, c_void, CStr},
    slice,
    sync::Mutex,
};

use blockifier::execution::contract_class::RunnableCompiledClass;
use blockifier::state::errors::StateError;
use blockifier::state::state_api::{StateReader, StateResult};
use cached::{Cached, SizedCache};
use once_cell::sync::Lazy;
use starknet_api::contract_class::compiled_class_hash::{HashVersion, HashableCompiledClass};
use starknet_api::core::{ClassHash, CompiledClassHash, ContractAddress, Nonce};
use starknet_api::state::StorageKey;
use starknet_types_core::felt::Felt;

use crate::{
    ffi_entrypoint::BlockInfo,
    ffi_type::class_info::class_info_from_json_str,
    state_reader::ffi::{
        JunoFree, JunoStateGetClassHashAt, JunoStateGetCompiledClass,
        JunoStateGetCompiledClassHash, JunoStateGetCompiledClassHashV2, JunoStateGetNonceAt,
        JunoStateGetStorageAt,
    },
};

struct CachedRunnableCompiledClass {
    pub definition: RunnableCompiledClass,
    pub declared_at: u64,
}

static CLASS_CACHE: Lazy<Mutex<SizedCache<ClassHash, CachedRunnableCompiledClass>>> =
    Lazy::new(|| Mutex::new(SizedCache::with_size(128)));

static CASM_CLASS_HASH_V2_CACHE: Lazy<Mutex<SizedCache<ClassHash, CompiledClassHash>>> =
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
    ) -> StateResult<Felt> {
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
            Ok(Felt::from_bytes_be(&buffer))
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
            Ok(Nonce(Felt::from_bytes_be(&buffer)))
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
            Ok(ClassHash(Felt::from_bytes_be(&buffer)))
        }
    }

    /// Returns the contract class of the given class hash.
    fn get_compiled_class(&self, class_hash: ClassHash) -> StateResult<RunnableCompiledClass> {
        if let Some(cached_class) = CLASS_CACHE.lock().unwrap().cache_get(&class_hash) {
            // A class declared at height D exists in every state from the end of block D on. Trace
            // and call flows can run block N on different states (start vs end of N), so a height
            // equal to D is not a hit.
            if self.height.is_after(cached_class.declared_at) {
                return Ok(cached_class.definition.clone());
            }
        }

        let class_hash_bytes = felt_to_byte_array(&class_hash.0);
        let mut declared_at: u64 = 0;
        let ptr = unsafe {
            JunoStateGetCompiledClass(self.handle, class_hash_bytes.as_ptr(), &mut declared_at)
        };
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
                    // Pending states report declared_at 0 for classes declared in the pending block.
                    if matches!(self.height, BlockHeight::Height(_)) {
                        CLASS_CACHE.lock().unwrap().cache_set(
                            class_hash,
                            CachedRunnableCompiledClass {
                                // This clone is cheap, it is just a reference copy in the underlying
                                // RunnableCompiledClass implementation
                                definition: runnable_compiled_class.clone(),
                                declared_at,
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
    /// Returns CompiledClassHash::default() if no v1_class is found for the given class hash.
    fn get_compiled_class_hash(&self, class_hash: ClassHash) -> StateResult<CompiledClassHash> {
        let class_hash_bytes = felt_to_byte_array(&class_hash.0);
        let mut buffer: [u8; 32] = [0; 32];
        let wrote = unsafe {
            JunoStateGetCompiledClassHash(
                self.handle,
                class_hash_bytes.as_ptr(),
                buffer.as_mut_ptr(),
            )
        };
        if wrote == 0 {
            Ok(CompiledClassHash::default())
        } else {
            assert!(wrote == 32, "Juno didn't write 32 bytes");
            Ok(CompiledClassHash(Felt::from_bytes_be(&buffer)))
        }
    }

    /// Returns the compiled class hash (v2) of the given class hash.
    /// Returns `StateError::MissingCompiledClassHashV2` if no v1_class is found for the given class
    /// hash.
    fn get_compiled_class_hash_v2(
        &self,
        class_hash: ClassHash,
        compiled_class: &RunnableCompiledClass,
    ) -> StateResult<CompiledClassHash> {
        if let Some(cached_hash) = CASM_CLASS_HASH_V2_CACHE
            .lock()
            .unwrap()
            .cache_get(&class_hash)
            .cloned()
        {
            return Ok(cached_hash);
        }

        let class_hash_bytes = felt_to_byte_array(&class_hash.0);
        let mut buffer: [u8; 32] = [0; 32];
        let wrote = unsafe {
            JunoStateGetCompiledClassHashV2(
                self.handle,
                class_hash_bytes.as_ptr(),
                buffer.as_mut_ptr(),
            )
        };

        let casm_hash_v2 = if wrote == 0 {
            // Not found in state, compute from compiled_class
            match compiled_class {
                RunnableCompiledClass::V0(_) => {
                    Err(StateError::MissingCompiledClassHashV2(class_hash))
                }
                RunnableCompiledClass::V1(class) => Ok(class.hash(&HashVersion::V2)),
            }
        } else {
            assert!(wrote == 32, "Juno didn't write 32 bytes");
            Ok(CompiledClassHash(Felt::from_bytes_be(&buffer)))
        };

        if let Ok(casm_hash_v2) = casm_hash_v2 {
            CASM_CLASS_HASH_V2_CACHE
                .lock()
                .unwrap()
                .cache_set(class_hash, casm_hash_v2);
        };

        casm_hash_v2
    }
}

pub fn felt_to_byte_array(felt: &Felt) -> [u8; 32] {
    felt.to_bytes_be()
}

pub fn ptr_to_felt(bytes: *const c_uchar) -> Felt {
    let slice = unsafe { slice::from_raw_parts(bytes, 32) };
    Felt::from_bytes_be_slice(slice)
}
