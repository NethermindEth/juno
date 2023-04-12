# Stage 1: Build golang dependencies and binaries
FROM alpine:3.18 AS build

# Install Alpine Dependencies
RUN apk update && \
    apk add build-base clang upx cargo go pkgconfig libressl-dev

WORKDIR /app

# Copy source code
COPY . .

# Build the project
RUN make juno

# Compress the executable with UPX
RUN upx /app/build/juno

# Stage 2: Build Docker image
FROM alpine:3.18 AS runtime

# Install runtime dependencies
RUN apk update --no-cache && \
    apk add --no-cache libgcc

COPY --from=build /app/build/juno /usr/local/bin/

ENTRYPOINT ["juno"]