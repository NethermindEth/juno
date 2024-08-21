use std::{env, fs::File, time::Instant};

use blockifier::state::cached_state::CachedState;
use criterion::{black_box, criterion_group, criterion_main, Criterion};
use juno_starknet_rs::{
    serstate::{self, CompiledNativeState, NativeState},
    VMArgs,
};

fn load_record() -> (VMArgs, NativeState) {
    // This is a workaround of not being able to supply arguments over the command line.
    let block_number: u64 = env::var("BENCH_BLOCK")
        .expect(
            "To select the block to benchmark add BENCH_BLOCK=block_number infront of the command",
        )
        .parse()
        .expect("failed to parse BENCH_BLOCK");

    let record_directory = env::var("JUNO_RECORD_DIR").expect("JUNO_RECORD_DIR has not been set");

    // TODO(xrvdg) built proper file path
    let args_file = File::open(format!("{record_directory}/{block_number}.args.cbor")).unwrap();
    let vm_args: VMArgs = ciborium::from_reader(args_file).unwrap();

    let serstate_file =
        File::open(format!("{record_directory}/{block_number}.state.cbor")).unwrap();

    let native_state: NativeState = ciborium::from_reader(serstate_file).unwrap();

    (vm_args, native_state)
}

// This benchmark preloads the compiled contracts and then starts the benchmark.
fn preload(c: &mut Criterion) {
    let mut group = c.benchmark_group("preload");
    group.sample_size(10);

    let (mut vm_args, native_state) = load_record();

    println!("Setup: loading in contracts");
    let start = Instant::now();
    // Either loads from disk or compiles
    let compiled_native_state = CompiledNativeState::new(native_state);
    println!(
        "Setup: finished loading contracts in {} ms",
        start.elapsed().as_millis()
    );

    group.bench_function("native", |b| {
        b.iter(|| {
            // The clone is negligible compared to the run
            let mut cached_compiled_native_state = CachedState::new(compiled_native_state.clone());
            serstate::run(
                black_box(&mut vm_args),
                black_box(&mut cached_compiled_native_state),
            );
        })
    });

    group.finish();
}

// This benchmark simulates a cold boot of the blockifier with none of the contracts loaded in.
fn cold(c: &mut Criterion) {
    let mut group = c.benchmark_group("cold");
    group.sample_size(10);

    let (mut vm_args, native_state) = load_record();

    group.bench_function("native", move |b| {
        b.iter(|| {
            // The clone is negligible compared to the run
            let mut cached_native_state = CachedState::new(native_state.clone());

            serstate::run(black_box(&mut vm_args), black_box(&mut cached_native_state))
        })
    });

    group.finish();
}

criterion_group!(benches, preload, cold,);
criterion_main!(benches);
