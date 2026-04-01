# Generating Go code from propeller.proto

The `propeller.proto` file imports `p2p/proto/common.proto` from the upstream
[starknet-p2p-specs](https://github.com/starknet-io/starknet-p2p-specs) repository.
Since the upstream module is not on the Buf Schema Registry, we use `buf export` + `protoc` directly.

From the project root:

```bash
# 1. Export upstream .proto sources (needed for import resolution)
buf export \
  "https://github.com/starknet-io/starknet-p2p-specs.git#branch=bcfa353a169c859e4d5d97757caccbe76f75bc06,depth=1" \
  -o /tmp/starknet-p2p-specs-proto

# 2. Generate Go code
protoc \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go_opt=Mp2p/proto/common.proto=github.com/starknet-io/starknet-p2p-specs/p2p/proto/common \
  -I /tmp/starknet-p2p-specs-proto \
  -I . \
  consensus/propeller/proto/propeller.proto
```

This produces `propeller.pb.go` in this directory.
