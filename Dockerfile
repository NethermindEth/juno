# syntax=docker/dockerfile:1.7.0

# --- Builder stage ---
FROM golang:1.25-bookworm AS builder

ARG RUST_VERSION=1.88.0

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl gcc make git build-essential \
    libjemalloc-dev libbz2-dev pkg-config \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Install Rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y --default-toolchain ${RUST_VERSION} --profile minimal
ENV PATH="/root/.cargo/bin:${PATH}"

WORKDIR /app

# Copy Cargo manifests for dependency caching
COPY Makefile ./
COPY starknet/compiler/rust/Makefile starknet/compiler/rust/Cargo.toml starknet/compiler/rust/Cargo.lock starknet/compiler/rust/
COPY vm/rust/Makefile vm/rust/Cargo.toml vm/rust/Cargo.lock vm/rust/
COPY core/rust/Makefile core/rust/Cargo.toml core/rust/Cargo.lock core/rust/

# Touch empty lib.rs to satisfy Cargo
RUN mkdir -p \
    starknet/compiler/rust/src \
    vm/rust/src \
    core/rust/src && \
    touch starknet/compiler/rust/src/lib.rs \
            vm/rust/src/lib.rs \
            core/rust/src/lib.rs

# Pre-build Rust dependencies, then clean to force cargo to only cache the dependencies and rebuild the application.
# See: https://github.com/rust-lang/cargo/issues/9598
RUN make rustdeps
RUN cargo clean --release --manifest-path starknet/compiler/rust/Cargo.toml --package juno-starknet-compiler-rs && \
    cargo clean --release --manifest-path vm/rust/Cargo.toml --package juno-starknet-rs && \
    cargo clean --release --manifest-path core/rust/Cargo.toml --package juno-starknet-core-rs

# Copy go mod files and download deps
COPY go.mod go.sum ./
COPY starknet-p2pspecs/go.mod starknet-p2pspecs/go.mod
RUN go mod download

# Copy the rest of the source
COPY . .

# Build with make juno
RUN make juno

# --- Final stage ---
FROM debian:bookworm-slim AS final

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    gawk \
    grep \
    libjemalloc-dev \
    libjemalloc2 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/build/juno /usr/local/bin/

ENTRYPOINT ["juno"]
