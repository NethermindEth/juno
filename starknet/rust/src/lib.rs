use std::ffi::{c_char, CStr, CString};
use cairo_lang_starknet::casm_contract_class::CasmContractClass;

#[no_mangle]
pub extern "C" fn compileSierraToCasm(sierra_json: *const c_char) -> *mut c_char {
    let sierra_json_str = unsafe { CStr::from_ptr(sierra_json) }.to_str().unwrap();

    let sierra_class =
        serde_json::from_str(sierra_json_str).map_err(|err| err.to_string());
    if let Err(e) = sierra_class {
        return raw_cstr(e)
    }

    let casm_class = CasmContractClass::from_contract_class(sierra_class.unwrap(), true)
        .map_err(|err| err.to_string());
    if let Err(e) = casm_class {
        return raw_cstr(e)
    }

    let casm_json = serde_json::to_string(&casm_class.unwrap());
    if let Err(e) = casm_json {
        return raw_cstr(e.to_string())
    }
    return raw_cstr(casm_json.unwrap())
}

fn raw_cstr(str: String) -> *mut c_char {
    CString::new(str).unwrap().into_raw()
}

#[no_mangle]
pub extern "C" fn freeCstr(ptr: *mut c_char) {
    unsafe {
        if ptr.is_null() {
            return;
        }
        let _ = CString::from_raw(ptr);
    };
}
