export CC = clang
.DEFAULT_GOAL := help

juno: ## compile
	@mkdir -p build
	@go build -ldflags="-X main.Version=$(shell git describe --tags)" -o build/juno ./cmd/juno/

generate: ## generate
	mkdir -p mocks
	go generate ./...

clean-testcache:
	go clean -testcache

test: clean-testcache ## tests
	go test ./...

test-race: clean-testcache
	go test ./... -race

benchmarks: ## benchmarking
	go test ./... -run=^# -bench=. -benchmem

test-cover: ## tests with coverage
	mkdir -p coverage
	go test -coverpkg=./... -coverprofile=coverage/coverage.out -covermode=atomic ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html

install-deps: | install-mockgen install-golangci-lint## install some project dependencies

install-gofumpt:
	go install mvdan.cc/gofumpt@latest

install-mockgen:
	go install github.com/golang/mock/mockgen@latest

install-golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.2

lint: install-golangci-lint
	golangci-lint run

tidy: ## add missing and remove unused modules
	 go mod tidy

format: ## run go formatter
	gofumpt -l -w .

clean: ## clean project builds
	@rm -rf ./build

help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
