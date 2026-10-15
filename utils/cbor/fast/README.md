WIP



# Fast CBOR decoder

Juno's custom decoder for the stored records (headers, transactions, receipts, state updates,
classes). It reads them in one pass, straight into the Go values, with readers that are mostly
generated from the Go types and hand-written where a record needs more. It is a
decoder only: [fxamacker/cbor](https://github.com/fxamacker/cbor) still writes every record in
the same format, so the fast path can be removed without migrating anything.

## How it relates to fxamacker

The decoder shares no code with it. fxamacker is the **writer** (the bytes on disk are whatever it
wrote), the **fallback** (any read the fast path does not take goes to it) and the **oracle** (the
tests decode the same bytes both ways and require equal values). So for every input the fast
decoder must return what fxamacker returns, or return `fast.ErrUnsupported` and let it decide.

## Layout

| Path | Role |
|---|---|
| `utils/cbor/cbor.go`, `route.go` | the API Juno calls, the route table (which Go types have a fast reader) and the fallback |
| `utils/cbor/fast/` | primitives: `Reader` cursor, heads, ints, strings, felts, `Skip`, key matching |
| `utils/cbor/fast/records/` | one reader per stored type: `readers_gen.go` (generated) and the hand-written ones |
| `utils/cbor/fast/records/internal/gen/` | the generator behind `readers_gen.go`; run `go generate ./utils/cbor/fast/records` |
| `utils/cbor/registry/` | registers tags and calls `records.Register()` |
| `core/partial_cbor_fast.go` | readers for projections: a few fields of a record, the rest skipped |
| `utils/cbor/fast/sweep/` | mainnet sweep and fuzz scripts |

`fast` must not import `core`.

## One decode

`cbor.Unmarshal` looks up the destination type in the route table. It goes to fxamacker when no
reader is registered, when `JUNO_CBOR_FAST=off`, or when the destination already holds a value
(fxamacker merges into it, we do not). Otherwise a pooled `Reader` runs the reader: an error is
returned as is (a wrong CBOR type is an error), `ErrUnsupported` resets the destination and
redoes the read with fxamacker, and success returns. The first fallback of each type and reason is
logged once (`cbor.SetFallbackLogger`, set in `node.New`). Nothing is counted.

## What is stored

A struct is a map keyed by field name in canonical order (shorter keys first, then bytewise). A
felt is an array of four unsigned integers. A value behind an interface is a tag, then the map of
its concrete type. Pointers and slices are the value or `null`.

## A reader

Each struct has a constant per field, a table of wire names, and a loop. Most are generated into
`records/readers_gen.go` (see `ReadReceipt`):

```go
s, err := r.OpenStruct(receiptFields)
for err == nil {
    var field int
    field, err = r.NextField(&s)
    switch field {
    case fast.Done:
        return err
    case receiptFee:
        err = readFelt(r, &dst.Fee)
    ...
    default: // a key that names no field
        err = r.Skip()
    }
}
```

The generator (`records/internal/gen`) takes a list of core types and finds their fields by
reflection, as fxamacker does. It picks each field's helper from its Go type, through the `known`
table (`*felt.Felt` is `readFelt`, `uint64` is `readUint64`, ...) and a rule for a generated struct
behind a pointer or in a slice. A type it does not know, or one with its own encoding, stops it with
the type and field named. A few fields are overrides (the two gas prices of a header share one
allocation). The output is the code you would write by hand, so it costs nothing at run time.

Some readers stay hand-written because they need more than a `case` per field: the tag dispatch of
`ReadTransaction` and `ReadClassDefinition`, `readExecutionResources` (three structs in one
allocation, in `receipt.go`), `SegmentLengths` and the felt-map helpers.

`NextField` first compares the bytes at the cursor with the key it expects next, three words at a
time, which suits canonical maps. Any other key is matched as fxamacker does: exactly, then ignoring
case. The helpers (`readFelt`, `readPtr`, `readSlice`, ...) are in `records/helpers.go`.

It is fast because it is one pass without reflection, matches keys in order, reads the common felt
form with four loads, shares one backing array across the elements of a slice or map, and decodes
only the needed fields in projections.

## Rules for every reader

1. Never keep a reference to the input; copy what you keep (`BytesNoCopy` and `StringNoCopy` borrow).
2. Write only the destination; `ErrUnsupported` may come before or after writing, since the wrapper resets it.
3. Count nesting through the `Reader` (`ArrayLen`/`MapLen`, then `Leave`), for fxamacker's limit of 32.
4. Never panic or hang, and never allocate more than the input can back.
5. Match fxamacker or fall back. A form Juno does not write, where the decoders differ, is handled in
   this order: return `ErrUnsupported`; match fxamacker if that is a few lines off the hot path; filter it
   from the fuzz only with a comment saying why it cannot be in a database.

We match: exact then case-insensitive keys (old headers use `GasPrice`), skipped unknown and
integer keys, `null` equal to `undefined` and leaving a plain field alone, short and long felt
arrays, nesting of 32 counted over the whole record. We differ: a wrong CBOR type is an error.
We fall back on: indefinite lengths, a tag where a value is expected, a repeated key, an unusual
felt count, a null `ResourceBounds` key, and Cairo 0 classes.

## Tests

| Test | Protects against |
|---|---|
| `fieldsOf` panic at init | a core field added or renamed without its reader |
| `TestGeneratedFilesAreFresh` | `readers_gen.go` left behind a change to a core type or to the generator |
| `internal/gen` tests | the generator accepting an unknown or self-encoding type |
| `TestFieldsFollowTheEncoder` | the encoder's key order changing, which would slow every key |
| `TestRandomValuesDecodeLikeFxamacker` | a field read differently than fxamacker reads it, with no fixtures to update |
| `TestNoTaggedTypeIsAPlainField` | a type with a CBOR tag held directly in a stored type, which no generated reader handles and so would always fall back |
| `…MatchesFxamacker`, `adversarial_test.go` | hand-built and mutated records giving a different result |
| `TestDecodingDoesNotRetainInput`, shared-array tests | aliasing the input, aliased arrays |
| `TestEventsBloomOfAnOldHeaderIsSkipped` | an unmigrated header, which still holds an events bloom, reading differently than fxamacker reads it |
| `Fuzz…` | arbitrary bytes giving a different result, or a panic |
| `core/partial_cbor_fast_test.go` | projection readers drifting from their records |
| `sweep/` (build tag `sweep`) | any difference on a real mainnet database; run before a release |

Run the fuzz with `sweep/fuzz.sh` and the sweep with `sweep/run.sh quick`, and rerun both when
fxamacker is upgraded.

## Changing it

- **Field added to a stored type:** run `go generate ./utils/cbor/fast/records` and commit the
  result (`TestGeneratedFilesAreFresh` and the init check fail until you do), then update any
  projection that names the field. If the generator stops with `no helper reads a X`, check how
  fxamacker stores that type and add it to `known` in `internal/gen/main.go` (a type with its own
  encoding needs a helper that reads that encoding), or override the field. The random-value test
  fails if the field is read wrongly.
- **Field changes type or encoding:** regenerate. The helper follows the type, and a new type may
  need a `known` entry. A field whose old records must still read needs both forms, or a fallback.
- **New routed type:** add it to `readers` in `internal/gen/main.go` and regenerate, or, if it needs
  more than a `case` per field, write the reader by hand (`readExecutionResources` is a model). Then
  register it in `records/register.go`, add it to the random-value, key-order and differential
  tests, add a fuzz target, and run the sweep. A type with no reader decodes through fxamacker,
  correctly and slower.
- **New endpoint:** nothing, if it reads types already routed. Needing a few fields of a big record
  is a projection in `core/partial_cbor_fast.go`.
- **Turn it off:** `JUNO_CBOR_FAST=off`, or `cbor.SetFastEnabled(false)`. To remove it, delete the
  `records.Register()` call and the `RegisterReader` calls in `core/partial_cbor_fast.go`.

## Sharp edges

- `route.go` keys routes on the runtime's interface type word (`unsafe`), saving about 20 ns a
  decode, and `isZero` compares bytes before asking reflect. Both rely on Go's layout.
- Felts are read through an unsafe cast from `[4]uint64`.
- The per-field helpers are concrete on purpose. One generic reader for every integer type
  measured 1.5% slower across the record benchmarks (4.5% on an L1 handler transaction), and a
  function for the felt-shape test, even an inlined one, 1 to 2% slower on `BenchmarkReadFeltArray`.
  Measure with an interleaved `benchstat` run before merging two of them.
- The generator must not import `utils/cbor/registry` (it imports `records`, which panics at init
  when `readers_gen.go` is stale). Keep overrides to one call of a compiled helper. For the same
  reason it cannot know which types carry a tag; `TestNoTaggedTypeIsAPlainField` does.
- Matching fxamacker is a standing cost: recheck it with the fuzz and the sweep when it changes.

# Fast CBOR decoder benchmarks

This package provides [CBOR reader primitives](reader.go) and [typed readers for
Juno's stored records](records/). The [CBOR wrapper](../route.go) selects the fast
reader or falls back to fxamacker (FX).

These tests used frozen source commit
`583218f49bb5080d5509afe8e17d5b6d1b002d5b`, Go 1.27.0.
FX and fast use the same candidate code; FX has fast routing disabled. Cborlite is
a benchmark-only port of `c47dbca94` onto the same candidate, including the current
felt leaf from #3951, with its original parser/planner and shape restrictions.


Changes below use paired block ratios. Positive CPU reduction and throughput gain
favor fast. The two comparator matrices have separate fast measurements; absolute
results across matrices are not pooled. Point estimates alone do not establish a
material improvement. Confidence intervals use four block-level log ratios and
Student t(3); intervals are per metric, without multiple-comparison correction.

### Server CPU per request

| Workload | CPU reduction vs FX, 4 VUs | vs FX, 16 VUs | vs cborlite, 4 VUs | vs cborlite, 16 VUs | Cborlite integration |
|---|---:|---:|---:|---:|---|
| getBlockWithTxHashes | +49.0% | +49.5% | Unsupported | Unsupported | Unsupported |
| getBlockWithTxs | +21.5% | +22.9% | +3.9% | +5.6% | Hybrid |
| getBlockWithReceipts | +22.3% | +25.1% | +6.6% | +6.1% | Hybrid |
| getTransactionByHash | +24.6% | +17.2% | +16.7% | +13.4% | Hybrid |
| getTransactionByBlockIdAndIndex | +44.5% | +40.9% | +40.6% | +38.1% | Hybrid |
| getTransactionReceipt | +35.2% | +21.1% | +21.0% | +13.3% | Hybrid |
| getTransactionStatus | +27.7% | +17.5% | +27.4% | +18.9% | Hybrid |
| getStateUpdate | +25.8% | +23.1% | +10.7% | +9.7% | Routed |
| getClassSierra | +29.9% | +35.3% | -0.8% | +1.0% | Routed |
| getClassAtSierra | +29.6% | +34.4% | +1.9% | +0.9% | Routed |
| getCompiledCasm | +23.9% | +28.3% | +0.2% | +0.3% | Routed |
| getEventsOneBlock | +24.2% | +22.5% | +25.7% | +23.8% | Hybrid |
| getEventsRange | +27.7% | +25.8% | +28.5% | +29.6% | Hybrid |
| getEventsFiltered | +33.6% | +34.4% | +35.0% | +36.1% | Hybrid |
| getClassCairo0 | -0.8% | +0.3% | -2.7% | -4.4% | Routed; fast uses FX |
| getStorageAt | -0.2% | +0.9% | -0.9% | -0.1% | Zero-route control |
| getClassSierraOutliers | +29.9% | +34.0% | +1.5% | +0.3% | Routed |
| mix | +32.2% | +32.9% | Unsupported | Unsupported | Unsupported |
| getBlockWithTxHashesCborliteSupported | +8.5% | +5.9% | +2.8% | +3.6% | Hybrid; supported subset |
| mixCborliteSupported | +32.0% | +32.4% | +27.0% | +28.1% | Hybrid; supported subset |

**Hybrid** means recorded cborlite calls include FX fallbacks. **Routed** means
registered cborlite calls were observed without a counted fallback. Counters cover
registered destination types; unregistered FX destination types are uncounted.
Storage is a zero-route control. Cairo0 uses FX fallback in fast mode.

Cborlite's full block-hash cohort returned errors for 4,903 of 5,016 measured
requests. Its separately named supported subset contains 113 measured requests
and 25 warmup requests. The full cborlite mix is unsupported. The supported mix
retains all 16 equally weighted components, replacing the block-hash component
with its supported subset; it has its own corpus hash. Comparisons within each row
use identical measured and warmup request cohorts.

### Throughput

| Workload | Gain vs FX, 4 VUs | vs FX, 16 VUs | vs cborlite, 4 VUs | vs cborlite, 16 VUs |
|---|---:|---:|---:|---:|
| getBlockWithTxHashes | +41.6% | +57.9% | Unsupported | Unsupported |
| getBlockWithTxs | +17.0% | +10.9% | +3.4% | +2.2% |
| getBlockWithReceipts | +14.9% | +11.8% | +3.8% | +2.4% |
| getTransactionByHash | +19.9% | +8.3% | +11.9% | +6.6% |
| getTransactionByBlockIdAndIndex | +34.1% | +17.0% | +28.6% | +14.4% |
| getTransactionReceipt | +35.9% | +11.5% | +18.0% | +7.3% |
| getTransactionStatus | +23.0% | +6.3% | +21.3% | +7.7% |
| getStateUpdate | +18.9% | +7.8% | +6.5% | +1.6% |
| getClassSierra | +20.9% | +17.5% | -1.1% | +0.2% |
| getClassAtSierra | +20.5% | +15.3% | +0.9% | +0.2% |
| getCompiledCasm | +14.1% | +5.3% | -0.5% | +0.1% |
| getEventsOneBlock | +18.8% | +8.3% | +20.0% | +8.0% |
| getEventsRange | +17.1% | -1.7% | +17.9% | +8.0% |
| getEventsFiltered | +48.2% | +50.9% | +51.7% | +59.3% |
| getClassCairo0 | -0.4% | +1.2% | -0.7% | -0.5% |
| getStorageAt | +0.0% | +0.7% | -0.7% | +0.1% |
| getClassSierraOutliers | +22.5% | +18.2% | +0.4% | +0.2% |
| mix | +29.9% | +46.8% | Unsupported | Unsupported |
| getBlockWithTxHashesCborliteSupported | +2.7% | -1.8% | +0.5% | +1.2% |
| mixCborliteSupported | +29.6% | +44.3% | +24.0% | +36.4% |

### Interpretation

- The full mix reduces CPU versus FX by **32.2%** at 4 VUs (95% CI 31.8–32.6%)
  and **32.9%** at 16 VUs (32.1–33.7%). Throughput improves 29.9% and 46.8%.
- The supported-subset mix reduces CPU versus hybrid cborlite by **27.0%** at
  4 VUs (26.2–27.7%) and **28.1%** at 16 VUs (27.6–28.6%). Throughput improves
  24.0% and 36.4%. This is a different cohort from the full mix.
- Sierra/CASM endpoints are close to CPU parity with cborlite. Cairo0's FX-fallback
  control uses 4.4% more CPU than cborlite at 16 VUs (95% CI 3.3–5.5%), exceeding
  the frozen 2% family tolerance.
- Sierra outliers at 16 VUs allocate 1.52% more bytes/request than FX (95% CI
  0.71–2.34%). That interval crosses the 1% materiality threshold, leaving the
  allocation result inconclusive. Event-range throughput and tail latency versus
  FX at 16 VUs also have wide, inconclusive intervals.

Tolerances were frozen before candidate measurements. Only transaction-by-hash,
block-with-receipts, Sierra-class and storage workloads were independently
calibrated; other workloads inherit family thresholds. CPU thresholds are
2.0–3.913%, throughput 3%, allocated bytes 1%, and p50/p95/p99 latency 5%/10%/15%.

## Decoder microbenchmarks

These are median **microseconds per decode** across five interleaved rounds,
300 ms per variant, pinned to one CPU with `GOMAXPROCS=1`. Every decode uses fresh
production destinations and the same stored mainnet bytes. The `p` labels denote
fixture-size percentiles, not request-latency percentiles. Speedup is comparator
time divided by fast time. Microbenchmarks use observed timing ranges, rather
than the k6 confidence-interval method.

| Record / fixture | FX µs | Cborlite µs | Fast µs | Speedup vs FX | Speedup vs cborlite |
|---|---:|---:|---:|---:|---:|
| receipt-p0 | 2.271 | 0.863 | 0.385 | 5.90× | 2.24× |
| receipt-p50 | 2.815 | 1.035 | 0.513 | 5.49× | 2.02× |
| receipt-p95 | 12.718 | 3.236 | 1.675 | 7.59× | 1.93× |
| receipt-p100 | 144.101 | 38.270 | 18.727 | 7.69× | 2.04× |
| class-p0 | 2.727 | 1.082 | 0.481 | 5.67× | 2.25× |
| class-p50 | 171.137 | 20.856 | 14.965 | 11.44× | 1.39× |
| class-p95 | 1,809.738 | 206.248 | 163.678 | 11.06× | 1.26× |
| class-p100 | 9,990.276 | 1,246.921 | 1,057.293 | 9.45× | 1.18× |
| header-p50 | 3.021 | 1.437 | 0.529 | 5.71× | 2.71× |
| state-p0 | 2.036 | 1.150 | 0.455 | 4.48× | 2.53× |
| state-p50 | 18.966 | 8.536 | 3.271 | 5.80× | 2.61× |
| state-p95 | 183.545 | 78.746 | 27.606 | 6.65× | 2.85× |
| state-p100 | 329.457 | 139.915 | 48.936 | 6.73× | 2.86× |
| transactionInvoke-p50 | 3.915 | 1.289 | 0.533 | 7.35× | 2.42× |
| transactionInvoke-p100 | 124.929 | 15.111 | 11.474 | 10.89× | 1.32× |
| transactionDeclare-p50 | 1.858 | 0.721 | 0.230 | 8.08× | 3.14× |
| transactionDeployAccount-p50 | 2.842 | Unsupported | 0.417 | 6.82× | — |
| transactionDeploy-p50 | 1.318 | 0.576 | 0.175 | 7.52× | 3.28× |
| transactionL1Handler-p50 | 1.377 | 0.596 | 0.178 | 7.74× | 3.35× |

The expanded corpus has 185 deduplicated cases: 54 full records, 77 projections,
4 prefix/index cases and 50 block/partial/lazy cases. Fast supports 184 cases;
cborlite supports 59, including two empty zero-route controls, leaving 57 actual
cborlite decoding cases. All 920 paired fast/FX micro samples use no more bytes
or allocations than FX. Supported timed decoder calls assert zero unsupported
or nonzero-destination fallbacks.

`fullSerialized` cases measure framing and lazy-container construction, rather
than fully materialized records. `UnmarshalFirst` owns decoded indexes and
intentionally borrows the byte-exact suffix. These are sampled benchmark results;
the full mainnet correctness sweep remains a separate enablement gate.
