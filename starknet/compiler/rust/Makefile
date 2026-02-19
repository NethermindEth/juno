all:
	cargo build --release

debug:
	cargo build

check:
	cargo check --release

lint:
	cargo clippy -- -D warnings

fix:
	cargo fix --lib -p juno-starknet-compiler-rs --allow-dirty --allow-staged

clean:
	cargo clean

format:
	cargo fmt

format-check:
	cargo fmt --check

install-rustfmt:
	cargo install rustfmt