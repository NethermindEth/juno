use starknet_core::types::contract::legacy::LegacyContractClass;
use std::{
    ffi::{c_char, c_uchar, CStr},
    slice,
};

#[no_mangle]
pub extern "C" fn Cairo0ClassHash(class_json_str: *const c_char, hash: *mut c_uchar) {
    let class_json = unsafe { CStr::from_ptr(class_json_str) }.to_str().unwrap();
    let class: Result<LegacyContractClass, serde_json::Error> = serde_json::from_str(class_json);
    if class.is_err() {
        return;
    }
    let class_hash = class.unwrap().class_hash();
    if class_hash.is_err() {
        return;
    }
    let hash_slice: &mut [u8] = unsafe { slice::from_raw_parts_mut(hash, 32) };
    let hash_bytes = class_hash.unwrap().to_bytes_be();
    hash_slice.copy_from_slice(hash_bytes.as_slice());
}
