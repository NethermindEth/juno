use std::{
    fs::File,
    time::{Duration, Instant},
};

use blockifier::state::cached_state::CachedState;
use criterion::{criterion_group, criterion_main, Criterion};
use juno_starknet_rs::{
    serstate::{self, CompiledNativeState, CompiledVMState, NativeState, SerState, VMState},
    VMArgs,
};

fn criterion_benchmark_precompiled_native(c: &mut Criterion) {
    let block_number = 633333;
    let args_file = File::open(format!(
        "/Users/xander/Nethermind/juno-native/record/{block_number}.args.cbor"
    ))
    .unwrap();
    // Can also do just a pattern match
    let mut vm_args: VMArgs = ciborium::from_reader(args_file).unwrap();

    let serstate_file = File::open(format!(
        "/Users/xander/Nethermind/juno-native/record/{block_number}.state.cbor"
    ))
    .unwrap();
    let serstate: SerState = ciborium::from_reader(serstate_file).unwrap();
    println!("setting up native state");
    let start = Instant::now();
    let native_state = CompiledNativeState::new(NativeState(serstate.clone()));
    println!(
        "setting up native state finished: {}",
        start.elapsed().as_millis()
    );
    // let mut vm_state = CachedState::new(VMState(serstate));

    // load up native cache
    // serstate::run(&mut vm_args, &mut native_state);

    let mut group = c.benchmark_group("sample_size");
    // group.warm_up_time(Duration::from_secs(15));
    group.sample_size(10);

    group.bench_function("native", |b| {
        b.iter(|| {
            // clone takes around 3ms
            let cloned_native_state = native_state.clone();
            let mut cached_native_state = CachedState::new(cloned_native_state);
            serstate::run(&mut vm_args, &mut cached_native_state);
        })
    });

    group.finish();
}

fn criterion_benchmark_compile_every_loop(c: &mut Criterion) {
    let mut group = c.benchmark_group("sample_size");
    group.sample_size(10);

    group.bench_function("fresh", move |b| {
        b.iter(|| {
            let block_number = 633333;
            let args_file = File::open(format!(
                "/Users/xander/Nethermind/juno-native/record/{block_number}.args.cbor"
            ))
            .unwrap();
            // Can also do just a pattern match
            let mut vm_args: VMArgs = ciborium::from_reader(args_file).unwrap();

            let serstate_file = File::open(format!(
                "/Users/xander/Nethermind/juno-native/record/{block_number}.state.cbor"
            ))
            .unwrap();
            let serstate: SerState = ciborium::from_reader(serstate_file).unwrap();
            let mut native_state =
                CachedState::new(CompiledNativeState::new(NativeState(serstate.clone())));

            serstate::run(&mut vm_args, &mut native_state)
        })
    });
    group.finish();
}

fn criterion_benchmark_cached_native_in(c: &mut Criterion) {
    let mut group = c.benchmark_group("sample_size");
    group.sample_size(10);

    group.bench_function("fresh", move |b| {
        b.iter(|| {
            let block_number = 633333;
            let args_file = File::open(format!(
                "/Users/xander/Nethermind/juno-native/record/{block_number}.args.cbor"
            ))
            .unwrap();
            // Can also do just a pattern match
            let mut vm_args: VMArgs = ciborium::from_reader(args_file).unwrap();

            let serstate_file = File::open(format!(
                "/Users/xander/Nethermind/juno-native/record/{block_number}.state.cbor"
            ))
            .unwrap();
            let serstate: SerState = ciborium::from_reader(serstate_file).unwrap();
            let mut native_state = CachedState::new(NativeState(serstate.clone()));

            serstate::run(&mut vm_args, &mut native_state)
        })
    });
    group.finish();
}

// Not working
fn criterion_benchmark______(c: &mut Criterion) {
    let mut group = c.benchmark_group("sample_size");
    group.sample_size(10);

    group.bench_function("fresh", move |b| {
        b.iter(|| {
            let block_number = 633333;
            let args_file = File::open(format!(
                "/Users/xander/Nethermind/juno-native/record/{block_number}.args.cbor"
            ))
            .unwrap();
            // Can also do just a pattern match
            let mut vm_args: VMArgs = ciborium::from_reader(args_file).unwrap();

            let serstate_file = File::open(format!(
                "/Users/xander/Nethermind/juno-native/record/{block_number}.state.cbor"
            ))
            .unwrap();
            let serstate: SerState = ciborium::from_reader(serstate_file).unwrap();
            let mut native_state = CachedState::new(VMState(serstate.clone()));

            serstate::run(&mut vm_args, &mut native_state)
        })
    });
    group.finish();
}

criterion_group!(benches, criterion_benchmark_precompiled_native);
criterion_main!(benches);
