use std::ffi::c_void;

use anyhow::Result;

use crate::{entrypoint::call::ffi::JunoAppendStateDiff, ffi_type::state_diff::StateDiff};

pub fn append_state_diff(
    reader_handle: usize,
    state_diff: &StateDiff,
    writer_buffer: &mut Vec<u8>,
) -> Result<(), serde_json::Error> {
    writer_buffer.clear();
    serde_json::to_writer(&mut *writer_buffer, state_diff)?;

    let ptr = writer_buffer.as_ptr();
    let len = writer_buffer.len();

    unsafe {
        JunoAppendStateDiff(reader_handle, ptr as *const c_void, len);
    };
    Ok(())
}
