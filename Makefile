export CC = clang
.DEFAULT_GOAL := help

juno: ## compile
	@mkdir -p build
	@go build -o build/juno ./cmd/juno/

all: juno

generate: ## generate
	@cd internal/db && $(MAKE) generate
	@cd pkg/felt && go generate ./...

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

install-deps: | install-gofumpt ## install some project dependencies

install-gofumpt:
	# install gofumpt
	go install mvdan.cc/gofumpt@latest

tidy: ## add missing and remove unused modules
	 go mod tidy

format: ## run go formatter
	gofumpt -l -w .

format-check: ## check formatting
	# assert `gofumpt -l` produces no output
	test ! $$(gofumpt -l . | tee /dev/stderr)

clean: ## clean project builds
	@rm -rf ./build

help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
