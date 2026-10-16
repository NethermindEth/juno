# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.

## Project Overview

Juno is a Go implementation of a Starknet full-node client by Nethermind. It's a mixed Go/Rust codebase where Go handles the main application logic and Rust provides Cairo VM execution, core cryptographic types, and compiler bindings.

## Build Commands

```bash
make juno              # Full build (compiles Rust deps + Go binary)
make juno-cached       # Fast build (skips Rust rebuild if unchanged)
make rustdeps          # Build only Rust dependencies
make clean             # Clean all build artifacts
```

The binary is output to `build/juno`.

**Requirements**: Go 1.25+, Rust 1.87+. On macOS, jemalloc is required. On Linux, libjemalloc-dev and libbz2-dev are needed.

## Testing

```bash
make test              # Run all tests (clears cache first)
make test-cached       # Run tests with cache
make test-cover        # Run with coverage (outputs to coverage/)
make benchmarks        # Run benchmark tests

# Single package/test
go test ./core/...                    # Test a package
go test -run TestBlockHash ./core/    # Run specific test
go test -v ./rpc/...                  # Verbose output
```

### Test behavior, not implementation

Test what a caller can observe, through the package's exported API — not its internals.
Drive the real code path and assert on observable outcomes (return values, HTTP responses,
emitted logs/metrics, persisted state). Do not call unexported functions or assert on
private fields from tests. If a behavior seems reachable only by poking at internals, test
it through the public entry point that triggers it (e.g. an HTTP request that yields a 503
and a log line — not the private helper that writes the log).

### Testing concurrent code

Use the standard library's `testing/synctest` (stable since Go 1.25) as the default tool for testing concurrent or timing-dependent code — prefer it over `time.Sleep`-based synchronization, which is flaky and slow.

- Wrap the test body in `synctest.Test(t, func(t *testing.T) { ... })`.
- Replace `time.Sleep` calls that wait for goroutines to reach a blocking point with `synctest.Wait()`, which blocks until every goroutine in the bubble is durably blocked — making running/queued state observable deterministically.
- Time is virtual inside the bubble, so timers/tickers fire instantly; use this to test timeouts and intervals without real delays.
- Use `assert.*` (not `require.*`) for checks that run off the test goroutine (inside spawned goroutines), since `require` calls `runtime.Goexit`.

See `utils/throttler/throttler_test.go`, `node/throttled_compiler_test.go`, and `pruner/staleness_test.go` for examples.

## Linting & Formatting

```bash
make lint              # Run full golangci-lint
make lint-diff         # Lint only changes vs origin/main (stricter rules)
make format            # Format Go (gofumpt) and Rust code
make tidy              # go mod tidy
```

## Code Generation

```bash
make generate          # Generate mocks and protobuf files
make generate-buf      # Generate protobuf only
```

## Architecture

```
cmd/juno/           # CLI entry point (cobra-based)
node/               # Node orchestration and configuration
core/               # Starknet core types (blocks, transactions, classes, felt)
rpc/                # JSON-RPC API with versioned implementations (v6-v10)
  └── rpccore/      # Shared RPC implementation
db/                 # Database layer
  ├── pebble/       # Pebble DB v1
  ├── pebblev2/     # Pebble DB v2
  └── remote/       # gRPC remote DB client
blockchain/         # Block validation and chain state
sync/               # Block synchronization
p2p/                # libp2p networking layer
mempool/            # Transaction mempool
vm/rust/            # Cairo VM (Rust, FFI bindings)
core/rust/          # Core Rust types
starknet/
  ├── compiler/rust/ # Starknet compiler bindings
  └── ...           # Starknet-specific Go types
adapters/           # Type conversions (core ↔ p2p, core ↔ starknet)
migration/          # Database migrations
consensus/          # Tendermint-based consensus
```

**Key entry points**:
- `cmd/juno/juno.go` - Main CLI setup and command definitions
- `node/node.go` - Node initialization and service orchestration
- `rpc/handlers.go` - RPC request routing

**Configuration**: CLI flags → environment variables (JUNO_*) → YAML config file. See `cmd/juno/juno.go` for all available flags.

## Branch Naming

Use `kebab-case` with prefixes: `feature/<name>` or `fix/<name>`. Reference issues when applicable.
