use std::{env, fs::File, path::PathBuf, time::Instant};

use blockifier::state::cached_state::CachedState;
use criterion::{black_box, criterion_group, criterion_main, Criterion};
use juno_starknet_rs::{
    recorded_state::{self, CompiledNativeState, NativeState},
    VMArgs,
};

enum Executor {
    Native,
    VM,
}

fn load_recording(exe: Executor) -> (u64, VMArgs, NativeState) {
    // This is a workaround of not being able to supply arguments over the command line.
    let block_number: u64 = env::var("BENCH_BLOCK")
        .expect(
            "To select the block to benchmark add BENCH_BLOCK=block_number infront of the command",
        )
        .parse()
        .expect("failed to parse BENCH_BLOCK");

    let record_directory = env::var("JUNO_RECORD_DIR").expect("JUNO_RECORD_DIR has not been set");

    let exe_dir = match exe {
        Executor::Native => "native",
        Executor::VM => "vm",
    };

    // TODO(xrvdg) Create directory paths without mutability
    let mut record_directory: PathBuf = record_directory.into();
    record_directory.push(exe_dir);

    let mut args_path: PathBuf = record_directory.clone();
    args_path.push(format!("{}.args.cbor", block_number));
    let args_file = File::open(args_path).unwrap();
    let vm_args: VMArgs = ciborium::from_reader(args_file).unwrap();

    let mut state_path: PathBuf = record_directory.clone();
    state_path.push(format!("{}.state.cbor", block_number));
    let state_file = File::open(state_path).unwrap();

    let native_state: NativeState = ciborium::from_reader(state_file).unwrap();

    (block_number, vm_args, native_state)
}

/// This benchmark preloads the compiled contracts and then starts the benchmark.
fn preload(c: &mut Criterion) {
    let (block_number, native_vm_args, native_state) = load_recording(Executor::Native);
    let (_block_number, vm_vm_args, vm_state) = load_recording(Executor::VM);

    let mut group = c.benchmark_group(format!("preload/{block_number}"));
    group.sample_size(10);

    // todo(xrvdg) how to ensure this isn't run when this benchmark is filtered out?
    // note: this is the recommended way of setting up data before a benchmark according to criterion's documentation.
    println!("Setup: loading contracts into memory");
    let start = Instant::now();
    // Either loads from disk or compiles
    let compiled_native_state = CompiledNativeState::new(native_state);
    println!(
        "Setup: finished loading contracts ({} ms)",
        start.elapsed().as_millis()
    );

    let compiled_vm_state = CompiledNativeState::new(vm_state);

    group.bench_function("native", |b| {
        b.iter(|| {
            // The clone is negligible compared to the run
            let mut cached_compiled_native_state = CachedState::new(compiled_native_state.clone());
            let mut vm_args = native_vm_args.clone();
            recorded_state::run(
                black_box(&mut vm_args),
                black_box(&mut cached_compiled_native_state),
            );
        })
    });

    group.bench_function("vm", |b| {
        b.iter(|| {
            // The clone is negligible compared to the run
            let mut cached_compiled_vm_state = CachedState::new(compiled_vm_state.clone());
            let mut vm_args = vm_vm_args.clone();
            recorded_state::run(
                black_box(&mut vm_args),
                black_box(&mut cached_compiled_vm_state),
            );
        })
    });

    group.finish();
}

/// This benchmark simulates a cold boot of the blockifier with none of the contracts loaded in.
///
/// This should be the same as the other two benchmarks combined and is left here to be able to verify that quickly.
// #[allow(dead_code)]
// fn cold(c: &mut Criterion) {
//     let (block_number, mut vm_args, native_state) = load_recording();

//     let mut group = c.benchmark_group(format!("cold/{block_number}"));
//     group.sample_size(10);

//     group.bench_function("native", move |b| {
//         b.iter(|| {
//             // The clone is negligible compared to the run
//             let mut cached_native_state = CachedState::new(native_state.clone());

//             recorded_state::run(black_box(&mut vm_args), black_box(&mut cached_native_state))
//         })
//     });

//     group.finish();
// }

/// Benchmark to track how fast it is to load contracts into memory
// fn loading(c: &mut Criterion) {
//     let (block_number, _, native_state) = load_recording();

//     let mut group = c.benchmark_group(format!("loading/{block_number}"));
//     group.sample_size(10);

//     group.bench_function("native", move |b| {
//         b.iter(|| {
//             let _ = CompiledNativeState::new(native_state.clone());
//         })
//     });

//     group.finish();
// }

criterion_group!(benches, preload);
criterion_main!(benches);
