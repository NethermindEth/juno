use cairo_lang_starknet_classes::casm_contract_class::CasmContractClass;
use std::ffi::{c_char, CStr, CString};

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn compileSierraToCasm(sierra_json: *const c_char) -> *mut c_char {
    let sierra_json_str = unsafe { CStr::from_ptr(sierra_json) }.to_str().unwrap();

    let sierra_class = match serde_json::from_str(sierra_json_str) {
        Ok(value) => value,
        Err(e) => return raw_cstr(e.to_string()),
    };

    let casm_class = match CasmContractClass::from_contract_class(sierra_class, true, usize::MAX) {
        Ok(value) => value,
        Err(e) => return raw_cstr(e.to_string()),
    };

    let casm_json = match serde_json::to_string(&casm_class) {
        Ok(value) => value,
        Err(e) => return raw_cstr(e.to_string()),
    };

    raw_cstr(casm_json)
}

fn raw_cstr(str: String) -> *mut c_char {
    CString::new(str).unwrap().into_raw()
}

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn freeCstr(ptr: *mut c_char) {
    unsafe {
        if ptr.is_null() {
            return;
        }
        let _ = CString::from_raw(ptr);
    };
}
