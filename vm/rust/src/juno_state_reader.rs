use std::{
    ffi::{c_char, c_uchar, c_void, CStr},
    fs,
    path::PathBuf,
    slice,
    sync::Mutex,
};
use blockifier::execution::contract_class::{ContractClass, NativeContractClassV1};
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
use serde::Deserialize;
use starknet_api::core::{ClassHash, CompiledClassHash, ContractAddress, Nonce};
use starknet_api::state::StorageKey;
use starknet_types_core::felt::Felt;

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
            let felt_val = ptr_to_felt(ptr);
            unsafe { JunoFree(ptr as *const c_void) };

            Ok(felt_val)
        }
    }

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
            Ok(Nonce(felt_val))
        }
    }

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
            Ok(ClassHash(felt_val))
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
        // lower number than previously cached or no cache
        // but same block will trigger a lot of loading

        let class_hash_bytes = felt_to_byte_array(&class_hash.0);
        let ptr = unsafe { JunoStateGetCompiledClass(self.handle, class_hash_bytes.as_ptr()) };
        if ptr.is_null() {
            Err(StateError::UndeclaredClassHash(class_hash))
        } else {
            // Invariant here we know that the contract does exist comparatively our reference block.
            let json_str = unsafe { CStr::from_ptr(ptr) }.to_str().unwrap();
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

pub fn felt_to_byte_array(felt: &Felt) -> [u8; 32] {
    felt.to_bytes_be().try_into().expect("Felt not [u8; 32]")
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

// todo(xrvdg) This should be private to juno state manager now it bypasses the cache
// when used for declare_transactions
pub fn class_info_from_json_str(
    raw_json: &str,
    class_hash: ClassHash,
) -> Result<BlockifierClassInfo, String> {
    let class_info: ClassInfo = serde_json::from_str(raw_json)
        .map_err(|err| format!("failed parsing class info: {:?}", err))?;
    let class_def = class_info.contract_class.to_string();

    // todo(xrvdg) Throws away errors
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

    return BlockifierClassInfo::new(
        &class,
        class_info.sierra_program_length,
        class_info.abi_length,
    )
    .map_err(|err| err.to_string());
}

/// Compiled Native contracts

fn native_try_from_json_string(
    raw_contract_class: &str,
    library_output_path: &PathBuf,
) -> Result<NativeContractClassV1, Box<dyn std::error::Error>> {
    fn compile_and_load(
        sierra_program: Program,
        library_output_path: &PathBuf,
    ) -> Result<AotNativeExecutor, NativeError> {
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
/// If the contract still has to be compiled, use [persist_from_native_module] instead, it compiles and load.
/// The reason for these to be like this is that there are expensive computations that on compiling is already available.
fn load_compiled_contract(
    sierra_program: &Program,
    library_output_path: &PathBuf,
) -> Option<Result<AotNativeExecutor, NativeError>> {
    fn load(
        sierra_program: &Program,
        library_output_path: &PathBuf,
    ) -> Result<AotNativeExecutor, NativeError> {
        let has_gas_builtin = sierra_program
            .type_declarations
            .iter()
            .any(|decl| decl.long_id.generic_id.0.as_str() == "GasBuiltin");
        let config = has_gas_builtin.then_some(Default::default());
        let gas_metadata = GasMetadata::new(sierra_program, config)?;
        let program_registry = ProgramRegistry::new(sierra_program)?;
        let library = unsafe {
            Library::new(library_output_path).map_err(|err| NativeError::Error(err.to_string()))?
        };
        Ok(AotNativeExecutor::new(
            library,
            program_registry,
            gas_metadata,
        ))
    }

    match library_output_path.is_file() {
        true => Some(load(sierra_program, library_output_path)),
        false => None,
    }
}

// todo(xrvdg) once [class_info_from_json_str] is part of JunoStateReader
// setup can move there
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
) -> Result<AotNativeExecutor, NativeError> {
    let object_data = cairo_native::module_to_object(native_module.module(), Default::default())
        .map_err(|err| NativeError::LLVMCompileError(err.to_string()))?; // cairo native didn't include a from instance

    cairo_native::object_to_shared_lib(&object_data, library_output_path)
        .map_err(|err| NativeError::Error(err.to_string()))?;

    let gas_metadata = native_module
        .remove_metadata()
        .expect("native_module should have set gas_metadata");

    // Native Module also contains the program registry but we can't unpack it.
    // luckily it's cheap to create
    let program_registry = ProgramRegistry::new(sierra_program)?;

    let library = unsafe {
        Library::new(library_output_path).map_err(|err| NativeError::Error(err.to_string()))?
    };

    Ok(AotNativeExecutor::new(
        library,
        program_registry,
        gas_metadata,
    ))
}
