# Stage 1: Build golang dependencies and binaries
FROM ubuntu:24.04 AS build

ARG VM_DEBUG

# Install Alpine Dependencies
# removal is temp. to fix https://github.com/orgs/community/discussions/120966
RUN rm /etc/apt/sources.list.d/microsoft-prod.list && \
    apt-get update && \
    apt-get install build-essential cargo git golang upx-ucl libjemalloc-dev libjemalloc2 -y

WORKDIR /app

# Copy source code
COPY . .

# Build the project
RUN VM_DEBUG=${VM_DEBUG} make juno

# Compress the executable with UPX
RUN upx-ucl /app/build/juno

# Stage 2: Build Docker image
FROM ubuntu:23.10 AS runtime

RUN apt-get update && apt-get install -y ca-certificates curl gawk grep libjemalloc-dev libjemalloc2

COPY --from=build /app/build/juno /usr/local/bin/

ENTRYPOINT ["juno"]