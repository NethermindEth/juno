use std::{
    ffi::{c_char, c_uchar, c_void, CStr},
    slice,
    sync::Mutex,
};

use blockifier::{
    execution::contract_class::{ContractClassV0, ContractClassV1, ContractClass},
    state::{
        state_api::{StateReader, StateResult, State},
        errors::StateError,
        cached_state::CommitmentStateDiff,
    }
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

    fn JunoStateSetStorage(
        reader_handle: usize,
        address: *const c_uchar,
        key: *const c_uchar,
        value: *const c_uchar,
    ) -> *const c_char;
    fn JunoStateIncrementNonce(
        reader_handle: usize,
        address: *const c_uchar,
    ) -> *const c_char;
    fn JunoStateSetClassHashAt(
        reader_handle: usize,
        address: *const c_uchar,
        class_hash: *const c_uchar,
    ) -> *const c_char;
    fn JunoStateSetContractClass(
        reader_handle: usize,
        class_hash: *const c_uchar,
    ) -> *const c_char;
    fn JunoStateSetCompiledClassHash(
        reader_handle: usize,
        class_hash: *const c_uchar,
        compiled_class_hash: *const c_uchar,
    ) -> *const c_char;
}

struct CachedContractClass {
    pub definition: ContractClass,
    pub cached_on_height: u64,
}

static CLASS_CACHE: Lazy<Mutex<SizedCache<ClassHash, CachedContractClass>>> =
    Lazy::new(|| Mutex::new(SizedCache::with_size(128)));

pub struct JunoState {
    pub handle: usize, // uintptr_t equivalent
    pub height: u64,
}

impl JunoState {
    pub fn new(handle: usize, height: u64) -> Self {
        Self { handle, height }
    }
}

impl StateReader for JunoState {
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
            if cached_class.cached_on_height <= self.height {
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

impl State for JunoState {
    /// Sets the storage value under the given key in the given contract instance.
    fn set_storage_at(
        &mut self,
        contract_address: ContractAddress,
        key: StorageKey,
        value: StarkFelt,
        ) {
        let addr = felt_to_byte_array(contract_address.0.key());
        let storage_key = felt_to_byte_array(key.0.key());
        let storage_value = felt_to_byte_array(&value);
        let result = state_read_err(unsafe { JunoStateSetStorage(self.handle, addr.as_ptr(), storage_key.as_ptr(), storage_value.as_ptr()) });
        if result.is_err() {
            panic!("{}", result.unwrap_err());
        }
    }

    /// Increments the nonce of the given contract instance.
    fn increment_nonce(&mut self, contract_address: ContractAddress) -> StateResult<()> {
        let addr = felt_to_byte_array(contract_address.0.key());
        state_read_err(unsafe { JunoStateIncrementNonce(self.handle, addr.as_ptr()) })
    }

    /// Allocates the given address to the given class hash.
    /// Raises an exception if the address is already assigned;
    /// meaning: this is a write once action.
    fn set_class_hash_at(
        &mut self,
        contract_address: ContractAddress,
        class_hash: ClassHash,
        ) -> StateResult<()> {
        let addr = felt_to_byte_array(contract_address.0.key());
        let class_hash = felt_to_byte_array(&class_hash.0);
        state_read_err(unsafe { JunoStateSetClassHashAt(self.handle, addr.as_ptr(), class_hash.as_ptr()) })
    }

    /// Sets the given contract class under the given class hash.
    fn set_contract_class(
        &mut self,
        class_hash: &ClassHash,
        _contract_class: ContractClass,
        ) -> StateResult<()> {
        let class_hash = felt_to_byte_array(&class_hash.0);
        state_read_err(unsafe { JunoStateSetContractClass(self.handle, class_hash.as_ptr()) })
    }

    /// Sets the given compiled class hash under the given class hash.
    fn set_compiled_class_hash(
        &mut self,
        class_hash: ClassHash,
        compiled_class_hash: CompiledClassHash,
        ) -> StateResult<()> {
        let class_hash_bytes = felt_to_byte_array(&class_hash.0);
        let compiled_class_hash_bytes = felt_to_byte_array(&compiled_class_hash.0);
        state_read_err(unsafe { JunoStateSetCompiledClassHash(self.handle, class_hash_bytes.as_ptr(), compiled_class_hash_bytes.as_ptr()) })
    }

    fn to_state_diff(&mut self) -> CommitmentStateDiff {
        unimplemented!()
    }
}

fn state_read_err(err_ptr: *const c_char) -> StateResult<()> {
        if err_ptr.is_null() {
            return Ok(())
        }
        let err_string = unsafe { CStr::from_ptr(err_ptr) }.to_str().unwrap();
        unsafe { JunoFree(err_ptr as *const c_void) };
        Err(StateError::StateReadError(err_string.to_string()))
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
