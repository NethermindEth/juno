use std::{env, fs::File, path::PathBuf, time::Instant};

use blockifier::state::cached_state::CachedState;
use criterion::{black_box, criterion_group, criterion_main, Criterion};
use juno_starknet_rs::{
    recorded_state::{self, CompiledNativeState, NativeState},
    VMArgs,
};

fn load_recording() -> (VMArgs, NativeState) {
    // This is a workaround of not being able to supply arguments over the command line.
    let block_number: u64 = env::var("BENCH_BLOCK")
        .expect(
            "To select the block to benchmark add BENCH_BLOCK=block_number infront of the command",
        )
        .parse()
        .expect("failed to parse BENCH_BLOCK");

    let record_directory = env::var("JUNO_RECORD_DIR").expect("JUNO_RECORD_DIR has not been set");

    let mut args_path: PathBuf = record_directory.clone().into();
    args_path.push(format!("{}.args.cbor", block_number));
    let args_file = File::open(args_path).unwrap();
    let vm_args: VMArgs = ciborium::from_reader(args_file).unwrap();

    let mut state_path: PathBuf = record_directory.into();
    state_path.push(format!("{}.state.cbor", block_number));
    let state_file = File::open(state_path).unwrap();

    let native_state: NativeState = ciborium::from_reader(state_file).unwrap();

    (vm_args, native_state)
}

/// This benchmark preloads the compiled contracts and then starts the benchmark.
fn preload(c: &mut Criterion) {
    let mut group = c.benchmark_group("preload");
    group.sample_size(10);

    // TODO(xrvdg) move this into a loading function otherwise it's always executed
    // even when filtered
    let (mut vm_args, native_state) = load_recording();

    println!("Setup: loading contracts into memory");
    let start = Instant::now();
    // Either loads from disk or compiles
    let compiled_native_state = CompiledNativeState::new(native_state);
    println!(
        "Setup: finished loading contracts ({} ms)",
        start.elapsed().as_millis()
    );

    group.bench_function("native", |b| {
        b.iter(|| {
            // The clone is negligible compared to the run
            let mut cached_compiled_native_state = CachedState::new(compiled_native_state.clone());
            recorded_state::run(
                black_box(&mut vm_args),
                black_box(&mut cached_compiled_native_state),
            );
        })
    });

    group.finish();
}

/// This benchmark simulates a cold boot of the blockifier with none of the contracts loaded in.
///
/// This should be the same as the other two benchmarks combined and is left here to be able to verify that quickly.
#[allow(dead_code)]
fn cold(c: &mut Criterion) {
    let mut group = c.benchmark_group("cold");
    group.sample_size(10);

    let (mut vm_args, native_state) = load_recording();

    group.bench_function("native", move |b| {
        b.iter(|| {
            // The clone is negligible compared to the run
            let mut cached_native_state = CachedState::new(native_state.clone());

            recorded_state::run(black_box(&mut vm_args), black_box(&mut cached_native_state))
        })
    });

    group.finish();
}

/// Benchmark to track how fast it is to load contracts into memory
fn load_only(c: &mut Criterion) {
    let mut group = c.benchmark_group("loading");
    group.sample_size(10);

    let (_, native_state) = load_recording();

    group.bench_function("native", move |b| {
        b.iter(|| {
            let _ = CompiledNativeState::new(native_state.clone());
        })
    });

    group.finish();
}

criterion_group!(benches, preload, load_only);
criterion_main!(benches);
