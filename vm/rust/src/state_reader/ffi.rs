use std::ffi::{c_char, c_int, c_uchar, c_void};

extern "C" {
    pub fn JunoFree(ptr: *const c_void);

    pub fn JunoStateGetStorageAt(
        reader_handle: usize,
        contract_address: *const c_uchar,
        storage_location: *const c_uchar,
        buffer: *mut c_uchar,
    ) -> c_int;
    pub fn JunoStateGetNonceAt(
        reader_handle: usize,
        contract_address: *const c_uchar,
        buffer: *mut c_uchar,
    ) -> c_int;
    pub fn JunoStateGetClassHashAt(
        reader_handle: usize,
        contract_address: *const c_uchar,
        buffer: *mut c_uchar,
    ) -> c_int;
    pub fn JunoStateGetCompiledClass(
        reader_handle: usize,
        class_hash: *const c_uchar,
    ) -> *const c_char;
    pub fn JunoStateGetCompiledClassHash(
        reader_handle: usize,
        class_hash: *const c_uchar,
        buffer: *mut c_uchar,
    ) -> c_int;
    pub fn JunoStateGetCompiledClassHashV2(
        reader_handle: usize,
        class_hash: *const c_uchar,
        buffer: *mut c_uchar,
    ) -> c_int;
}
