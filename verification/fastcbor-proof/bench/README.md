# Frozen decoder performance gate

This harness compares the untouched `a4a67242c` baseline with the typed candidate.
It retains all 185 historical IDs: 177 compatible decoder cases, six explicitly
removed Header bloom projections, and two measured zero-decoder controls. Controls
are reported separately and cannot supply evidence of decoder speedups.

The Go overlay uses the existing public fast APIs and block serializers. Class
frames lose the same eight-byte prefix as the historical harness. Corpus loading,
block framing, selecting closures and warming the planner happen before timing.
Every iteration allocates the original destination shape and retains its result in
a sink. Block operation selection happens outside the measured closure. The
wrapper overlay preserves its successful fast path and replaces only its fallback
branches with a panic. There are no smoke counters or FX-routing toggles in timed
calls. Preflight tests exercise all 179 present operations, compare direct record,
projection and index outputs with FX, and verify rejection of fallback. The saved
ordinary and race/checkptr compatibility sweeps separately cover complete block
operation parity and absence of FX calls within fast decoding.

The runner checks both saved smoke reports and frozen source/corpus hashes before
building. Neither baseline nor candidate production files are edited. Each run
records source, harness, wrapper, binary and raw-sample hashes. Use a new directory
for a new implementation or harness. Interrupted runs can resume completed pairs
only when the source/binary/protocol identities and saved raw samples still match.
A partial pair is measured again; it is never combined with a different round.

Measurements hold the historical shared exclusive lock, use CPU 2,
`GOMAXPROCS=1`, Go 1.27.0 linux/amd64/v1, `GOGC=100`, no memory limit and 300 ms
per sample. Each case's baseline/candidate pair runs consecutively in separate
processes; their order alternates by case and round. All present cases run each
round. Builds and correctness preflights precede acquisition of the lock.

Decisions occur only after 5, 10 and 20 rounds. For each case and metric, the
runner calculates paired log ratios and Student-t bounds with one-sided
alpha = 0.05/3. An upper ratio bound at most 1.03 passes; a lower bound above 1.03
fails; overlap is inconclusive. All three metrics of every compatible decoder case
must pass. This intersection-union acceptance rule does not average over cases or
require a cross-case multiplicity adjustment to claim that *all* meet the limit.
The bounds assume independent paired log ratios with an adequate t approximation;
they do not eliminate scheduling, thermal or other host noise. The alpha/3 budget
accounts for the three planned opportunities to accept the complete run.

A zero allocation baseline must stay zero in every observed pair. Mixed zero and
positive allocation counts are conservatively inconclusive, except any increase
from an always-zero baseline fails. A run stops on a complete pass or a detected
regression at a planned look; unresolved cases at round 20 remain inconclusive.
A failure diagnosis is not a simultaneous confidence claim identifying every
regressing case. Do not rerun the same candidate repeatedly to select a lucky
passing result: retain all attempts and investigate changes or noise explicitly.

```sh
python3 -m venv verification/fastcbor-proof/.tools/bench-venv
verification/fastcbor-proof/.tools/bench-venv/bin/pip install scipy==1.15.3
verification/fastcbor-proof/.tools/bench-venv/bin/python -m unittest discover \
  -s verification/fastcbor-proof/bench -p test_runner.py
verification/fastcbor-proof/.tools/bench-venv/bin/python \
  verification/fastcbor-proof/bench/run.py /tmp/juno-cbor-gate-RUN --prepare-only
verification/fastcbor-proof/.tools/bench-venv/bin/python \
  verification/fastcbor-proof/bench/run.py /tmp/juno-cbor-gate-RUN --resume
```

`report.json` is written atomically after each completed pair and planned look.
It is not evidence of a live process: inspect the process/tool handle before
resuming an interrupted run. Do not run concurrent resumes into one directory.
A terminal report cannot resume. Preserve raw logs alongside any published report.

After a run terminates, validate its artifact before reporting a gate result:

```sh
verification/fastcbor-proof/.tools/bench-venv/bin/python \
  verification/fastcbor-proof/bench/check.py /tmp/juno-cbor-gate-RUN
```

The checker rejects live/partial runs and verifies all frozen IDs, both compatibility
reports, current source and binary hashes, the source-derived wrapper overlays,
preflight successes, every raw sample's hash/identity/metrics, and the complete
scheduled look sequence. It recomputes every saved decision. A verified `fail` or
`inconclusive` report remains a failed or inconclusive gate. Run the checker while
the original source revision and smoke evidence are available, before making fixes;
later source changes deliberately invalidate verification against the current tree.
The checker establishes artifact consistency, not independent proof of CPU isolation
or the benchmark process's actual environment.
