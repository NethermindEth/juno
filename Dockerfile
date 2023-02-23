# Stage 1: Build golang dependencies and binaries
FROM golang:1.19-alpine AS build

# Install Alpine Dependencies
RUN apk update && \
    apk add build-base clang upx

WORKDIR /app

# Copy source code
COPY . .

# Build the project
RUN make juno

# Compress the executable with UPX
RUN upx /app/build/juno

# Stage 2: Build Docker image
FROM alpine:3.14 AS runtime

COPY --from=build /app/build/juno /usr/local/bin/

ENTRYPOINT ["juno"]