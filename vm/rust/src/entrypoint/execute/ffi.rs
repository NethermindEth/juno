use std::ffi::{c_uchar, c_ulonglong, c_void};

extern "C" {
    pub fn JunoAddExecutionSteps(reader_handle: usize, execSteps: c_ulonglong);
    pub fn JunoAppendTrace(reader_handle: usize, json_trace: *const c_void, len: usize);
    pub fn JunoAppendReceipt(reader_handle: usize, json_receipt: *const c_void, len: usize);
    pub fn JunoAppendActualFee(reader_handle: usize, ptr: *const c_uchar);
    pub fn JunoAppendDAGas(reader_handle: usize, ptr: *const c_uchar, ptr: *const c_uchar);
    pub fn JunoAppendGasConsumed(
        reader_handle: usize,
        ptr: *const c_uchar,
        ptr: *const c_uchar,
        ptr: *const c_uchar,
    );
}
