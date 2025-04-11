# Multi-stage Dockerfile for building Juno

ARG GO_VERSION=1.24.1
ARG RUST_VERSION=1.85.1
ARG JUNO_VERSION=unknown

# ==================================================================
# Stage 1: Build Rust libs with cargo-chef
# ==================================================================
FROM lukemathwalker/cargo-chef:0.1.71-rust-${RUST_VERSION}-slim-bookworm AS rust-builder

# Install Rust build tools (cbindgen) & C toolchain
RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential && \
    cargo install cbindgen --version 0.26.0 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

# --- Prepare shared dependencies using cargo-chef ---

# Virtual workspace for cargo-chef
RUN echo '[workspace]\nmembers = ["starknet_compiler", "core_rust", "vm_rust"]' > Cargo.toml

# Copy manifests for cache
COPY starknet/compiler/rust/Cargo.toml starknet/compiler/rust/Cargo.lock ./starknet_compiler/
COPY core/rust/Cargo.toml core/rust/Cargo.lock ./core_rust/
COPY vm/rust/Cargo.toml vm/rust/Cargo.lock ./vm_rust/

# Prepare cargo-chef recipe (requires dummy lib.rs for libs)
RUN mkdir -p ./starknet_compiler/src ./core_rust/src ./vm_rust/src && \
    touch ./starknet_compiler/src/lib.rs ./core_rust/src/lib.rs ./vm_rust/src/lib.rs && \
    cargo chef prepare --recipe-path recipe.json

# Cook dependencies (cached layer)
RUN cargo chef cook --release --recipe-path recipe.json

# --- Build the actual projects ---

# Copy source
COPY starknet/compiler/rust/ ./starknet_compiler/
COPY core/rust/ ./core_rust/
COPY vm/rust/ ./vm_rust/

# Args for package/library names
ARG STARKNET_COMPILER_PKG_NAME=juno-starknet-compiler-rs
ARG CORE_RUST_PKG_NAME=juno-starknet-core-rs
ARG VM_RUST_PKG_NAME=juno-starknet-rs
ARG STARKNET_COMPILER_LIB_NAME=juno_starknet_compiler_rs
ARG CORE_RUST_LIB_NAME=juno_starknet_core_rs
ARG VM_RUST_LIB_NAME=juno_starknet_rs

# Build Rust libs & generate C headers
RUN cargo build --release --package ${STARKNET_COMPILER_PKG_NAME} && \
    cbindgen --crate ${STARKNET_COMPILER_PKG_NAME} --output target/release/${STARKNET_COMPILER_PKG_NAME}.h

RUN cargo build --release --package ${CORE_RUST_PKG_NAME} && \
    cbindgen --crate ${CORE_RUST_PKG_NAME} --output target/release/${CORE_RUST_PKG_NAME}.h

RUN cargo build --release --package ${VM_RUST_PKG_NAME} && \
    cbindgen --crate ${VM_RUST_PKG_NAME} --output target/release/${VM_RUST_PKG_NAME}.h

# --- Collect build artifacts ---
RUN mkdir /artifacts && \
    cp target/release/lib${STARKNET_COMPILER_LIB_NAME}.a /artifacts/ && \
    cp target/release/${STARKNET_COMPILER_PKG_NAME}.h /artifacts/ && \
    \
    cp target/release/lib${CORE_RUST_LIB_NAME}.a /artifacts/ && \
    cp target/release/${CORE_RUST_PKG_NAME}.h /artifacts/ && \
    \
    cp target/release/lib${VM_RUST_LIB_NAME}.a /artifacts/ && \
    cp target/release/${VM_RUST_PKG_NAME}.h /artifacts/


# ==================================================================
# Stage 2: Build Go application
# ==================================================================
FROM golang:${GO_VERSION}-bookworm AS go-builder

ARG JUNO_VERSION

# Install CGO dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        libjemalloc-dev \
        libbz2-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Cache Go dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source
COPY . .

# Args needed for artifact paths
ARG STARKNET_COMPILER_PKG_NAME=juno-starknet-compiler-rs
ARG CORE_RUST_PKG_NAME=juno-starknet-core-rs
ARG VM_RUST_PKG_NAME=juno-starknet-rs
ARG STARKNET_COMPILER_LIB_NAME=juno_starknet_compiler_rs
ARG CORE_RUST_LIB_NAME=juno_starknet_core_rs
ARG VM_RUST_LIB_NAME=juno_starknet_rs

# Copy Rust libs (.a)
COPY --from=rust-builder /artifacts/lib${STARKNET_COMPILER_LIB_NAME}.a /app/starknet/compiler/rust/target/release/
COPY --from=rust-builder /artifacts/lib${CORE_RUST_LIB_NAME}.a /app/core/rust/target/release/
COPY --from=rust-builder /artifacts/lib${VM_RUST_LIB_NAME}.a /app/vm/rust/target/release/

# Copy Rust headers (.h)
COPY --from=rust-builder /artifacts/${STARKNET_COMPILER_PKG_NAME}.h /app/starknet/compiler/
COPY --from=rust-builder /artifacts/${CORE_RUST_PKG_NAME}.h /app/core/
COPY --from=rust-builder /artifacts/${VM_RUST_PKG_NAME}.h /app/vm/

# Index static libs (for linker)
RUN ranlib /app/starknet/compiler/rust/target/release/lib${STARKNET_COMPILER_LIB_NAME}.a && \
    ranlib /app/core/rust/target/release/lib${CORE_RUST_LIB_NAME}.a && \
    ranlib /app/vm/rust/target/release/lib${VM_RUST_LIB_NAME}.a

# Configure CGO
ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-ljemalloc -lm -lpthread -ldl"

# Build Go binary (passing version, stripping symbols)
RUN go build -ldflags="-X main.Version=${JUNO_VERSION} -s -w" -o /app/juno ./cmd/juno/


# ==================================================================
# Stage 3: Final runtime image
# ==================================================================
FROM debian:bookworm-slim AS final

# Install minimal runtime dependencies
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        ca-certificates \
        libjemalloc2 \
        libbz2-1.0 \
        tzdata \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Copy final binary
COPY --from=go-builder /app/juno /usr/local/bin/

ENTRYPOINT ["juno"]