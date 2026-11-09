#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.27.0}"
export GOWORK=off
poc_tools="$PWD/.tools"
revision=$(cat perennial.rev)
if [[ ! -d "$poc_tools/perennial/.git" ]]; then
  git clone --depth 1 --branch lean https://github.com/mit-pdos/perennial.git "$poc_tools/perennial"
fi
if [[ $(git -C "$poc_tools/perennial" rev-parse HEAD) != "$revision" ]]; then
  git -C "$poc_tools/perennial" fetch --depth 1 origin "$revision"
  git -C "$poc_tools/perennial" checkout --detach "$revision"
fi
(cd "$poc_tools/perennial" && etc/setup-hooks.sh)
mkdir -p "$poc_tools/bin"
(cd "$poc_tools/perennial" && go build -o "$poc_tools/bin/goose" ./goose/cmd/goose)
(cd "$poc_tools/perennial" && go build -o "$poc_tools/bin/proofgen" ./goose/cmd/proofgen)
export ELAN_HOME="${ELAN_HOME:-$poc_tools/elan}"
if [[ ! -x "$ELAN_HOME/bin/elan" ]]; then
  curl --fail --location --retry 2 https://raw.githubusercontent.com/leanprover/elan/master/elan-init.sh --output "$poc_tools/elan-init.sh"
  sh "$poc_tools/elan-init.sh" -y --no-modify-path --default-toolchain none
fi
"$ELAN_HOME/bin/elan" toolchain install "$(cat lean-toolchain)"
"$ELAN_HOME/bin/lake" update
