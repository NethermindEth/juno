FROM golang:1.18-alpine as build

WORKDIR /app

# Install Alpine Dependencies
RUN apk update && apk upgrade && apk add --update alpine-sdk && \
    apk add --no-cache bash git openssh make cmake clang

# Copy all source code
COPY . .
RUN go mod download

RUN make compile

FROM python:3.7.13-alpine

WORKDIR /app

RUN addgroup -S appgroup && adduser -S app -G appgroup
USER app
ENV HOME /home/app

COPY --from=build /app/build/juno /home/app/juno

EXPOSE  8080
ENTRYPOINT ["/home/app/juno"]

