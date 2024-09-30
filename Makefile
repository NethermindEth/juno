.DEFAULT_GOAL := help

.PHONY: vm

ifeq ($(VM_DEBUG),true)
    GO_TAGS = -tags vm_debug
    VM_TARGET = debug
else
    GO_TAGS =
    VM_TARGET = all
endif

ifeq ($(shell uname -s),Darwin)
	export CGO_LDFLAGS=-framework Foundation -framework SystemConfiguration

	# Set macOS deployment target in order to avoid linker warnings linke
	# "ld: warning: object file XX was built for newer macOS version (14.4) than being linked (14.0)"
	export MACOSX_DEPLOYMENT_TARGET=$(shell sw_vers --productVersion)

	# for test-race we need to pass -ldflags to fix linker warnings on macOS
	# see https://github.com/golang/go/issues/61229#issuecomment-1988965927
	TEST_RACE_LDFLAGS=-ldflags=-extldflags=-Wl,-ld_classic

	# Number of processes
	NPROCS = $(shell sysctl hw.ncpu  | grep -o '[0-9]\+')
else
	export CGO_LDFLAGS=-ldl -lm
	TEST_RACE_LDFLAGS=
	NPROCS = $(shell grep -c 'processor' /proc/cpuinfo)
endif

MAKEFLAGS += -j$(NPROCS)

rustdeps: vm core-rust compiler

juno: rustdeps ## compile
	@mkdir -p build
	@go build $(GO_TAGS) -a -ldflags="-X main.Version=$(shell git describe --tags)" -o build/juno ./cmd/juno/

juno-cached:
	@mkdir -p build
	@go build $(GO_TAGS) -ldflags="-X main.Version=$(shell git describe --tags)" -o build/juno ./cmd/juno/

vm:
	$(MAKE) -C vm/rust $(VM_TARGET)

core-rust:
	$(MAKE) -C core/rust $(VM_TARGET)

compiler:
	$(MAKE) -C starknet/rust $(VM_TARGET)

generate: ## generate
	mkdir -p mocks
	go generate ./...

clean-testcache:
	go clean -testcache

test: clean-testcache rustdeps ## tests
	go test $(GO_TAGS) ./...

test-cached: rustdeps ## tests with existing cache
	go test $(GO_TAGS) ./...

test-race: clean-testcache rustdeps
	go test $(GO_TAGS) ./... -race $(TEST_RACE_LDFLAGS)

benchmarks: rustdeps ## benchmarking
	go test $(GO_TAGS) ./... -run=^# -bench=. -benchmem

test-cover: clean-testcache rustdeps ## tests with coverage
	mkdir -p coverage
	go test $(GO_TAGS) -coverpkg=./... -coverprofile=coverage/coverage.out -covermode=atomic ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html

install-deps: | install-gofumpt install-mockgen install-golangci-lint## install some project dependencies

install-gofumpt:
	go install mvdan.cc/gofumpt@latest

install-mockgen:
	go install go.uber.org/mock/mockgen@latest

install-golangci-lint:
	@which golangci-lint || go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.60.1

lint: install-golangci-lint
	golangci-lint run

tidy: ## add missing and remove unused modules
	 go mod tidy

format: ## run go & rust formatters
	$(MAKE) -C vm/rust format
	$(MAKE) -C core/rust format
	$(MAKE) -C starknet/rust format
	gofumpt -l -w .

clean: ## clean project builds
	$(MAKE) -C vm/rust clean
	$(MAKE) -C core/rust clean
	$(MAKE) -C starknet/rust clean
	@rm -rf ./build

help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

feedernode: juno-cached
	./build/juno \
	--network=sepolia \
	--log-level=debug \
	--db-path="/Users/pnowosie/juno/snapshots/juno-sepolia" \
	--p2p \
	--p2p-feeder-node \
	--p2p-addr=/ip4/0.0.0.0/tcp/7777 \
	--p2p-private-key="5f6cdc3aebcc74af494df054876100368ef6126e3a33fa65b90c765b381ffc37a0a63bbeeefab0740f24a6a38dabb513b9233254ad0020c721c23e69bc820089" \
	--metrics-port=9090 \
	--pprof \
	--pprof-port=9095

node1: juno-cached
# todo remove rm before merge
	rm -rf ./p2p-dbs/node1/ && \
	./build/juno \
	--network=sepolia \
	--log-level=debug \
	--metrics \
	--db-path=./p2p-dbs/node1 \
	--p2p \
	--p2p-peers=/ip4/127.0.0.1/tcp/7777/p2p/12D3KooWLdURCjbp1D7hkXWk6ZVfcMDPtsNnPHuxoTcWXFtvrxGG \
	--p2p-addr=/ip4/0.0.0.0/tcp/7778 \
	--p2p-private-key="8aeffc26c3c371565dbe634c5248ae26f4fa5c33bc8f7328ac95e73fb94eaf263550f02449521f7cf64af17d248c5f170be46c06986a29803124c0819cb8fac3" \
	--metrics-port=9091 \
	--pprof \
	--pprof-port=9096 \
	--p2p-sync-mode="snap"

#	--p2p-peers=/ip4/127.0.0.1/tcp/7778/p2p/12D3KooWDQVMmK6cQrfFcWUoFF8Ch5vYegfwiP5Do2SFC2NAXeBk \

node2:
	./build/juno \
	--network=sepolia \
	--log-level=debug \
	--db-path=./p2p-dbs/node2 \
	--p2p \
	--p2p-peers=/ip4/127.0.0.1/tcp/7777/p2p/12D3KooWLdURCjbp1D7hkXWk6ZVfcMDPtsNnPHuxoTcWXFtvrxGG \
	--p2p-private-key="2d87e1d1c9d8dda1cf9a662de1978d2cd0b96e6ba390c75ded87c6c4fab657057fa782ae5977c3bd02d58281dccd16f2c26990d1f6c22f818a84edac97957348" \
	--metrics-port=9092

node3:
	./build/juno \
	--network=sepolia \
	--log-level=debug \
	--db-path=./p2p-dbs/node3 \
	--p2p \
	--p2p-peers=/ip4/127.0.0.1/tcp/7777/p2p/12D3KooWLdURCjbp1D7hkXWk6ZVfcMDPtsNnPHuxoTcWXFtvrxGG \
	--p2p-private-key="54a695e2a5d5717d5ba8730efcafe6f17251a1955733cffc55a4085fbf7f5d2c1b4009314092069ef7ca9b364ce3eb3072531c64dfb2799c6bad76720a5bdff0" \
	--metrics-port=9093

pathfinder: juno-cached
	./build/juno \
    	--network=sepolia \
    	--log-level=debug \
    	--db-path=./p2p-dbs/node-pathfinder \
    	--p2p \
    	--p2p-peers=/ip4/127.0.0.1/tcp/8888/p2p/12D3KooWF1JrZWQoBiBSjsFSuLbDiDvqcmJQRLaFQLmpVkHA9duk \
    	--p2p-private-key="54a695e2a5d5717d5ba8730efcafe6f17251a1955733cffc55a4085fbf7f5d2c1b4009314092069ef7ca9b364ce3eb3072531c64dfb2799c6bad76720a5bdff0" \
    	--metrics-port=9094
