// note(rdr): unknowingy took the route of having similar named crates which is warning
// given by clippy (enabled in this commit). We should consider pros/cons and if we want
// change it or not.
#![allow(clippy::module_inception)]

mod entrypoint;
mod error;
mod ffi_entrypoint;
mod ffi_type;
mod state_reader;
mod versioned_constants;
