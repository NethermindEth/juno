use std::ffi::CString;
use std::str::FromStr;
use std::{
    collections::HashMap,
    ffi::{c_char, CStr},
};

use blockifier::versioned_constants::VersionedConstants;

use crate::types::StarknetVersion;

static mut CUSTOM_VERSIONED_CONSTANTS: Option<VersionedConstants> = None;

lazy_static! {
    static ref CONSTANTS: HashMap<String, VersionedConstants> = {
        let mut m = HashMap::new();
        m.insert(
            "0.13.0".to_string(),
            serde_json::from_slice(include_bytes!(
                "../resources/versioned_constants_0_13_0.json"
            ))
            .unwrap(),
        );
        m.insert(
            "0.13.1".to_string(),
            serde_json::from_slice(include_bytes!(
                "../resources/versioned_constants_0_13_1.json"
            ))
            .unwrap(),
        );
        m.insert(
            "0.13.1.1".to_string(),
            serde_json::from_slice(include_bytes!(
                "../resources/versioned_constants_0_13_1_1.json"
            ))
            .unwrap(),
        );
        m.insert(
            "0.13.2".to_string(),
            serde_json::from_slice(include_bytes!(
                "../resources/versioned_constants_0_13_2.json"
            ))
            .unwrap(),
        );
        m.insert(
            "0.13.3".to_string(),
            serde_json::from_slice(include_bytes!(
                "../resources/versioned_constants_0_13_3.json"
            ))
            .unwrap(),
        );
        m.insert(
            "0.13.4".to_string(),
            serde_json::from_slice(include_bytes!(
                "../resources/versioned_constants_0_13_4.json"
            ))
            .unwrap(),
        );
        m
    };
}

#[allow(static_mut_refs)]
pub fn get_versioned_constants(version: *const c_char) -> VersionedConstants {
    let version_str = unsafe { CStr::from_ptr(version) }
        .to_str()
        .unwrap_or("0.0.0");

    let version = match StarknetVersion::from_str(version_str) {
        Ok(v) => v,
        Err(_) => StarknetVersion::from_str("0.0.0").unwrap(),
    };

    if let Some(constants) = unsafe { &CUSTOM_VERSIONED_CONSTANTS } {
        constants.clone()
    } else if version < StarknetVersion::from_str("0.13.1").unwrap() {
        CONSTANTS.get(&"0.13.0".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str("0.13.1.1").unwrap() {
        CONSTANTS.get(&"0.13.1".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str("0.13.2").unwrap() {
        CONSTANTS.get(&"0.13.1.1".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str("0.13.2.1").unwrap() {
        CONSTANTS.get(&"0.13.2".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str("0.13.3").unwrap() {
        CONSTANTS.get(&"0.13.3".to_string()).unwrap().to_owned()
    } else if version < StarknetVersion::from_str("0.13.4").unwrap() {
        CONSTANTS.get(&"0.13.4".to_string()).unwrap().to_owned()
    } else {
        VersionedConstants::latest_constants().to_owned()
    }
}

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn setVersionedConstants(json_bytes: *const c_char) -> *const c_char {
    let json_str = unsafe {
        match CStr::from_ptr(json_bytes).to_str() {
            Ok(s) => s,
            Err(_) => {
                return CString::new("Failed to convert JSON bytes to string")
                    .unwrap()
                    .into_raw()
            }
        }
    };

    match serde_json::from_str(json_str) {
        Ok(parsed) => unsafe {
            CUSTOM_VERSIONED_CONSTANTS = Some(parsed);
            CString::new("").unwrap().into_raw() // No error, return an empty string
        },
        Err(_) => CString::new("Failed to parse JSON").unwrap().into_raw(),
    }
}
