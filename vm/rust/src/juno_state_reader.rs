use std::collections::{HashMap, HashSet};
use std::{
    ffi::{c_char, c_int, c_uchar, c_void, CStr},
    slice,
    sync::Mutex,
};

use blockifier::execution::contract_class::ContractClass;
use blockifier::state::errors::StateError;
use blockifier::state::state_api::UpdatableState;
use blockifier::{
    execution::contract_class::{
        ClassInfo as BlockifierClassInfo, ContractClassV0, ContractClassV1,
    },
    state::cached_state::{ContractClassMapping, StateMaps},
    state::state_api::State,
    state::state_api::{StateReader, StateResult},
};
use cached::{Cached, SizedCache};
use once_cell::sync::Lazy;
use serde::Deserialize;
use starknet_api::core::{ClassHash, CompiledClassHash, ContractAddress, Nonce};
use starknet_api::state::StorageKey;
use starknet_types_core::felt::Felt;

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
    fn JunoStateGetCompiledClass(reader_handle: usize, class_hash: *const c_uchar)
        -> *const c_char;

    fn JunoStateSetStorage(
        reader_handle: usize,
        address: *const c_uchar,
        key: *const c_uchar,
        value: *const c_uchar,
    ) -> *const c_char;
    fn JunoStateIncrementNonce(reader_handle: usize, address: *const c_uchar) -> *const c_char;
    fn JunoStateSetClassHashAt(
        reader_handle: usize,
        address: *const c_uchar,
        class_hash: *const c_uchar,
    ) -> *const c_char;
    fn JunoStateSetContractClass(reader_handle: usize, class_hash: *const c_uchar)
        -> *const c_char;
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

pub struct JunoStateReader {
    pub handle: usize, // uintptr_t equivalent
    pub height: u64,
    visited_pcs: HashMap<ClassHash, HashSet<usize>>,
}

impl JunoStateReader {
    pub fn new(handle: usize, height: u64) -> Self {
        Self {
            handle,
            height,
            visited_pcs: HashMap::default(),
        }
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
    fn get_compiled_contract_class(&self, class_hash: ClassHash) -> StateResult<ContractClass> {
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
            if cached_class.cached_on_height < self.height {
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
            if let Ok(class_info) = &class_info_res {
                CLASS_CACHE.lock().unwrap().cache_set(
                    class_hash,
                    CachedContractClass {
                        definition: class_info.contract_class(),
                        cached_on_height: self.height,
                    },
                );
            }

            unsafe { JunoFree(ptr as *const c_void) };

            class_info_res.map(|ci| ci.contract_class()).map_err(|err| {
                StateError::StateReadError(format!(
                    "parsing JSON string for class hash {}: {}",
                    class_hash.0, err
                ))
            })
        }
    }

    /// Returns the compiled class hash of the given class hash.
    fn get_compiled_class_hash(&self, _class_hash: ClassHash) -> StateResult<CompiledClassHash> {
        unimplemented!()
    }
}

impl UpdatableState for JunoStateReader {
    fn apply_writes(
        &mut self,
        _writes: &StateMaps,
        _class_hash_to_class: &ContractClassMapping,
        _visited_pcs: &HashMap<ClassHash, HashSet<usize>>,
    ) {
        unimplemented!()
    }
}

impl State for JunoStateReader {
    /// Sets the storage value under the given key in the given contract instance.
    fn set_storage_at(
        &mut self,
        contract_address: ContractAddress,
        key: StorageKey,
        value: StarkFelt,
    ) -> Result<(), blockifier::state::errors::StateError> {
        let addr = felt_to_byte_array(contract_address.0.key());
        let storage_key = felt_to_byte_array(key.0.key());
        let storage_value = felt_to_byte_array(&value);
        let result = state_read_err(unsafe {
            JunoStateSetStorage(
                self.handle,
                addr.as_ptr(),
                storage_key.as_ptr(),
                storage_value.as_ptr(),
            )
        });
        result
    }

    fn add_visited_pcs(&mut self, class_hash: ClassHash, pcs: &HashSet<usize>) {
        self.visited_pcs.entry(class_hash).or_default().extend(pcs);
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
        state_read_err(unsafe {
            JunoStateSetClassHashAt(self.handle, addr.as_ptr(), class_hash.as_ptr())
        })
    }

    /// Sets the given contract class under the given class hash.
    fn set_contract_class(
        &mut self,
        class_hash: ClassHash,
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
        state_read_err(unsafe {
            JunoStateSetCompiledClassHash(
                self.handle,
                class_hash_bytes.as_ptr(),
                compiled_class_hash_bytes.as_ptr(),
            )
        })
    }
}

fn state_read_err(err_ptr: *const c_char) -> StateResult<()> {
    if err_ptr.is_null() {
        return Ok(());
    }
    let err_string = unsafe { CStr::from_ptr(err_ptr) }.to_str().unwrap();
    unsafe { JunoFree(err_ptr as *const c_void) };
    Err(StateError::StateReadError(err_string.to_string()))
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
    contract_class: Box<serde_json::value::RawValue>,
    sierra_program_length: usize,
    abi_length: usize,
}

pub fn class_info_from_json_str(raw_json: &str) -> Result<BlockifierClassInfo, String> {
    let class_info: ClassInfo = serde_json::from_str(raw_json)
        .map_err(|err| format!("failed parsing class info: {:?}", err))?;
    let class_def = class_info.contract_class.to_string();
    let mut sierra_len = class_info.sierra_program_length;
    let mut abi_len = class_info.abi_length;
    let class: ContractClass =
        if let Ok(class) = ContractClassV0::try_from_json_string(class_def.as_str()) {
            sierra_len = 0;
            abi_len = 0;
            class.into()
        } else if let Ok(class) = ContractClassV1::try_from_json_string(class_def.as_str()) {
            class.into()
        } else {
            return Err("not a valid contract class".to_string());
        };
    Ok(BlockifierClassInfo::new(&class, sierra_len, abi_len).unwrap())
}
