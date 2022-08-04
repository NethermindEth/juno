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

COPY --from=build /app/build/juno /home/app/juno
COPY --from=build /app/requirements.txt /req/requirements.txt

RUN apk update && apk upgrade && apk add --update alpine-sdk && \
    apk add --no-cache bash git openssh make cmake clang && pip install -r /req/requirements.txt

RUN addgroup -S appgroup && adduser -S app -G appgroup
USER app
ENV HOME /home/app

EXPOSE  8080
ENTRYPOINT ["/home/app/juno"]

