# syntax=docker/dockerfile:1.7.0

# --- Builder stage ---
FROM golang:1.24-bookworm AS builder

ARG RUST_VERSION=1.86.0

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
COPY starknet/compiler/rust/Cargo.toml starknet/compiler/rust/Cargo.lock starknet/compiler/rust/
COPY vm/rust/Cargo.toml vm/rust/Cargo.lock vm/rust/
COPY core/rust/Cargo.toml core/rust/Cargo.lock core/rust/

# Touch empty lib.rs to satisfy Cargo
RUN mkdir -p \
    starknet/compiler/rust/src \
    vm/rust/src \
    core/rust/src && \
    touch starknet/compiler/rust/src/lib.rs \
            vm/rust/src/lib.rs \
            core/rust/src/lib.rs

# Pre-fetch dependencies
RUN cargo build --manifest-path=starknet/compiler/rust/Cargo.toml
RUN cargo build --manifest-path=vm/rust/Cargo.toml
RUN cargo build --manifest-path=core/rust/Cargo.toml

# Build Rust libraries
COPY Makefile ./
COPY starknet/compiler/rust/ ./starknet/compiler/rust/
RUN make compiler

COPY vm/rust/ ./vm/rust/
RUN make vm

COPY core/rust/ ./core/rust/
RUN make core-rust

# Copy go mod files and download deps
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build with make juno (setting VM_DEBUG=false to use release builds)
ENV VM_DEBUG=false
RUN make juno-cached

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