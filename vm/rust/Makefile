all:
	cargo build --release

debug:
	cargo build && \
	mkdir -p target/release/ && \
	mv -n target/debug/libjuno_starknet_rs.* target/release/

check:
	cargo check --release

clean:
	cargo clean