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

# --- Debian image ---
# The earlier runtime image with a shell and utilities. Releases publish it as nethermind/juno:<tag>
# for users that cannot move to the distroless image yet. Build it with --target debian.
FROM debian:bookworm-slim AS debian

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

# --- Runtime base stage ---
# Builds a Wolfi filesystem with glibc and no shell or package manager from the packages pinned in
# docker/apko.lock.json. It runs on the build platform, so a cross-arch build needs no emulation.
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS base

# Keep in sync with APKO_VERSION in the Makefile.
ARG APKO_VERSION=v1.4.11
ARG TARGETARCH

# The golang image has python3 only as a dependency of mercurial. Install it explicitly for the
# manifest lookup below.
RUN apt-get update && apt-get install -y --no-install-recommends python3 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN go install chainguard.dev/apko@${APKO_VERSION}

WORKDIR /base
COPY docker/apko.yaml docker/apko.lock.json ./

# apko writes an image tarball with one layer. Extract that layer as the root filesystem.
RUN apko build --arch ${TARGETARCH} --lockfile apko.lock.json --sbom=false apko.yaml juno-base image.tar \
    && tar -xf image.tar \
    && layer="$(python3 -c 'import json, sys; m = json.load(open("manifest.json")); \
        (len(m) == 1 and len(m[0]["Layers"]) == 1) or sys.exit("apko manifest.json: want 1 image with 1 layer"); \
        print(m[0]["Layers"][0])')" \
    && mkdir /rootfs \
    && tar -xzf "$layer" -C /rootfs

# --- Distroless image ---
# The default target. Releases publish it as nethermind/juno:<tag>-distroless.
FROM scratch AS distroless

COPY --from=base /rootfs/ /
COPY --from=builder /app/build/juno /usr/local/bin/

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

LABEL org.opencontainers.image.title="juno" \
      org.opencontainers.image.vendor="Nethermind" \
      org.opencontainers.image.source="https://github.com/NethermindEth/juno" \
      org.opencontainers.image.licenses="Apache-2.0"

ENTRYPOINT ["juno"]
