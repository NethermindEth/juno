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

RUN addgroup -S appgroup && adduser -S app -G appgroup
USER app
ENV HOME /home/app

COPY --from=build /app/build/juno /home/app/juno

EXPOSE  8080
ENTRYPOINT ["/home/app/juno"]

