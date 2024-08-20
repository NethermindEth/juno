use std::{fs::File, time::Instant};

use blockifier::state::cached_state::CachedState;
use criterion::{criterion_group, criterion_main, Criterion};
use juno_starknet_rs::{
    serstate::{self, CompiledNativeState, NativeState},
    VMArgs,
};

fn criterion_benchmark_precompiled_native(c: &mut Criterion) {
    let block_number = 633333;
    let args_file = File::open(format!(
        "/Users/xander/Nethermind/juno-native/record/{block_number}.args.cbor"
    ))
    .unwrap();
    let mut vm_args: VMArgs = ciborium::from_reader(args_file).unwrap();

    let serstate_file = File::open(format!(
        "/Users/xander/Nethermind/juno-native/record/{block_number}.state.cbor"
    ))
    .unwrap();
    let native_state: NativeState = ciborium::from_reader(serstate_file).unwrap();
    println!("setting up native state");
    let start = Instant::now();

    let compiled_native_state = CompiledNativeState::new(native_state);
    println!(
        "setting up native state finished: {}",
        start.elapsed().as_millis()
    );

    let mut group = c.benchmark_group("sample_size");

    group.sample_size(10);

    group.bench_function("native", |b| {
        b.iter(|| {
            let mut cached_compiled_native_state = CachedState::new(compiled_native_state.clone());
            serstate::run(&mut vm_args, &mut cached_compiled_native_state);
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
            let native_state: NativeState = ciborium::from_reader(serstate_file).unwrap();
            // check this clone
            let mut cached_compiled_native_state =
                CachedState::new(CompiledNativeState::new(native_state));

            serstate::run(&mut vm_args, &mut cached_compiled_native_state)
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
            let native_state: NativeState = ciborium::from_reader(serstate_file).unwrap();
            // check this clone
            let mut cached_native_state = CachedState::new(native_state);

            serstate::run(&mut vm_args, &mut cached_native_state)
        })
    });
    group.finish();
}

criterion_group!(benches, criterion_benchmark_precompiled_native);
criterion_main!(benches);
