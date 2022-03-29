FROM golang:1.18-alpine as build

WORKDIR /app

# Install Alpine Dependencies
RUN apk update && apk upgrade && apk add --update alpine-sdk && \
    apk add --no-cache bash git openssh make cmake

# Copy all source code
COPY . .
RUN go mod download

RUN make compile

FROM alpine:3.15

WORKDIR /app

COPY --from=build /app/build/juno /app/juno

ENTRYPOINT ["/app/juno"]

