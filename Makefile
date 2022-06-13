export CC = clang
.DEFAULT_GOAL 	:= help

compile: ## compile:
	@mkdir -p build
	@go build -o build/juno cmd/juno/main.go
	@go build -o build/juno-cli cmd/juno-cli/main.go

run: ## run
	@./build/juno

all: compile run ## build and run

generate:
	@cd internal/db && $(MAKE) generate

test: ## tests
	go test ./...

benchmarks: ## Benchmarking
	go test ./... -bench=.

test-cover: ## tests with coverage
	mkdir -p coverage
	go test -coverprofile=coverage/coverage.out -covermode=count ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html

install-deps: | install-courtney install-gofumpt ## Install some project dependencies

install-courtney:
	# install courtney fork
	git clone https://github.com/stdevMac/courtney
	(cd courtney && go get  ./... && go build courtney.go)
	go get ./...


install-gofumpt:
	# install gofumpt
	go install mvdan.cc/gofumpt@latest

codecov-test:
	mkdir -p coverage
	@cd internal/db && $(MAKE) add-notest
	courtney/courtney -v -o coverage/coverage.out ./...
	@cd internal/db && $(MAKE) rm-notest

tidy: ## add missing and remove unused modules
	 go mod tidy

format: ## run go formatter
	gofumpt -l -w .

format-check: ## check formatting
	# assert `gofumpt -l` produces no output
	test ! $$(gofumpt -l . | tee /dev/stderr)

clean: ## Clean project builds
	@rm -rf ./build/juno
	@rm -rf ./build/juno-cli
	@cd internal/db && $(MAKE) clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
