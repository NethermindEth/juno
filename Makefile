.DEFAULT_GOAL := help

.PHONY: vm

juno: vm ## compile
	@mkdir -p build
	@go build -a -ldflags="-X main.Version=$(shell git describe --tags)" -o build/juno ./cmd/juno/

vm:
	$(MAKE) -C vm/rust all

generate: ## generate
	mkdir -p mocks
	go generate ./...

clean-testcache:
	go clean -testcache

test: clean-testcache vm ## tests
	go test ./...

test-cached: vm ## tests with existing cache
	go test ./...

test-race: clean-testcache vm
	go test ./... -race

benchmarks: vm ## benchmarking
	go test ./... -run=^# -bench=. -benchmem

test-cover: vm ## tests with coverage
	mkdir -p coverage
	go test -coverpkg=./... -coverprofile=coverage/coverage.out -covermode=atomic ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html

install-deps: | install-gofumpt install-mockgen install-golangci-lint## install some project dependencies

install-gofumpt:
	go install mvdan.cc/gofumpt@latest

install-mockgen:
	go install github.com/golang/mock/mockgen@latest

install-golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.3

lint:
	@which golangci-lint || make install-golangci-lint
	golangci-lint run

tidy: ## add missing and remove unused modules
	 go mod tidy

format: ## run go formatter
	gofumpt -l -w .

clean: ## clean project builds
	$(MAKE) -C vm/rust clean
	@rm -rf ./build

help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
