# Stage 1: Build golang dependencies and binaries
FROM golang:1.18-alpine as go_builder

WORKDIR /app

# Install Alpine Dependencies
RUN apk update && apk upgrade && apk add --update alpine-sdk && \
    apk add --no-cache bash git openssh make cmake clang

# Copy all source code
COPY . .
RUN go mod download

RUN make juno

# Stage 2: Build Python dependencies for Cairo
FROM python:3.7.13-alpine as py_builder

WORKDIR /app

COPY ./requirements.txt /req/requirements.txt

# Install Python dependencies
RUN apk update && apk upgrade && apk add --update alpine-sdk && apk add --no-cache gmp-dev cmake gcc g++ linux-headers

# Install project dependencies
RUN pip --disable-pip-version-check install -r /req/requirements.txt
ENV PY_PATH=/usr/local/lib/python3.7/
RUN find ${PY_PATH} -type d -a -name test -exec rm -rf '{}' + \
    && find ${PY_PATH} -type d -a -name tests  -exec rm -rf '{}' + \
    && find ${PY_PATH} -type f -a -name '*.pyc' -exec rm -rf '{}' + \
    && find ${PY_PATH} -type f -a -name '*.pyo' -exec rm -rf '{}' +

# Stage 3: Build Docker image
FROM python:3.7.13-alpine as runtime

COPY --from=py_builder /usr/local/lib/python3.7/site-packages /usr/local/lib/python3.7/site-packages
COPY --from=go_builder /app/build/juno /home/app/juno

ENV HOME /home/app
RUN addgroup -S appgroup && adduser -S app -G appgroup
RUN apk add --no-cache gmp-dev gcc g++ linux-headers
USER app

EXPOSE 6060
EXPOSE 9090
ENTRYPOINT ["/home/app/juno"]


