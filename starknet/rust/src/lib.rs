use cairo_lang_starknet_classes::casm_contract_class::CasmContractClass;
use std::ffi::{c_char, CStr, CString};

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn compileSierraToCasm(sierra_json: *const c_char, result: *mut *mut c_char) -> u8 {
    let sierra_json_str = match unsafe { CStr::from_ptr(sierra_json) }.to_str() {
        Ok(value) => value,
        Err(e) => {
            unsafe {
                *result = raw_cstr(e.to_string());
            }
            return 0;
        }
    };

    let sierra_class = match serde_json::from_str(sierra_json_str) {
        Ok(value) => value,
        Err(e) => {
            unsafe {
                *result = raw_cstr(e.to_string());
            }
            return 0;
        }
    };

    let casm_class = match CasmContractClass::from_contract_class(sierra_class, true, usize::MAX) {
        Ok(value) => value,
        Err(e) => {
            unsafe {
                *result = raw_cstr(e.to_string());
            }
            return 0;
        }
    };

    let casm_json = match serde_json::to_string(&casm_class) {
        Ok(value) => value,
        Err(e) => {
            unsafe {
                *result = raw_cstr(e.to_string());
            }
            return 0;
        }
    };

    unsafe {
        *result = raw_cstr(casm_json);
    }
    1
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
