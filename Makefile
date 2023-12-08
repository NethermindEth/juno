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

juno: vm core-rust ## compile
	@mkdir -p build
	@go build $(GO_TAGS) -a -ldflags="-X main.Version=$(shell git describe --tags)" -o build/juno ./cmd/juno/

vm:
	$(MAKE) -C vm/rust $(VM_TARGET)

core-rust:
	$(MAKE) -C core/rust $(VM_TARGET)

generate: ## generate
	mkdir -p mocks
	go generate ./...

clean-testcache:
	go clean -testcache

test: clean-testcache vm core-rust ## tests
	go test $(GO_TAGS) ./...

test-cached: vm core-rust ## tests with existing cache
	go test $(GO_TAGS) ./...

test-race: clean-testcache vm core-rust
	go test $(GO_TAGS) ./... -race

benchmarks: vm core-rust ## benchmarking
	go test $(GO_TAGS) ./... -run=^# -bench=. -benchmem

test-cover: vm core-rust ## tests with coverage
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
	@rm -rf ./build

help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

dump:
	@printenv