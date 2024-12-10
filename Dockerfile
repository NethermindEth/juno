# Stage 1: Build golang dependencies and binaries
FROM ubuntu:25.04 AS build

ARG VM_DEBUG


RUN apt-get -qq update && \
    apt-get -qq install curl build-essential git golang upx-ucl libjemalloc-dev libjemalloc2 libbz2-dev
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -q -y

WORKDIR /app

# Copy source code
COPY . .

# Build the project
RUN bash -c 'source ~/.cargo/env && VM_DEBUG=${VM_DEBUG} make juno'

# Compress the executable with UPX
RUN upx-ucl /app/build/juno

# Stage 2: Build Docker image
FROM ubuntu:25.04 AS runtime

RUN apt-get update && apt-get install -y ca-certificates curl gawk grep libjemalloc-dev libjemalloc2

COPY --from=build /app/build/juno /usr/local/bin/

ENTRYPOINT ["juno"]