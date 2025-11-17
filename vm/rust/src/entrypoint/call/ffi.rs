use std::ffi::{c_uchar, c_void};

extern "C" {
    pub fn JunoAppendResponse(reader_handle: usize, ptr: *const c_uchar);
    pub fn JunoAppendStateDiff(reader_handle: usize, json_state_diff: *const c_void, len: usize);
}
