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
endif

rustdeps: vm core-rust compiler

juno: rustdeps ## compile
	@mkdir -p build
	@go build $(GO_TAGS) -a -ldflags="-X main.Version=$(shell git describe --tags)" -o build/juno ./cmd/juno/

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
	go test $(GO_TAGS) ./... -race

benchmarks: rustdeps ## benchmarking
	go test $(GO_TAGS) ./... -run=^# -bench=. -benchmem

test-cover: rustdeps ## tests with coverage
	mkdir -p coverage
	go test $(GO_TAGS) -coverpkg=./... -coverprofile=coverage/coverage.out -covermode=atomic ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html

install-deps: | install-gofumpt install-mockgen install-golangci-lint## install some project dependencies

install-gofumpt:
	go install mvdan.cc/gofumpt@latest

install-mockgen:
	go install go.uber.org/mock/mockgen@latest

install-golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2

lint:
	@which golangci-lint || make install-golangci-lint
	golangci-lint run

tidy: ## add missing and remove unused modules
	 go mod tidy

format: ## run go formatter
	gofumpt -l -w .

clean: ## clean project builds
	$(MAKE) -C vm/rust clean
	$(MAKE) -C core/rust clean
	$(MAKE) -C starknet/rust clean
	@rm -rf ./build

help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

bootnode: juno
	./build/juno --network=sepolia --log-level=debug --p2p --p2p-bootnode --p2p-addr /ip4/0.0.0.0/tcp/7777 \
    --p2p-private-key \
	"5f6cdc3aebcc74af494df054876100368ef6126e3a33fa65b90c765b381ffc37a0a63bbeeefab0740f24a6a38dabb513b9233254ad0020c721c23e69bc820089"

.PHONY: node
node: juno
	./build/juno --network=sepolia --log-level=debug --p2p \
	--p2p-boot-peers /ip4/127.0.0.1/tcp/7777/p2p/12D3KooWLdURCjbp1D7hkXWk6ZVfcMDPtsNnPHuxoTcWXFtvrxGG \
	--db-path ./juno2