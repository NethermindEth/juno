WIP



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
