export CC = clang
.DEFAULT_GOAL := help

juno: ## compile
	@mkdir -p build
	@go build -o build/juno cmd/main.go

all: juno

generate: ## generate
	@cd internal/db && $(MAKE) generate
	@cd pkg/felt && go generate ./...

test: ## tests
	go test ./...

benchmarks: ## benchmarking
	go test ./... -bench=.

test-cover: ## tests with coverage
	mkdir -p coverage
	go test -coverprofile=coverage/coverage.out -covermode=count ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html

install-deps: | install-courtey install-gofumpt ## install some project dependencies

install-courtey:
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
	@find ./internal/cairovm/vmrpc/ -name '*.pb.go' -exec sh -c "awk '/func/ { print \"// notest\"; }; 1;' {} > tmp_file && mv tmp_file {}" \;
	courtney/courtney -v -o coverage/coverage.out ./...
	@find ./internal/cairovm/vmrpc/ -name '*.pb.go' -exec sh -c "awk '!/\/\/ notest/' {} > tmp_file && mv tmp_file {}" \;
	@cd internal/db && $(MAKE) rm-notest

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
