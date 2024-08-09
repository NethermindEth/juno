use starknet_core::types::contract::legacy::LegacyContractClass;
use std::{
    ffi::{c_char, c_uchar, CStr},
    slice,
};

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn Cairo0ClassHash(class_json_str: *const c_char, hash: *mut c_uchar) {
    let class_json = unsafe { CStr::from_ptr(class_json_str) };
    let class_json = match class_json.to_str() {
        Ok(s) => s,
        Err(_) => return,
    };
    let class: Result<LegacyContractClass, serde_json::Error> = serde_json::from_str(class_json);

    let class = match class {
        Ok(c) => c,
        Err(_) => return,
    };

    let class_hash = match class.class_hash() {
        Ok(h) => h,
        Err(_) => return,
    };

    let hash_slice: &mut [u8] = unsafe { slice::from_raw_parts_mut(hash, 32) };
    let hash_bytes = class_hash.to_bytes_be();
    hash_slice.copy_from_slice(hash_bytes.as_slice());
}
