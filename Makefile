.DEFAULT_GOAL 	:= help

compile: ## compile:
	@mkdir -p build
	@go build -o build/juno cmd/main.go

run: ## run
	@./build/juno

all: compile run ## build and run

test: ## tests
	mkdir -p coverage
	go test -coverprofile=coverage/coverage.out -covermode=count ./...

gomod_tidy: ## add missing and remove unused modules
	 go mod tidy

gofmt: ## run go formatter
	go fmt -x ./...

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'