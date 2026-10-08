#!/usr/bin/env bash
# Decodes every routed record of the mainnet database with fxamacker and the wrapper and fails on
# any mismatch, decode error, unexpected fallback or retained input. The database is opened
# read-only.
#
#   ./utils/cbor/fast/sweep/run.sh            every block (takes a long time)
#   ./utils/cbor/fast/sweep/run.sh quick      every 1000th block, all classes
#
# Environment: SWEEP_DB, SWEEP_KINDS, SWEEP_STRIDE, SWEEP_FROM, SWEEP_TO, SWEEP_WORKERS (see
# sweep_test.go), SWEEP_CPUS (default 14,15), SWEEP_OUT (log directory, default a temporary
# directory). The full sweep is CPU bound in decoding (about 1.5M blocks an hour on two cores at
# mainnet 12.9M; the database holds a lock, so only one sweep can run at a time), and one
# iterator feeds a decoding goroutine per CPU, so give it more CPUs to go faster:
#
#   SWEEP_CPUS=4-13 ./utils/cbor/fast/sweep/run.sh
#
# SWEEP_FROM and SWEEP_TO resume or limit the block-keyed buckets, with SWEEP_KINDS to leave
# out the classes.
set -euo pipefail
cd "$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"

if [[ ${1:-} == quick ]]; then export SWEEP_STRIDE=${SWEEP_STRIDE:-1000}; fi
cpus=${SWEEP_CPUS:-14,15}
procs=$(taskset -c "$cpus" nproc)
out=${SWEEP_OUT:-$(mktemp -d -t cbor-sweep.XXXXXX)}
mkdir -p "$out"
# The VM library is linked from this tree's build, or from the main checkout's.
if [[ -z ${CGO_LDFLAGS:-} ]]; then
	lib=$PWD/vm/rust/target/release
	[[ -e $lib/libjuno_starknet_rs.a ]] || lib=$HOME/work/juno/vm/rust/target/release
	export CGO_LDFLAGS="-L$lib -lm"
fi
export SWEEP_SUMMARY="$out/summary.txt"

go test -c -tags sweep -o "$out/sweep.test" ./utils/cbor/fast/sweep
echo "sweep: commit $(git rev-parse --short HEAD), cpus $cpus, log $out/sweep.log"
status=0
GOMAXPROCS=$procs taskset -c "$cpus" "$out/sweep.test" -test.run '^TestMainnetSweep$' -test.v -test.timeout 24h \
	2>&1 | tee "$out/sweep.log" || status=$?
grep -q -- '^--- PASS: TestMainnetSweep' "$out/sweep.log" || status=1
cat "$SWEEP_SUMMARY" 2>/dev/null || true
exit $status
