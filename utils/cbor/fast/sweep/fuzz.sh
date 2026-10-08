#!/usr/bin/env bash
# Runs every differential fuzz target of the fast decoder for a bounded time, each in its own
# process with memory and time limits, seeded from the golden vectors and the real records of the
# benchmark corpus. A failing input is minimized by the fuzzer and kept as a regression case in
# the package's testdata/fuzz directory; the script lists them and exits non-zero.
#
#   ./utils/cbor/fast/sweep/fuzz.sh                  every target, FUZZ_TIME each
#   ./utils/cbor/fast/sweep/fuzz.sh FuzzReceipt ...  only these targets
#
# Environment: FUZZ_TIME (default 60s), FUZZ_CPUS (default 14,15), FUZZ_MEM_KB (virtual memory
# limit per process, default 8 GiB), FUZZ_GOMEMLIMIT (default 1GiB), FUZZ_CORPUS (benchmark
# records, "-" for none), FUZZ_CACHE (generated corpus, kept between runs), FUZZ_OUT (logs).
set -uo pipefail
root=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
cd "$root" || exit 1

fuzztime=${FUZZ_TIME:-60s}
cpus=${FUZZ_CPUS:-14,15}
procs=$(taskset -c "$cpus" nproc)
mem_kb=${FUZZ_MEM_KB:-8388608}
cache=${FUZZ_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/juno-cbor-fuzz}
out=${FUZZ_OUT:-$(mktemp -d -t cbor-fuzz.XXXXXX)}
mkdir -p "$out" "$cache"
if [[ -z ${CGO_LDFLAGS:-} ]]; then
	lib=$PWD/vm/rust/target/release
	[[ -e $lib/libjuno_starknet_rs.a ]] || lib=$HOME/work/juno/vm/rust/target/release
	export CGO_LDFLAGS="-L$lib -lm"
fi

targets=(
	records:FuzzReceipt records:FuzzHeader records:FuzzTransaction records:FuzzStateUpdate
	records:FuzzSierraClass fast:FuzzSkipMatchesFxamacker fast:FuzzReadersMatchFxamacker
	fast:FuzzNeverPanics fast:FuzzFeltsMatchFxamacker
)
if (($#)); then
	wanted=" $* "
	selected=()
	for t in "${targets[@]}"; do [[ $wanted == *" ${t#*:} "* ]] && selected+=("$t"); done
	targets=("${selected[@]}")
fi

echo "fuzz: commit $(git rev-parse --short HEAD), $fuzztime per target, cpus $cpus, cache $cache, logs $out"
go test -c -o "$out/fast.test" ./utils/cbor/fast || exit 1
go test -c -o "$out/records.test" ./utils/cbor/fast/records || exit 1
FUZZ_SEED_DIR=$cache go test -tags sweep -run '^TestWriteFuzzSeeds$' -count=1 ./utils/cbor/fast/sweep >"$out/seeds.log" 2>&1 ||
	{ cat "$out/seeds.log"; exit 1; }

# Regression inputs that exist before this run, so new ones can be told apart.
before=$(find utils/cbor/fast -path '*/testdata/fuzz/*' -type f 2>/dev/null | sort)

limit_s=$(( $(sed -E 's/s$//; s/m$/*60/; s/h$/*3600/' <<<"$fuzztime" | bc) + 300 ))
failed=()
for t in "${targets[@]}"; do
	pkg=${t%%:*} name=${t#*:}
	dir=utils/cbor/fast
	[[ $pkg == records ]] && dir=utils/cbor/fast/records
	start=$(date +%s)
	(
		cd "$dir" || exit 1
		ulimit -v "$mem_kb"
		GOMEMLIMIT=${FUZZ_GOMEMLIMIT:-1GiB} GOMAXPROCS=$procs exec timeout --kill-after=30 "$limit_s" \
			taskset -c "$cpus" "$out/$pkg.test" -test.run '^$' -test.fuzz "^$name\$" \
			-test.fuzztime "$fuzztime" -test.fuzzcachedir "$cache/$pkg" -test.parallel "$procs"
	) >"$out/$name.log" 2>&1
	status=$?
	execs=$(grep -oE 'execs: [0-9]+' "$out/$name.log" | tail -1)
	if ((status == 0)); then
		echo "fuzz: PASS $name ($(( $(date +%s) - start ))s, ${execs:-no stats})"
	else
		echo "fuzz: FAIL $name (exit $status, ${execs:-no stats}); log $out/$name.log"
		grep -E 'Failing input|Error:|Messages:|panic:|hung|terminated' "$out/$name.log" | head -8 | sed 's/^/      /'
		failed+=("$name")
	fi
done

after=$(find utils/cbor/fast -path '*/testdata/fuzz/*' -type f 2>/dev/null | sort)
new=$(comm -13 <(echo "$before") <(echo "$after"))
if [[ -n $new ]]; then
	echo "fuzz: new regression inputs (minimized; they now run in go test):"
	echo "$new" | sed 's/^/  /'
fi
if ((${#failed[@]})); then
	echo "fuzz: ${#failed[@]} target(s) failed: ${failed[*]}"
	exit 1
fi
echo "fuzz: all ${#targets[@]} targets passed"
