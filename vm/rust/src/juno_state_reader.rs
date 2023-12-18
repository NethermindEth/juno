use std::{
    ffi::{c_char, c_uchar, c_void, CStr},
    slice,
    sync::Mutex,
};

use blockifier::execution::contract_class::ContractClass;
use blockifier::state::errors::StateError;
use blockifier::{
    execution::contract_class::{ContractClassV0, ContractClassV1},
    state::state_api::{StateReader, StateResult},
};
use cached::{Cached, SizedCache};
use once_cell::sync::Lazy;
use starknet_api::core::{ClassHash, CompiledClassHash, ContractAddress, Nonce};
use starknet_api::hash::StarkFelt;
use starknet_api::state::StorageKey;

extern "C" {
    fn JunoFree(ptr: *const c_void);

    fn JunoStateGetStorageAt(
        reader_handle: usize,
        contract_address: *const c_uchar,
        storage_location: *const c_uchar,
    ) -> *const c_uchar;
    fn JunoStateGetNonceAt(
        reader_handle: usize,
        contract_address: *const c_uchar,
    ) -> *const c_uchar;
    fn JunoStateGetClassHashAt(
        reader_handle: usize,
        contract_address: *const c_uchar,
    ) -> *const c_uchar;
    fn JunoStateGetCompiledClass(reader_handle: usize, class_hash: *const c_uchar)
        -> *const c_char;
}

struct CachedContractClass {
    pub definition: ContractClass,
    pub cached_on_height: u64,
}

static CLASS_CACHE: Lazy<Mutex<SizedCache<ClassHash, CachedContractClass>>> =
    Lazy::new(|| Mutex::new(SizedCache::with_size(128)));

pub struct JunoStateReader {
    pub handle: usize, // uintptr_t equivalent
    pub height: u64,
}

impl JunoStateReader {
    pub fn new(handle: usize, height: u64) -> Self {
        Self { handle, height }
    }
}

impl StateReader for JunoStateReader {
    fn get_storage_at(
        &mut self,
        contract_address: ContractAddress,
        key: StorageKey,
    ) -> StateResult<StarkFelt> {
        let addr = felt_to_byte_array(contract_address.0.key());
        let storage_key = felt_to_byte_array(key.0.key());
        let ptr =
            unsafe { JunoStateGetStorageAt(self.handle, addr.as_ptr(), storage_key.as_ptr()) };
        if ptr.is_null() {
            Err(StateError::StateReadError(format!(
                "failed to read location {} at address {}",
                key.0.key(),
                contract_address.0.key()
            )))
        } else {
            let felt_val = ptr_to_felt(ptr);
            unsafe { JunoFree(ptr as *const c_void) };

            Ok(felt_val)
        }
    }

    /// Returns the nonce of the given contract instance.
    /// Default: 0 for an uninitialized contract address.
    fn get_nonce_at(&mut self, contract_address: ContractAddress) -> StateResult<Nonce> {
        let addr = felt_to_byte_array(contract_address.0.key());
        let ptr = unsafe { JunoStateGetNonceAt(self.handle, addr.as_ptr()) };
        if ptr.is_null() {
            Err(StateError::StateReadError(format!(
                "failed to read nonce of address {}",
                contract_address.0.key()
            )))
        } else {
            let felt_val = ptr_to_felt(ptr);
            unsafe { JunoFree(ptr as *const c_void) };
            Ok(Nonce(felt_val))
        }
    }

    /// Returns the class hash of the contract class at the given contract instance.
    /// Default: 0 (uninitialized class hash) for an uninitialized contract address.
    fn get_class_hash_at(&mut self, contract_address: ContractAddress) -> StateResult<ClassHash> {
        let addr = felt_to_byte_array(contract_address.0.key());
        let ptr = unsafe { JunoStateGetClassHashAt(self.handle, addr.as_ptr()) };
        if ptr.is_null() {
            Err(StateError::StateReadError(format!(
                "failed to read class hash of address {}",
                contract_address.0.key()
            )))
        } else {
            let felt_val = ptr_to_felt(ptr);
            unsafe { JunoFree(ptr as *const c_void) };

            Ok(ClassHash(felt_val))
        }
    }

    /// Returns the contract class of the given class hash.
    fn get_compiled_contract_class(
        &mut self,
        class_hash: &ClassHash,
    ) -> StateResult<ContractClass> {
        if let Some(cached_class) = CLASS_CACHE.lock().unwrap().cache_get(class_hash) {
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
            if cached_class.cached_on_height < self.height {
                return Ok(cached_class.definition.clone());
            }
        }

        let class_hash_bytes = felt_to_byte_array(&class_hash.0);
        let ptr = unsafe { JunoStateGetCompiledClass(self.handle, class_hash_bytes.as_ptr()) };
        if ptr.is_null() {
            Err(StateError::UndeclaredClassHash(*class_hash))
        } else {
            let json_str = unsafe { CStr::from_ptr(ptr) }.to_str().unwrap();
            let contract_class = contract_class_from_json_str(json_str);
            if let Ok(class) = &contract_class {
                CLASS_CACHE.lock().unwrap().cache_set(
                    *class_hash,
                    CachedContractClass {
                        definition: class.clone(),
                        cached_on_height: self.height,
                    },
                );
            }

            unsafe { JunoFree(ptr as *const c_void) };

            contract_class.map_err(|_| {
                StateError::StateReadError(format!(
                    "error parsing JSON string for class hash {}",
                    class_hash.0
                ))
            })
        }
    }

    /// Returns the compiled class hash of the given class hash.
    fn get_compiled_class_hash(
        &mut self,
        _class_hash: ClassHash,
    ) -> StateResult<CompiledClassHash> {
        unimplemented!()
    }
}

pub fn felt_to_byte_array(felt: &StarkFelt) -> [u8; 32] {
    felt.bytes().try_into().expect("StarkFelt not [u8; 32]")
}

pub fn ptr_to_felt(bytes: *const c_uchar) -> StarkFelt {
    let slice = unsafe { slice::from_raw_parts(bytes, 32) };
    StarkFelt::new(slice.try_into().expect("Juno felt not [u8; 32]"))
        .expect("cannot new Starkfelt from Juno bytes")
}

pub fn contract_class_from_json_str(raw_json: &str) -> Result<ContractClass, String> {
    let v0_class = ContractClassV0::try_from_json_string(raw_json);
    if let Ok(class) = v0_class {
        return Ok(class.into());
    }
    let v1_class = ContractClassV1::try_from_json_string(raw_json);
    if let Ok(class) = v1_class {
        Ok(class.into())
    } else {
        Err(format!("not a valid contract class: {}", v1_class.err().unwrap()))
    }
}
