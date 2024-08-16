use blockifier::execution::contract_class::{ContractClass, NativeContractClassV1};
use blockifier::state::cached_state::StorageEntry;
use blockifier::state::errors::StateError;
use blockifier::{
    execution::contract_class::{
        ClassInfo as BlockifierClassInfo, ContractClassV0, ContractClassV1,
    },
    state::state_api::{StateReader, StateResult},
};
use cached::{Cached, SizedCache};
use cairo_lang_sierra::{program::Program, program_registry::ProgramRegistry};
use cairo_native::{
    context::NativeContext, error::Error as NativeError, executor::AotNativeExecutor,
    metadata::gas::GasMetadata, module::NativeModule,
};
use libloading::Library;
use once_cell::sync::Lazy;
use serde::{Deserialize, Serialize};
use starknet_api::core::{ClassHash, CompiledClassHash, ContractAddress, Nonce};
use starknet_api::state::StorageKey;
use starknet_types_core::felt::Felt;
use std::cell::RefCell;
use std::collections::HashMap;
use std::{
    ffi::{c_char, c_uchar, c_void, CStr},
    fs,
    path::PathBuf,
    slice,
    sync::Mutex,
};

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

#[derive(Default, Serialize, Deserialize)]
pub struct SerState {
    // Could have been wrapped around with felt
    storage: HashMap<StorageEntry, [u8; 32]>,
    nonce: HashMap<ContractAddress, Nonce>,
    class_hash: HashMap<ContractAddress, ClassHash>,
    contract_class: HashMap<ClassHash, String>,
}

impl StateReader for SerState {
    fn get_storage_at(
        &self,
        contract_address: ContractAddress,
        key: StorageKey,
    ) -> StateResult<Felt> {
        // might have to deal with
        let bytes = self
            .storage
            .get(&(contract_address, key))
            .expect("no storage");
        Ok(Felt::from_bytes_be(bytes))
    }

    fn get_nonce_at(&self, contract_address: ContractAddress) -> StateResult<Nonce> {
        Ok(*self
            .nonce
            .get(&contract_address)
            .expect("no nonce for this"))
    }

    fn get_class_hash_at(&self, contract_address: ContractAddress) -> StateResult<ClassHash> {
        Ok(*self
            .class_hash
            .get(&contract_address)
            .expect("no nonce for this"))
    }

    fn get_compiled_contract_class(&self, class_hash: ClassHash) -> StateResult<ContractClass> {
        // Passing along version information could make this a V1 and V1Native test
        let json_str = self
            .contract_class
            .get(&class_hash)
            .expect("request non existed class");
        Ok(class_info_from_json_str(json_str, class_hash)
            .expect("decoding class went wrong")
            .contract_class())
    }

    fn get_compiled_class_hash(&self, _class_hash: ClassHash) -> StateResult<CompiledClassHash> {
        unimplemented!()
    }
}

pub struct JunoStateReader {
    pub handle: usize, // uintptr_t equivalent
    pub height: u64,
    pub ser: RefCell<SerState>,
}

impl JunoStateReader {
    pub fn new(handle: usize, height: u64) -> Self {
        Self {
            handle,
            height,
            ser: Default::default(),
        }
    }
}

impl StateReader for JunoStateReader {
    // input serializable output isn't
    fn get_storage_at(
        &self,
        contract_address: ContractAddress,
        key: StorageKey,
    ) -> StateResult<Felt> {
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
            let felt_val = {
                let slice = unsafe { slice::from_raw_parts(ptr, 32) };

                let bytes = slice.try_into().expect("Juno felt not [u8; 32]");
                // Insert into serialization map
                // must return none
                assert_eq!(
                    self.ser
                        .borrow_mut()
                        .storage
                        .insert((contract_address, key), bytes),
                    None,
                    "Overwritten storage"
                );

                Felt::from_bytes_be(&bytes)
            };
            unsafe { JunoFree(ptr as *const c_void) };

            Ok(felt_val)
        }
    }

    // input and output are serializable
    /// Returns the nonce of the given contract instance.
    /// Default: 0 for an uninitialized contract address.
    fn get_nonce_at(&self, contract_address: ContractAddress) -> StateResult<Nonce> {
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

            let nonce = Nonce(felt_val);

            assert_eq!(
                self.ser.borrow_mut().nonce.insert(contract_address, nonce),
                None,
                "Overwriting nonce"
            );

            Ok(nonce)
        }
    }

    // input and output are serializable
    /// Returns the class hash of the contract class at the given contract instance.
    /// Default: 0 (uninitialized class hash) for an uninitialized contract address.
    fn get_class_hash_at(&self, contract_address: ContractAddress) -> StateResult<ClassHash> {
        println!(
            "Juno State Reader(Rust): calling `get_class_hash_at` {0}",
            contract_address
        );
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

            println!(
                "Juno State Reader(Rust): returning `get_class_hash_at` {0} ",
                contract_address
            );

            let class_hash = ClassHash(felt_val);
            assert_eq!(
                self.ser
                    .borrow_mut()
                    .class_hash
                    .insert(contract_address, class_hash),
                None,
                "Overwritten class_hash"
            );
            Ok(class_hash)
        }
    }

    // input is serializable but output isn't
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

        let class_hash_bytes = felt_to_byte_array(&class_hash.0);
        let ptr = unsafe { JunoStateGetCompiledClass(self.handle, class_hash_bytes.as_ptr()) };
        if ptr.is_null() {
            Err(StateError::UndeclaredClassHash(class_hash))
        } else {
            let json_str = unsafe { CStr::from_ptr(ptr) }.to_str().unwrap();

            assert_eq!(
                self.ser
                    .borrow_mut()
                    .contract_class
                    .insert(class_hash, json_str.to_string()),
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

// todo(xrvdg) Unnecessary
pub fn felt_to_byte_array(felt: &Felt) -> [u8; 32] {
    felt.to_bytes_be()
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

// todo(xrvdg) This should be private to juno state manager that way
// caching can also be used for declare_transactions
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
            class.into()
        } else if let Ok(class) = {
            let library_output_path = generate_library_path(class_hash);
            native_try_from_json_string(class_def.as_str(), &library_output_path)
        } {
            class.into()
        } else {
            return Err("not a valid contract class".to_string());
        };

    BlockifierClassInfo::new(
        &class,
        class_info.sierra_program_length,
        class_info.abi_length,
    )
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
    fn compile_and_load(
        sierra_program: Program,
        library_output_path: &PathBuf,
    ) -> Result<AotNativeExecutor, Box<dyn std::error::Error>> {
        let native_context = NativeContext::new();
        let native_module = native_context.compile(&sierra_program, None)?;
        persist_from_native_module(native_module, &sierra_program, library_output_path)
    }

    let sierra_contract_class: cairo_lang_starknet_classes::contract_class::ContractClass =
        serde_json::from_str(raw_contract_class)?;

    // todo(rodro): we are having two instances of a sierra program, one it's object form
    // and another in its felt encoded form. This can be avoided by either:
    //   1. Having access to the encoding/decoding functions
    //   2. Refactoring the code on the Cairo mono-repo

    let sierra_program = sierra_contract_class.extract_sierra_program()?;

    // todo(xrvdg) lift this match out of the function once we do not need sierra_program anymore
    let executor = match load_compiled_contract(&sierra_program, library_output_path) {
        Some(executor) => {
            executor.or_else(|_err| compile_and_load(sierra_program, library_output_path))
        }
        None => compile_and_load(sierra_program, library_output_path),
    }?;

    Ok(NativeContractClassV1::new(executor, sierra_contract_class)?)
}

/// Load a contract that is already compiled.
///
/// Returns None if the contract does not exist at the output_path.
///
/// To compile and load a contract use [persist_from_native_module] instead.
fn load_compiled_contract(
    sierra_program: &Program,
    library_output_path: &PathBuf,
) -> Option<Result<AotNativeExecutor, Box<dyn std::error::Error>>> {
    fn load(
        sierra_program: &Program,
        library_output_path: &PathBuf,
    ) -> Result<AotNativeExecutor, Box<dyn std::error::Error>> {
        let has_gas_builtin = sierra_program
            .type_declarations
            .iter()
            .any(|decl| decl.long_id.generic_id.0.as_str() == "GasBuiltin");
        let config = has_gas_builtin.then_some(Default::default());
        let gas_metadata = GasMetadata::new(sierra_program, config)?;
        let program_registry = ProgramRegistry::new(sierra_program)?;
        let library = unsafe { Library::new(library_output_path)? };
        Ok(AotNativeExecutor::new(
            library,
            program_registry,
            gas_metadata,
        ))
    }

    library_output_path
        .is_file()
        .then_some(load(sierra_program, library_output_path))
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

/// Compiles and load contract
///
/// Modelled after [AotNativeExecutor::from_native_module].
/// Needs a sierra_program to workaround limitations of NativeModule
fn persist_from_native_module(
    mut native_module: NativeModule,
    sierra_program: &Program,
    library_output_path: &PathBuf,
) -> Result<AotNativeExecutor, Box<dyn std::error::Error>> {
    let object_data = cairo_native::module_to_object(native_module.module(), Default::default())
        .map_err(|err| NativeError::LLVMCompileError(err.to_string()))?; // cairo native didn't include a from instance

    cairo_native::object_to_shared_lib(&object_data, library_output_path)?;

    let gas_metadata = native_module
        .remove_metadata()
        .expect("native_module should have set gas_metadata");

    // Recreate the program registry as it can't be moved out of native module.
    let program_registry = ProgramRegistry::new(sierra_program)?;

    let library = unsafe { Library::new(library_output_path)? };

    Ok(AotNativeExecutor::new(
        library,
        program_registry,
        gas_metadata,
    ))
}
