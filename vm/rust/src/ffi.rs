use std::ffi::{c_char, c_longlong, c_uchar, c_ulonglong, c_void, CString};

extern "C" {
    pub fn JunoReportError(reader_handle: usize, txnIndex: c_longlong, err: *const c_char);
    pub fn JunoAppendTrace(reader_handle: usize, json_trace: *const c_void, len: usize);
    pub fn JunoAppendResponse(reader_handle: usize, ptr: *const c_uchar);
    pub fn JunoAppendActualFee(reader_handle: usize, ptr: *const c_uchar);
    pub fn JunoAppendDAGas(reader_handle: usize, ptr: *const c_uchar, ptr: *const c_uchar);
    pub fn JunoAppendGasConsumed(
        reader_handle: usize,
        ptr: *const c_uchar,
        ptr: *const c_uchar,
        ptr: *const c_uchar,
    );
    pub fn JunoAddExecutionSteps(reader_handle: usize, execSteps: c_ulonglong);
}

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn freeString(s: *mut c_char) {
    if !s.is_null() {
        unsafe {
            // Convert the raw C string pointer back to a CString. This operation
            // takes ownership of the memory again and ensures it gets deallocated
            // when drop function returns.
            drop(CString::from_raw(s));
        }
    }
}
