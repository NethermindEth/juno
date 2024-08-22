use std::fs::File;

use juno_starknet_rs::{recorded_state::NativeState, VMArgs};

#[test]
fn load_vmargs() {
    let block_number = 633333;
    let args_file = File::open(format!(
        "/Users/xander/Nethermind/juno-native/record/{block_number}.args.cbor"
    ))
    .unwrap();
    // Can also do just a pattern match
    let _vm_args: VMArgs = ciborium::from_reader(args_file).unwrap();
}

#[test]
fn load_state() {
    let block_number = 633333;
    let state_file = File::open(format!(
        "/Users/xander/Nethermind/juno-native/record/{block_number}.state.cbor"
    ))
    .unwrap();
    // Can also do just a pattern match
    let _state: NativeState = ciborium::from_reader(state_file).unwrap();
}
