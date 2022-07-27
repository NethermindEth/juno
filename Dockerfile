# syntax=docker/dockerfile:1

FROM golang:1.18-alpine as build

WORKDIR /app

# Install Alpine Dependencies
RUN apk update && apk upgrade && apk add --update alpine-sdk && \
    apk add --no-cache bash git openssh make cmake clang

# Set c compiler to CLANG, only working
ENV CC=clang

# Copy all source code
COPY . .

# Install golang deps
RUN go mod download

# Compile juno
RUN make compile

# Get the Python dependencies running in a reasonable size container
FROM python:3.7.13-alpine as pydeps

RUN apk update && apk upgrade && apk add --update alpine-sdk && \
    apk add --no-cache clang

# Set c compiler to CLANG, only working
ENV CC=clang

RUN addgroup --system appgroup && adduser --system app 
USER app
ENV HOME /home/app

COPY requirements.txt ./requirements.txt
# RUN pip install -r requirements.txt -q

# Copy over file from the build 
COPY --from=build /app/build/juno /home/app/juno

EXPOSE  8080
ENTRYPOINT ["/home/app/juno"]
