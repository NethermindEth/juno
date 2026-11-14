# Go 1.27.4 and later reject cgo linker inputs that contain LLVM bitcode. The objects of the
# prebuilt Rust standard library contain bitcode, thus each Rust staticlib contains it too.
# The linker does not use these sections. Write a copy without them for cgo.
LIB = libjuno_starknet_rs.a
# llvm-objcopy comes from the llvm-tools component in rust-toolchain.toml.
OBJCOPY ?= $(shell rustc --print sysroot)/lib/rustlib/$(shell rustc -vV | sed -n 's/host: //p')/bin/llvm-objcopy
ifeq ($(shell uname -s),Darwin)
	BITCODE_SECTIONS = --remove-section=__LLVM,__bitcode --remove-section=__LLVM,__cmdline
else
	BITCODE_SECTIONS = --remove-section=.llvmbc --remove-section=.llvmcmd
endif

define strip-bitcode
	@mkdir -p target/$(1)/cgo
	"$(OBJCOPY)" $(BITCODE_SECTIONS) target/$(1)/$(LIB) target/$(1)/cgo/$(LIB)
endef

all:
	cargo build --release
	$(call strip-bitcode,release)

debug:
	cargo build
	$(call strip-bitcode,debug)

check:
	cargo check --release

lint:
	cargo clippy -- -D warnings

fix:
	cargo fix --lib -p juno-starknet-rs --allow-dirty --allow-staged

clean:
	cargo clean

format:
	cargo fmt

format-check:
	cargo fmt --check

install-rustfmt:
	cargo install rustfmt