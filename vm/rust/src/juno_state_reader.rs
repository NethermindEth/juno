use std::{
    ffi::{c_char, c_uchar, c_void, CStr},
    fs, mem, slice,
    sync::Mutex,
    time,
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
use cairo_lang_sierra::program_registry::ProgramRegistry;
use cairo_native::executor::AotNativeExecutor;
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

pub fn class_info_from_json_str(
    raw_json: &str,
    class_hash: ClassHash,
) -> Result<BlockifierClassInfo, String> {
    let class_info: ClassInfo = serde_json::from_str(raw_json)
        .map_err(|err| format!("failed parsing class info: {:?}", err))?;
    let class_def = class_info.contract_class.to_string();

    let class: ContractClass =
        if let Ok(class) = ContractClassV0::try_from_json_string(class_def.as_str()) {
            class.into()
        } else if let Ok(class) = ContractClassV1::try_from_json_string(class_def.as_str()) {
            class.into()
        } else if let Ok(class) = native_try_from_json_string(class_def.as_str(), class_hash) {
            class.into()
        } else {
            return Err("not a valid contract class".to_string());
        };
    return Ok(BlockifierClassInfo::new(
        &class.into(),
        class_info.sierra_program_length,
        class_info.abi_length,
    )
    .unwrap());
}

fn native_try_from_json_string(
    raw_contract_class: &str,
    class_hash: ClassHash,
) -> Result<NativeContractClassV1, Box<dyn std::error::Error>> {
    // Compile the Sierra Program to native code and loads it into the process'
    // memory space.
    fn compile_and_load(
        sierra_program: &cairo_lang_sierra::program::Program,
        class_hash: ClassHash,
    ) -> Result<AotNativeExecutor, cairo_native::error::Error> {
        let start = std::time::Instant::now();
        let native_context = cairo_native::context::NativeContext::new();
        println!(
            "native context creation duration: {}ms",
            start.elapsed().as_millis()
        );
        let start = std::time::Instant::now();
        let mut native_module = native_context.compile(sierra_program, None)?;
        println!("native compile duration: {}ms", start.elapsed().as_millis());
        // Redoing work from compile because we can't get the
        let start = std::time::Instant::now();
        let program_registry = ProgramRegistry::new(sierra_program).unwrap();
        println!(
            "program registry duration: {}ms",
            start.elapsed().as_millis()
        );

        let start = std::time::Instant::now();
        // TODO(xrvdg) choose where you want to have the directory
        let mut library_path = std::env::current_dir().unwrap();
        library_path.push("native_cache");
        library_path.push(env!("NATIVE_VERSION"));
        // TODO(xrvdg) don't create directory on every compile
        let _ = fs::create_dir_all(&library_path);
        // TODO(xrvdg) class hash without
        library_path.push(class_hash.to_string());

        println!("library_path duration: {}ms", start.elapsed().as_millis());

        let start = std::time::Instant::now();
        // TODO(xrvdg) check if shared object is already in memory
        // RTLD_NOLOAD on library, Windows seems to have something similar
        if !library_path.is_file() {
            println!("compiling {}", library_path.display());
            let opt_level = cairo_native::OptLevel::Default;
            let object_data =
                cairo_native::module_to_object(native_module.module(), opt_level).unwrap();
            cairo_native::object_to_shared_lib(&object_data, &library_path).unwrap();
        }
        println!("is_file duration: {}ms", start.elapsed().as_millis());
        println!("Loading {}", library_path.display());

        let start = std::time::Instant::now();
        let aot_native_executor = AotNativeExecutor::new(
            unsafe { Library::new(library_path).unwrap() },
            program_registry,
            native_module.remove_metadata().unwrap(),
        );
        println!("loading duration: {}ms", start.elapsed().as_millis());

        Ok(aot_native_executor)
    }

    let sierra_contract_class: cairo_lang_starknet_classes::contract_class::ContractClass =
        serde_json::from_str(raw_contract_class)?;

    // todo(rodro): we are having two instances of a sierra program, one it's object form
    // and another in its felt encoded form. This can be avoided by either:
    //   1. Having access to the encoding/decoding functions
    //   2. Refactoring the code on the Cairo mono-repo

    let sierra_program = sierra_contract_class.extract_sierra_program()?;
    let executor = compile_and_load(&sierra_program, class_hash)?;

    Ok(NativeContractClassV1::new(executor, sierra_contract_class)?)
}
