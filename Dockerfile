# syntax=docker/dockerfile:1.4

# Stage 1: Build Go + Rust project
FROM golang:1.24-bookworm AS build

# Install Rust 1.85.1
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -q -y --default-toolchain 1.85.1

# Install build dependencies
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    git \
    build-essential \
    libjemalloc-dev \
    libbz2-dev \
    pkg-config && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Setup environment
ENV PATH=$PATH:/root/.cargo/bin \
    GOPROXY=https://proxy.golang.org,direct \
    MAKEFLAGS="-j$(nproc)"

WORKDIR /app

# Copy Go dependency files first for better caching
COPY go.mod go.sum ./

# Download Go dependencies
RUN go mod download

# Copy only Cargo.toml files and Makefiles first for better Rust dependency caching
COPY vm/rust/Cargo.toml vm/rust/Cargo.lock vm/rust/
COPY vm/rust/Makefile vm/rust/
COPY core/rust/Cargo.toml core/rust/Cargo.lock core/rust/
COPY core/rust/Makefile core/rust/
COPY starknet/compiler/rust/Cargo.toml starknet/compiler/rust/Cargo.lock starknet/compiler/rust/
COPY starknet/compiler/rust/Makefile starknet/compiler/rust/

# Create dummy source files to satisfy Cargo
RUN mkdir -p vm/rust/src core/rust/src starknet/compiler/rust/src && \
    touch vm/rust/src/lib.rs core/rust/src/lib.rs starknet/compiler/rust/src/lib.rs

# Fetch Rust dependencies so they can be cached
RUN cd vm/rust && . /root/.cargo/env && cargo fetch && \
    cd /app/core/rust && . /root/.cargo/env && cargo fetch && \
    cd /app/starknet/compiler/rust && . /root/.cargo/env && cargo fetch

# Now copy the actual Rust source code
COPY vm/rust/src/ vm/rust/src/
COPY core/rust/src/ core/rust/src/
COPY starknet/compiler/rust/src/ starknet/compiler/rust/src/

# Copy main Makefile to build Rust components
COPY Makefile ./

# Build Rust components
RUN . /root/.cargo/env && \
    make vm && \
    make core-rust && \
    make compiler

# Now copy the rest of the source code
COPY . .

# Build the Go part of the project (which will now use the pre-built Rust libraries)
RUN GO_LDFLAGS="-w -s" \
    make juno

# Stage 2: Minimal runtime
FROM debian:bookworm-slim AS runtime

# Install runtime deps
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    grep \
    gawk \
    libjemalloc-dev \
    libjemalloc2 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Copy binary from builder
COPY --from=build /app/build/juno /usr/local/bin/

# Set executable permissions
RUN chmod +x /usr/local/bin/juno

# Create data directory
RUN mkdir -p /var/lib/juno

ENTRYPOINT ["/usr/local/bin/juno"]