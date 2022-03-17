.DEFAULT_GOAL 	:= help

compile: ## compile:
	@mkdir -p build
	@go build -o build/juno cmd/juno/main.go

run: ## run
	@./build/juno

all: compile run ## build and run

test: ## tests
	go test ./test/

benchmarks: ## Benchmarking
	go test ./test/ -bench=.

test-cover: ## tests with coverage
	mkdir -p coverage
	go test -coverprofile=coverage/coverage.out -covermode=count ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html

install-deps: ## Install some project dependencies
	git clone https://github.com/DemerzelSolutions/courtney
	(cd courtney && go get  ./... && go build courtney.go)
	go get ./...

codecov-test:
	mkdir -p coverage
	courtney/courtney -v -o coverage/coverage.out ./...

gomod_tidy: ## add missing and remove unused modules
	 go mod tidy

gofmt: ## run go formatter
	go fmt -x ./...

clean: ## Clean project builds
	@rm -rf ./build/juno

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
