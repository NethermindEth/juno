all:
	cargo build --release

tracing:
	cargo build --release --features tracing

debug:
	cargo build

check:
	cargo check --release

lint:
	cargo clippy

fix:
	cargo fix --lib -p juno-starknet-rs --allow-dirty --allow-staged

clean:
	cargo clean

format:
	cargo fmt

install-rustfmt:
	cargo install rustfmt

