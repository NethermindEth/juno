# syntax=docker/dockerfile:1.7.0

# --- Builder stage ---
FROM golang:1.27-bookworm AS builder

ARG RUST_VERSION=1.94.1

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl gcc make git build-essential \
    libjemalloc-dev libbz2-dev pkg-config \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Remove the shared jemalloc so that -ljemalloc links libjemalloc.a. The final image then needs
# no jemalloc package, and the Debian libjemalloc.so.2 does not pull in libstdc++.
RUN rm /usr/lib/$(gcc -dumpmachine)/libjemalloc.so

# Install Rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y --default-toolchain ${RUST_VERSION} --profile minimal
ENV PATH="/root/.cargo/bin:${PATH}"

WORKDIR /app

# Copy Cargo manifests for dependency caching
COPY Makefile ./
COPY vm/rust/Makefile vm/rust/Cargo.toml vm/rust/Cargo.lock vm/rust/

# Touch empty lib.rs to satisfy Cargo
RUN mkdir -p vm/rust/src && touch vm/rust/src/lib.rs

# Pre-build Rust dependencies, then clean to force cargo to only cache the dependencies and rebuild the application.
# See: https://github.com/rust-lang/cargo/issues/9598
RUN make rustdeps
RUN cargo clean --release --manifest-path vm/rust/Cargo.toml --package juno-starknet-rs

# Copy go mod files and download deps
COPY go.mod go.sum ./
COPY starknet-p2p-specs/go.mod starknet-p2p-specs/go.mod
RUN go mod download

# Copy the rest of the source
COPY . .

# Build with make juno
RUN make juno

# --- Final stage ---
# Wolfi glibc runtime with no shell or package manager. The free tier publishes only the latest
# tag, so the digest pins it and Dependabot bumps it.
FROM cgr.dev/chainguard/glibc-dynamic:latest@sha256:2a3f1ec8a825dfbb1211b5835a9b8b3fb8d76c7bd22a189b1f84157ed2028293 AS final

# The base image runs as uid 65532. Data directories that earlier images made are owned by root.
USER root

COPY --from=builder /app/build/juno /usr/local/bin/

ENTRYPOINT ["juno"]
