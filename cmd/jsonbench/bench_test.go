package jsonbench

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	gojson "github.com/goccy/go-json"
	segjson "github.com/segmentio/encoding/json"

	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
)

var libraries = []JSONLibrary{
	{"stdlib", json.Marshal, json.Unmarshal},
	{"goccy", gojson.Marshal, gojson.Unmarshal},
	{"segmentio", segjson.Marshal, segjson.Unmarshal},
}

func init() {
	if sonicLib != nil {
		libraries = append(libraries, *sonicLib)
	}
}

// testCase holds a loaded golden test fixture.
type testCase struct {
	Name      string          // file-derived label, e.g. "block_100_8KB"
	RawResult json.RawMessage // extracted "result" field bytes
	Size      int             // len(RawResult)
	Typed     any             // pre-deserialized into correct Juno type
	NewTarget func() any      // factory for fresh zero-value target
}

// rpcMethod defines how to load and type a golden test for a given RPC endpoint.
type rpcMethod struct {
	Name      string
	Dir       string // directory name, e.g. "starknet_getBlockWithTxs"
	NewTarget func() any
	Typed     func(raw json.RawMessage) (any, error)
}

var rpcMethods = []rpcMethod{
	{
		Name:      "BlockWithTxs",
		Dir:       "starknet_getBlockWithTxs",
		NewTarget: func() any { return new(rpcv10.BlockWithTxs) },
		Typed: func(raw json.RawMessage) (any, error) {
			var v rpcv10.BlockWithTxs
			return &v, json.Unmarshal(raw, &v)
		},
	},
	{
		Name:      "BlockWithReceipts",
		Dir:       "starknet_getBlockWithReceipts",
		NewTarget: func() any { return new(rpcv10.BlockWithReceipts) },
		Typed: func(raw json.RawMessage) (any, error) {
			var v rpcv10.BlockWithReceipts
			return &v, json.Unmarshal(raw, &v)
		},
	},
	{
		Name:      "BlockWithTxHashes",
		Dir:       "starknet_getBlockWithTxHashes",
		NewTarget: func() any { return new(rpcv10.BlockWithTxHashes) },
		Typed: func(raw json.RawMessage) (any, error) {
			var v rpcv10.BlockWithTxHashes
			return &v, json.Unmarshal(raw, &v)
		},
	},
	{
		Name:      "StateUpdate",
		Dir:       "starknet_getStateUpdate",
		NewTarget: func() any { return new(rpcv10.StateUpdate) },
		Typed: func(raw json.RawMessage) (any, error) {
			var v rpcv10.StateUpdate
			return &v, json.Unmarshal(raw, &v)
		},
	},
	{
		Name:      "Transaction",
		Dir:       "starknet_getTransactionByHash",
		NewTarget: func() any { return new(rpcv10.Transaction) },
		Typed: func(raw json.RawMessage) (any, error) {
			var v rpcv10.Transaction
			return &v, json.Unmarshal(raw, &v)
		},
	},
	{
		Name:      "TransactionReceipt",
		Dir:       "starknet_getTransactionReceipt",
		NewTarget: func() any { return new(rpcv10.TransactionReceipt) },
		Typed: func(raw json.RawMessage) (any, error) {
			var v rpcv10.TransactionReceipt
			return &v, json.Unmarshal(raw, &v)
		},
	},
	{
		Name:      "TraceBlockTransactions",
		Dir:       "starknet_traceBlockTransactions",
		NewTarget: func() any { return new([]rpcv10.TracedBlockTransaction) },
		Typed: func(raw json.RawMessage) (any, error) {
			var v []rpcv10.TracedBlockTransaction
			return &v, json.Unmarshal(raw, &v)
		},
	},
	{
		Name:      "SimulateTransactions",
		Dir:       "starknet_simulateTransactions",
		NewTarget: func() any { return new([]rpcv10.SimulatedTransaction) },
		Typed: func(raw json.RawMessage) (any, error) {
			var v []rpcv10.SimulatedTransaction
			return &v, json.Unmarshal(raw, &v)
		},
	},
	{
		Name:      "Class",
		Dir:       "starknet_getClass",
		NewTarget: func() any { return new(rpcv10.Class) },
		Typed: func(raw json.RawMessage) (any, error) {
			var v rpcv10.Class
			return &v, json.Unmarshal(raw, &v)
		},
	},
	{
		Name:      "TraceTransaction",
		Dir:       "starknet_traceTransaction",
		NewTarget: func() any { return new(rpcv10.TransactionTrace) },
		Typed: func(raw json.RawMessage) (any, error) {
			var v rpcv10.TransactionTrace
			return &v, json.Unmarshal(raw, &v)
		},
	},
}

// goldenTestsDir returns the path to the golden tests repo.
func goldenTestsDir(t testing.TB) string {
	t.Helper()
	dir := os.Getenv("GOLDEN_TESTS_DIR")
	if dir == "" {
		t.Skip("GOLDEN_TESTS_DIR not set; skipping benchmark")
	}
	return dir
}

// extractResult parses a JSON-RPC response and returns the "result" field as raw bytes.
func extractResult(data []byte) (json.RawMessage, error) {
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	if envelope.Result == nil {
		return nil, fmt.Errorf("no result field in response")
	}
	return envelope.Result, nil
}

func humanSize(n int) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.0fMB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.0fKB", float64(n)/1024)
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// discoverTestCases finds all .output.json files under sepolia/v0.10/<dir>,
// skips hash-variant duplicates (filenames containing "-0x"),
// deduplicates files with identical response sizes (same struct shape, no new info),
// and loads each into a testCase.
func discoverTestCases(t testing.TB, method rpcMethod) []testCase {
	t.Helper()
	baseDir := filepath.Join(goldenTestsDir(t), "tests", "sepolia", "v0.10", method.Dir)

	var cases []testCase
	seenSizes := map[int]bool{}

	err := filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".output.json") {
			return nil
		}
		// Skip hash-variant files — same response content as the number-based file
		if strings.Contains(d.Name(), "-0x") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Logf("SKIP %s: read error: %v", path, readErr)
			return nil
		}
		raw, extractErr := extractResult(data)
		if extractErr != nil {
			t.Logf("SKIP %s: %v", path, extractErr)
			return nil
		}

		// Deduplicate: skip files whose result is the same byte length as one
		// we already loaded. Same-size responses for the same method exercise
		// the same code paths and struct shapes — no extra signal.
		if seenSizes[len(raw)] {
			return nil
		}
		seenSizes[len(raw)] = true

		typed, typedErr := method.Typed(raw)
		if typedErr != nil {
			t.Logf("SKIP %s: unmarshal into %s failed: %v", path, method.Name, typedErr)
			return nil
		}

		// Build label from relative path
		rel, _ := filepath.Rel(baseDir, path)
		label := strings.TrimSuffix(rel, ".output.json")
		label = strings.ReplaceAll(label, "/", "_")
		label = fmt.Sprintf("%s_%s", label, humanSize(len(raw)))

		cases = append(cases, testCase{
			Name:      label,
			RawResult: raw,
			Size:      len(raw),
			Typed:     typed,
			NewTarget: method.NewTarget,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", baseDir, err)
	}

	// Sort by size for consistent ordering
	sort.Slice(cases, func(i, j int) bool {
		return cases[i].Size < cases[j].Size
	})
	return cases
}

func BenchmarkMarshal(b *testing.B) {
	for _, method := range rpcMethods {
		b.Run(method.Name, func(b *testing.B) {
			cases := discoverTestCases(b, method)
			for _, tc := range cases {
				b.Run(tc.Name, func(b *testing.B) {
					for _, lib := range libraries {
						b.Run(lib.Name, func(b *testing.B) {
							b.SetBytes(int64(tc.Size))
							b.ReportAllocs()
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								out, err := lib.Marshal(tc.Typed)
								if err != nil {
									b.Fatal(err)
								}
								_ = out
							}
						})
					}
				})
			}
		})
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	for _, method := range rpcMethods {
		b.Run(method.Name, func(b *testing.B) {
			cases := discoverTestCases(b, method)
			for _, tc := range cases {
				b.Run(tc.Name, func(b *testing.B) {
					for _, lib := range libraries {
						b.Run(lib.Name, func(b *testing.B) {
							b.SetBytes(int64(tc.Size))
							b.ReportAllocs()
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								target := tc.NewTarget()
								if err := lib.Unmarshal(tc.RawResult, target); err != nil {
									b.Fatal(err)
								}
							}
						})
					}
				})
			}
		})
	}
}

// BenchmarkMarshalHeap measures heap usage per marshal operation.
func BenchmarkMarshalHeap(b *testing.B) {
	for _, method := range rpcMethods {
		b.Run(method.Name, func(b *testing.B) {
			cases := discoverTestCases(b, method)
			for _, tc := range cases {
				b.Run(tc.Name, func(b *testing.B) {
					for _, lib := range libraries {
						b.Run(lib.Name, func(b *testing.B) {
							b.ReportAllocs()
							var msBefore, msAfter runtime.MemStats
							runtime.GC()
							runtime.ReadMemStats(&msBefore)
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								out, err := lib.Marshal(tc.Typed)
								if err != nil {
									b.Fatal(err)
								}
								_ = out
							}
							b.StopTimer()
							runtime.ReadMemStats(&msAfter)
							b.ReportMetric(float64(msAfter.TotalAlloc-msBefore.TotalAlloc)/float64(b.N), "heap-B/op")
						})
					}
				})
			}
		})
	}
}

// BenchmarkUnmarshalHeap measures heap usage per unmarshal operation.
func BenchmarkUnmarshalHeap(b *testing.B) {
	for _, method := range rpcMethods {
		b.Run(method.Name, func(b *testing.B) {
			cases := discoverTestCases(b, method)
			for _, tc := range cases {
				b.Run(tc.Name, func(b *testing.B) {
					for _, lib := range libraries {
						b.Run(lib.Name, func(b *testing.B) {
							b.ReportAllocs()
							var msBefore, msAfter runtime.MemStats
							runtime.GC()
							runtime.ReadMemStats(&msBefore)
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								target := tc.NewTarget()
								if err := lib.Unmarshal(tc.RawResult, target); err != nil {
									b.Fatal(err)
								}
							}
							b.StopTimer()
							runtime.ReadMemStats(&msAfter)
							b.ReportMetric(float64(msAfter.TotalAlloc-msBefore.TotalAlloc)/float64(b.N), "heap-B/op")
						})
					}
				})
			}
		})
	}
}

// ============================================================================
// Summary test: runs all benchmarks and prints a comparative report
// ============================================================================

type benchResult struct {
	Method  string
	Size    string
	Lib     string
	NsPerOp float64
	MBPerS  float64
	BPerOp  float64
	Allocs  float64
}

// repeatCount returns how many times to repeat each benchmark in TestSummary.
// Set BENCH_REPEAT env var (default 3).
func repeatCount() int {
	s := os.Getenv("BENCH_REPEAT")
	if s == "" {
		return 3
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 3
	}
	return n
}

// TestSummary discovers all golden test files, runs marshal/unmarshal benchmarks
// BENCH_REPEAT times (default 3), averages results, and prints comparison tables.
//
// Run: GOLDEN_TESTS_DIR=... go test -v -run=TestSummary -count=1 -timeout=30m ./cmd/jsonbench/
func TestSummary(t *testing.T) {
	dir := os.Getenv("GOLDEN_TESTS_DIR")
	if dir == "" {
		t.Skip("GOLDEN_TESTS_DIR not set")
	}

	repeats := repeatCount()
	t.Logf("Running benchmarks with %d repeats (set BENCH_REPEAT to change)", repeats)

	// Count total test cases for progress
	totalCases := 0
	for _, method := range rpcMethods {
		cases := discoverTestCases(t, method)
		totalCases += len(cases)
	}
	totalBenchmarks := totalCases * len(libraries) * 2 * repeats // 2 = marshal+unmarshal
	t.Logf("Discovered %d test cases across %d methods → %d total benchmark runs",
		totalCases, len(rpcMethods), totalBenchmarks)

	marshalResults := collectResults(t, "marshal", repeats)
	unmarshalResults := collectResults(t, "unmarshal", repeats)

	fmt.Println()
	fmt.Println(strings.Repeat("=", 105))
	fmt.Println("JSON LIBRARY BENCHMARK SUMMARY")
	fmt.Printf("  %d test cases × %d libraries × %d repeats = %d runs per operation\n",
		totalCases, len(libraries), repeats, totalCases*len(libraries)*repeats)
	fmt.Println(strings.Repeat("=", 105))
	fmt.Println()
	fmt.Println("Metrics:")
	fmt.Println("  ns/op     = nanoseconds per operation (lower is better)")
	fmt.Println("  MB/s      = megabytes per second throughput (higher is better)")
	fmt.Println("  B/op      = bytes of heap memory allocated per operation (lower is better)")
	fmt.Println("  allocs/op = number of distinct heap allocations per operation (lower is better)")
	fmt.Println("  *         = best in category for this test case")
	fmt.Println("  +N%       = how much worse than best (e.g. +50% means 1.5x the best value)")
	fmt.Println()

	fmt.Println(strings.Repeat("-", 105))
	fmt.Println("MARSHAL — Per test case")
	fmt.Println(strings.Repeat("-", 105))
	printDetailedTable(marshalResults)

	fmt.Println(strings.Repeat("-", 105))
	fmt.Println("UNMARSHAL — Per test case")
	fmt.Println(strings.Repeat("-", 105))
	printDetailedTable(unmarshalResults)

	fmt.Println(strings.Repeat("=", 105))
	fmt.Printf("AGGREGATE SUMMARY (averaged across %d test cases, %d repeats each)\n", totalCases, repeats)
	fmt.Println(strings.Repeat("=", 105))
	fmt.Println()
	fmt.Println("MARSHAL:")
	printAggregate(marshalResults)
	fmt.Println()
	fmt.Println("UNMARSHAL:")
	printAggregate(unmarshalResults)
	fmt.Println()
}

func collectResults(t *testing.T, operation string, repeats int) []benchResult {
	t.Helper()
	// Accumulate across repeats, then average.
	type key struct {
		Method, Size, Lib string
	}
	accum := map[key]*[4]float64{} // [nsPerOp, mbPerS, bPerOp, allocs]
	var orderedKeys []key

	for rep := 0; rep < repeats; rep++ {
		fmt.Fprintf(os.Stderr, "%s repeat %d/%d\n", operation, rep+1, repeats)
		for _, method := range rpcMethods {
			cases := discoverTestCases(t, method)
			fmt.Fprintf(os.Stderr, "  %s: %d cases\n", method.Name, len(cases))
			for _, tc := range cases {
				for _, lib := range libraries {
					var br testing.BenchmarkResult
					if operation == "marshal" {
						br = testing.Benchmark(func(b *testing.B) {
							b.SetBytes(int64(tc.Size))
							b.ReportAllocs()
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								out, err := lib.Marshal(tc.Typed)
								if err != nil {
									b.Fatal(err)
								}
								_ = out
							}
						})
					} else {
						br = testing.Benchmark(func(b *testing.B) {
							b.SetBytes(int64(tc.Size))
							b.ReportAllocs()
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								target := tc.NewTarget()
								if err := lib.Unmarshal(tc.RawResult, target); err != nil {
									b.Fatal(err)
								}
							}
						})
					}

					nsPerOp := float64(br.T.Nanoseconds()) / float64(br.N)
					bPerOp := float64(br.MemBytes) / float64(br.N)
					allocs := float64(br.MemAllocs) / float64(br.N)
					mbPerS := 0.0
					if nsPerOp > 0 {
						mbPerS = float64(tc.Size) / nsPerOp * 1000
					}

					k := key{method.Name, tc.Name, lib.Name}
					if accum[k] == nil {
						accum[k] = &[4]float64{}
						orderedKeys = append(orderedKeys, k)
					}
					accum[k][0] += nsPerOp
					accum[k][1] += mbPerS
					accum[k][2] += bPerOp
					accum[k][3] += allocs
				}
			}
		}
	}

	results := make([]benchResult, 0, len(orderedKeys))
	for _, k := range orderedKeys {
		a := accum[k]
		results = append(results, benchResult{
			Method:  k.Method,
			Size:    k.Size,
			Lib:     k.Lib,
			NsPerOp: a[0] / float64(repeats),
			MBPerS:  a[1] / float64(repeats),
			BPerOp:  a[2] / float64(repeats),
			Allocs:  a[3] / float64(repeats),
		})
	}
	return results
}

func printDetailedTable(results []benchResult) {
	type caseKey struct{ Method, Size string }
	grouped := map[caseKey][]benchResult{}
	var keys []caseKey
	seen := map[caseKey]bool{}

	for _, r := range results {
		k := caseKey{r.Method, r.Size}
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
		grouped[k] = append(grouped[k], r)
	}

	for _, k := range keys {
		caseResults := grouped[k]
		fmt.Printf("\n  %s / %s\n", k.Method, k.Size)
		fmt.Printf("  %-12s %12s %12s %12s %12s\n", "Library", "ns/op", "MB/s", "B/op", "allocs/op")

		bestNs := math.MaxFloat64
		bestMB := 0.0
		bestB := math.MaxFloat64
		bestA := math.MaxFloat64
		for _, r := range caseResults {
			if r.NsPerOp < bestNs {
				bestNs = r.NsPerOp
			}
			if r.MBPerS > bestMB {
				bestMB = r.MBPerS
			}
			if r.BPerOp < bestB {
				bestB = r.BPerOp
			}
			if r.Allocs < bestA {
				bestA = r.Allocs
			}
		}

		sort.Slice(caseResults, func(i, j int) bool {
			return caseResults[i].NsPerOp < caseResults[j].NsPerOp
		})

		for _, r := range caseResults {
			fmt.Printf("  %-12s %12s %12s %12s %12s\n", r.Lib,
				fmtDelta(r.NsPerOp, bestNs, false),
				fmtDelta(r.MBPerS, bestMB, true),
				fmtDelta(r.BPerOp, bestB, false),
				fmtDelta(r.Allocs, bestA, false),
			)
		}
	}
	fmt.Println()
}

// fmtDelta formats a value and shows % difference from best.
func fmtDelta(val, best float64, higherIsBetter bool) string {
	valStr := fmtNum(val)
	if best == 0 {
		return valStr
	}

	var pctDiff float64
	if higherIsBetter {
		pctDiff = (best - val) / best * 100
	} else {
		pctDiff = (val - best) / best * 100
	}

	if pctDiff < 0.5 {
		return valStr + " *"
	}
	return fmt.Sprintf("%s +%.0f%%", valStr, pctDiff)
}

func fmtNum(v float64) string {
	switch {
	case v >= 1_000_000:
		return fmt.Sprintf("%.0fK", v/1000)
	case v >= 1000:
		return fmt.Sprintf("%.0f", v)
	case v >= 1:
		return fmt.Sprintf("%.1f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

type libAggregate struct {
	Name          string
	AvgNsDelta    float64
	AvgMBDelta    float64
	AvgBDelta     float64
	AvgAllocDelta float64
	WinCountNs    int
	WinCountMB    int
	WinCountB     int
	WinCountA     int
	TestCount     int
}

func printAggregate(results []benchResult) {
	type caseKey struct{ Method, Size string }
	grouped := map[caseKey][]benchResult{}
	var keys []caseKey
	seen := map[caseKey]bool{}

	for _, r := range results {
		k := caseKey{r.Method, r.Size}
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
		grouped[k] = append(grouped[k], r)
	}

	agg := map[string]*libAggregate{}
	for _, l := range libraries {
		agg[l.Name] = &libAggregate{Name: l.Name}
	}

	totalCases := len(keys)

	for _, k := range keys {
		caseResults := grouped[k]
		bestNs := math.MaxFloat64
		bestMB := 0.0
		bestB := math.MaxFloat64
		bestA := math.MaxFloat64
		for _, r := range caseResults {
			if r.NsPerOp < bestNs {
				bestNs = r.NsPerOp
			}
			if r.MBPerS > bestMB {
				bestMB = r.MBPerS
			}
			if r.BPerOp < bestB {
				bestB = r.BPerOp
			}
			if r.Allocs < bestA {
				bestA = r.Allocs
			}
		}
		for _, r := range caseResults {
			a := agg[r.Lib]
			a.TestCount++

			nsDelta := (r.NsPerOp - bestNs) / bestNs * 100
			mbDelta := (bestMB - r.MBPerS) / bestMB * 100
			bDelta := 0.0
			if bestB > 0 {
				bDelta = (r.BPerOp - bestB) / bestB * 100
			}
			aDelta := 0.0
			if bestA > 0 {
				aDelta = (r.Allocs - bestA) / bestA * 100
			}

			a.AvgNsDelta += nsDelta
			a.AvgMBDelta += mbDelta
			a.AvgBDelta += bDelta
			a.AvgAllocDelta += aDelta

			if nsDelta < 0.5 {
				a.WinCountNs++
			}
			if mbDelta < 0.5 {
				a.WinCountMB++
			}
			if bDelta < 0.5 {
				a.WinCountB++
			}
			if aDelta < 0.5 {
				a.WinCountA++
			}
		}
	}

	for _, a := range agg {
		if a.TestCount > 0 {
			a.AvgNsDelta /= float64(a.TestCount)
			a.AvgMBDelta /= float64(a.TestCount)
			a.AvgBDelta /= float64(a.TestCount)
			a.AvgAllocDelta /= float64(a.TestCount)
		}
	}

	sorted := make([]*libAggregate, 0, len(agg))
	for _, a := range agg {
		sorted = append(sorted, a)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].AvgNsDelta < sorted[j].AvgNsDelta
	})

	fmt.Printf("  %-12s %20s %20s %20s %20s\n",
		"Library", "Speed (ns/op)", "Throughput (MB/s)", "Memory (B/op)", "Allocations")
	fmt.Printf("  %-12s %20s %20s %20s %20s\n",
		"", "avg % from best", "avg % from best", "avg % from best", "avg % from best")
	for _, a := range sorted {
		fmt.Printf("  %-12s %20s %20s %20s %20s\n", a.Name,
			fmtAgg(a.AvgNsDelta, a.WinCountNs, totalCases),
			fmtAgg(a.AvgMBDelta, a.WinCountMB, totalCases),
			fmtAgg(a.AvgBDelta, a.WinCountB, totalCases),
			fmtAgg(a.AvgAllocDelta, a.WinCountA, totalCases),
		)
	}
	fmt.Printf("\n  (N=%d test cases. \"wins\" = test cases where library was best in that metric)\n", totalCases)
}

func fmtAgg(avgDelta float64, wins, total int) string {
	if avgDelta < 0.5 {
		return fmt.Sprintf("BEST (%d/%d wins)", wins, total)
	}
	return fmt.Sprintf("+%.1f%% (%d/%d wins)", avgDelta, wins, total)
}
