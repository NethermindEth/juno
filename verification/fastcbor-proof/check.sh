#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
proof_dir="$PWD"
repo_dir=$(cd ../.. && pwd)
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.27.0}"
export GOWORK=off
export ELAN_HOME="${ELAN_HOME:-$proof_dir/.tools/elan}"
packages=(./core ./core/internal/cborschema ./core/internal/cborprojections ./core/internal/cborreceipts ./core/internal/cbortransactions ./core/internal/cborstate ./core/internal/cborclasses ./l1/eth ./core/felt ./utils/cbor/cursor ./utils/cbor/cborerr ./verification/fastcbor-proof/feasibility/maps ./verification/fastcbor-proof/feasibility/interfaces github.com/consensys/gnark-crypto/ecc/stark-curve/fp unicode/utf8 math/big internal/cpu encoding/json encoding/json/jsontext)
roots=(-lean-root github.com/NethermindEth/juno=ProofCheck -lean-root github.com/consensys/gnark-crypto=ProofCheck -lean-root unicode/utf8=ProofCheck -lean-root math/big=ProofCheck -lean-root internal/cpu=ProofCheck -lean-root encoding/json=ProofCheck)
(cd "$repo_dir" && go run ./verification/fastcbor-proof/cmd/entrymanifest -check)
(cd "$repo_dir" && go run ./utils/cbor/cmd/cborgen -check)
(cd "$repo_dir" && go test -short ./verification/fastcbor-proof/feasibility/...)
"$proof_dir/.tools/bin/goose" -dir "$repo_dir" -out "$proof_dir/ProofCheck/Code" -configdir "$proof_dir/config" "${roots[@]}" "${packages[@]}"
"$proof_dir/.tools/bin/proofgen" -dir "$repo_dir" -out "$proof_dir/ProofCheck/GeneratedProof" -configdir "$proof_dir/config" "${roots[@]}" "${packages[@]}"
"$ELAN_HOME/bin/lake" build ProofCheck 2>&1 | tee "$proof_dir/lean-build.log"
python3 "$proof_dir/audit.py" "$proof_dir/lean-build.log"
