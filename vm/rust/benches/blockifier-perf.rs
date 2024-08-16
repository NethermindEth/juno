use criterion::{black_box, criterion_group, criterion_main, Criterion};
use juno_starknet_rs::serstate;

fn criterion_benchmark(c: &mut Criterion) {
    let mut group = c.benchmark_group("sample_size");
    group.sample_size(10);
    group.bench_function("block", |b| b.iter(|| serstate::run(633333)));
}

criterion_group!(benches, criterion_benchmark);
criterion_main!(benches);
