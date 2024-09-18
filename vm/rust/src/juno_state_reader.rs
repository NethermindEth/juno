use blockifier::execution::contract_class::{ContractClass, NativeContractClassV1};
use blockifier::state::errors::StateError;
use blockifier::{
    execution::contract_class::{
        ClassInfo as BlockifierClassInfo, ContractClassV0, ContractClassV1,
    },
    state::state_api::{StateReader, StateResult},
};
use cached::{Cached, SizedCache};
use cairo_native::OptLevel;
use cairo_native::
    executor::contract::ContractExecutor
;
use once_cell::sync::Lazy;
use serde::Deserialize;
use starknet_api::core::{ClassHash, CompiledClassHash, ContractAddress, Nonce};
use starknet_api::state::StorageKey;
use starknet_types_core::felt::Felt;
use std::cell::RefCell;
use std::sync::Arc;
use std::{
    ffi::{c_char, c_uchar, c_void, CStr},
    fs,
    path::PathBuf,
    slice,
    sync::Mutex,
};

use crate::recorded_state;

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
    pub serdes: RefCell<recorded_state::NativeState>,
}

impl JunoStateReader {
    pub fn new(handle: usize, height: u64) -> Self {
        Self { handle, height, serdes: Default::default() }
    }
}

// Note [Replay Invariant]
//
// The CachedReader does the heavy lifting when replaying Juno calls and when executing blocks it memoizes calls to Juno.
// The recording relies on this memoization and the assertion is there to verifies this.
// However this assertion will panic if JunoStateReader is run without being wrapped in a CachedReader.
// For now this is fine, but if this changes in the future, rethink how to record to calls to Juno  .

impl StateReader for JunoStateReader {
    fn get_storage_at(
        &self,
        contract_address: ContractAddress,
        key: StorageKey,
    ) -> StateResult<Felt> {
        let addr = contract_address.0.key().to_bytes_be();
        let storage_key = key.0.key().to_bytes_be();
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

            // Note [Replay Invariant]
            assert_eq!(
                self.serdes.borrow_mut().storage.insert((contract_address, key), felt_val),
                None,
                "Overwritten storage"
            );

            Ok(felt_val)
        }
    }

    /// Returns the nonce of the given contract instance.
    /// Default: 0 for an uninitialized contract address.
    fn get_nonce_at(&self, contract_address: ContractAddress) -> StateResult<Nonce> {
        let addr = contract_address.0.key().to_bytes_be();
        let ptr = unsafe { JunoStateGetNonceAt(self.handle, addr.as_ptr()) };
        if ptr.is_null() {
            Err(StateError::StateReadError(format!(
                "failed to read nonce of address {}",
                contract_address.0.key()
            )))
        } else {
            let felt_val = ptr_to_felt(ptr);
            unsafe { JunoFree(ptr as *const c_void) };

            let nonce = Nonce(felt_val);

            // Note [Replay Invariant]
            assert_eq!(
                self.serdes.borrow_mut().nonce.insert(contract_address, nonce),
                None,
                "Overwriting nonce"
            );

            Ok(nonce)
        }
    }

    /// Returns the class hash of the contract class at the given contract instance.
    /// Default: 0 (uninitialized class hash) for an uninitialized contract address.
    fn get_class_hash_at(&self, contract_address: ContractAddress) -> StateResult<ClassHash> {
        println!("Juno State Reader(Rust): calling `get_class_hash_at` {0}", contract_address);
        let addr = contract_address.0.key().to_bytes_be();
        let ptr = unsafe { JunoStateGetClassHashAt(self.handle, addr.as_ptr()) };
        if ptr.is_null() {
            Err(StateError::StateReadError(format!(
                "failed to read class hash of address {}",
                contract_address.0.key()
            )))
        } else {
            let felt_val = ptr_to_felt(ptr);
            unsafe { JunoFree(ptr as *const c_void) };

            println!(
                "Juno State Reader(Rust): returning `get_class_hash_at` {0} ",
                contract_address
            );

            let class_hash = ClassHash(felt_val);

            // Note [Replay Invariant]
            assert_eq!(
                self.serdes.borrow_mut().class_hash.insert(contract_address, class_hash),
                None,
                "Overwritten class_hash"
            );
            Ok(class_hash)
        }
    }

    /// Returns the contract class of the given class hash.
    fn get_compiled_contract_class(&self, class_hash: ClassHash) -> StateResult<ContractClass> {
        println!("Juno State Reader(Rust): calling `get_compiled_contract_class` with class hash: {class_hash}");

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
                println!(
                    "Juno State Reader(Rust): `get_compiled_contract_class`: class hash {class_hash} in cache, returning."
                );
                return Ok(cached_class.definition.clone());
            }
        }

        let class_hash_bytes = class_hash.0.to_bytes_be();
        let ptr = unsafe { JunoStateGetCompiledClass(self.handle, class_hash_bytes.as_ptr()) };
        if ptr.is_null() {
            Err(StateError::UndeclaredClassHash(class_hash))
        } else {
            let json_str = unsafe { CStr::from_ptr(ptr) }.to_str().unwrap();

            // Note [Replay Invariant]
            assert_eq!(
                self.serdes.borrow_mut().contract_class.insert(class_hash, json_str.to_string()), // Can't serialize the Contract Class due to AotNativeExecutor therefore we store the string
                None,
                "Overwritten compiled contract_class"
            );

            let class_info_res = class_info_from_json_str(json_str, class_hash);
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

            println!(
                    "Juno State Reader(Rust): `get_compiled_contract_class`: success getting definition with class hash {class_hash}"
                );

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

pub fn ptr_to_felt(bytes: *const c_uchar) -> Felt {
    let slice = unsafe { slice::from_raw_parts(bytes, 32) };
    Felt::from_bytes_be(slice.try_into().expect("Juno felt not [u8; 32]"))
}

#[derive(Deserialize)]
pub struct ClassInfo {
    contract_class: Box<serde_json::value::RawValue>,
    sierra_program_length: usize,
    abi_length: usize,
}

pub fn class_info_from_json_str(
    raw_json: &str,
    class_hash: ClassHash,
) -> Result<BlockifierClassInfo, String> {
    let class_info: ClassInfo = serde_json::from_str(raw_json)
        .map_err(|err| format!("failed parsing class info: {:?}", err))?;
    let class_def = class_info.contract_class.to_string();

    // todo(xrvdg) Don't throw away errors
    let class: ContractClass =
        if let Ok(class) = ContractClassV0::try_from_json_string(class_def.as_str()) {
            class.into()
        } else if let Ok(class) = ContractClassV1::try_from_json_string(class_def.as_str()) {
            println!("v1 contract");
            class.into()
        } else if let Ok(class) = {
            println!("native contract");
            let library_output_path = generate_library_path(class_hash);
            native_try_from_json_string(class_def.as_str(), &library_output_path)
        } {
            class.into()
        } else {
            return Err("not a valid contract class".to_string());
        };

    BlockifierClassInfo::new(&class, class_info.sierra_program_length, class_info.abi_length)
        .map_err(|err| err.to_string())
}

/// Compiled Native contracts

/// Load a compiled native contract into memory
///
/// Tries to load the compiled contract class from library_output_path if it
/// exists, otherwise it will compile the raw_contract_class, load it into memory
/// and save the compilation artifact to library_output_path.
fn native_try_from_json_string(
    raw_contract_class: &str,
    library_output_path: &PathBuf,
) -> Result<NativeContractClassV1, Box<dyn std::error::Error>> {
    let sierra_contract_class: cairo_lang_starknet_classes::contract_class::ContractClass =
        serde_json::from_str(raw_contract_class)?;
    let sierra_program = sierra_contract_class.extract_sierra_program()?;
    let executor = ContractExecutor::load(&library_output_path).unwrap_or({
        let executor = ContractExecutor::new(&sierra_program, OptLevel::Default)?;
        executor.save(&library_output_path)?;
        executor
    });
    let contract_executor = NativeContractClassV1::new(Arc::new(executor), sierra_contract_class)?;
    Ok(contract_executor)
}

// todo(xrvdg) once [class_info_from_json_str] is part of JunoStateReader
// setup can move there.
lazy_static! {
    static ref JUNO_NATIVE_CACHE_DIR: PathBuf = setup_native_cache_dir();
}

fn setup_native_cache_dir() -> PathBuf {
    let mut path: PathBuf = match std::env::var("JUNO_NATIVE_CACHE_DIR") {
        Ok(path) => path.into(),
        Err(_err) => {
            let mut path = std::env::current_dir().unwrap();
            path.push("native_cache");
            path
        }
    };
    // The cache is invalidated by moving to a new native revision.
    // This environment variable is a compile time variable and set
    // by build.rs
    path.push(env!("NATIVE_VERSION"));
    let _ = fs::create_dir_all(&path);
    path
}

fn generate_library_path(class_hash: ClassHash) -> PathBuf {
    let mut path = JUNO_NATIVE_CACHE_DIR.clone();
    path.push(class_hash.to_string().trim_start_matches("0x"));
    path
}
