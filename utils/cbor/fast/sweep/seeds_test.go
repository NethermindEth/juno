//go:build sweep

package sweep

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

const (
	defaultCorpus = "/mnt/dev/juno-benchmark/cbor-fast-iter/corpus"
	golden        = "../../compatibility/testdata/golden_bytes.json"
	// maxSeed keeps seeds small enough for the fuzzer to mutate quickly.
	maxSeed = 64 << 10
	// maxPrimitiveSeed bounds the seeds of the primitive targets, which walk one item.
	maxPrimitiveSeed = 4 << 10
)

// TestWriteFuzzSeeds writes seed inputs for every differential fuzz target into the fuzz cache
// directory FUZZ_SEED_DIR (the -test.fuzzcachedir of the target's package), one directory per
// target: the golden vectors and the real records of the benchmark corpus (FUZZ_CORPUS).
// Headers are written without their events bloom, which the header target cannot give
// fxamacker (see FuzzHeader).
func TestWriteFuzzSeeds(t *testing.T) {
	dir := os.Getenv("FUZZ_SEED_DIR")
	if dir == "" {
		t.Skip("set FUZZ_SEED_DIR")
	}
	corpus := os.Getenv("FUZZ_CORPUS")
	if corpus == "" {
		corpus = defaultCorpus
	}

	goldenBytes := loadGolden(t)
	records := loadRecords(t, corpus)

	pick := func(limit int, golden func(string) bool, corpus func(string) bool) [][]byte {
		var out [][]byte
		add := func(set map[string][]byte, match func(string) bool) {
			keys := make([]string, 0, len(set))
			for k := range set {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if match(k) && len(set[k]) <= limit && !bytes.Contains(bytes.ToLower(set[k]), []byte("eventsbloom")) {
					out = append(out, set[k])
				}
			}
		}
		add(goldenBytes, golden)
		add(records, corpus)
		return out
	}
	prefix := func(p string) func(string) bool { return func(s string) bool { return strings.HasPrefix(s, p) } }
	contains := func(p string) func(string) bool { return func(s string) bool { return strings.Contains(s, p) } }
	all := func(string) bool { return true }
	isTransaction := func(s string) bool {
		return strings.Contains(s, "Transaction") && !strings.Contains(s, "Receipt") && !strings.Contains(s, "Indexes")
	}

	targets := map[string][][]byte{
		"records/FuzzReceipt":     pick(maxSeed, contains("TransactionReceipt"), prefix("receipt")),
		"records/FuzzHeader":      pick(maxSeed, prefix("Header"), prefix("header")),
		"records/FuzzTransaction": pick(maxSeed, isTransaction, prefix("transaction")),
		"records/FuzzStateUpdate": pick(maxSeed, prefix("StateUpdate"), prefix("state")),
		"records/FuzzSierraClass": pick(maxSeed, func(s string) bool {
			return s == "SierraClass" || strings.HasPrefix(s, "DeclaredClassDefinition")
		}, prefix("class")),
		"fast/FuzzSkipMatchesFxamacker":  pick(maxPrimitiveSeed, all, all),
		"fast/FuzzReadersMatchFxamacker": pick(maxPrimitiveSeed, all, all),
		"fast/FuzzNeverPanics":           pick(maxPrimitiveSeed, all, all),
		"fast/FuzzFeltsMatchFxamacker":   pick(maxPrimitiveSeed, all, all),
	}
	for target, seeds := range targets {
		require.NotEmpty(t, seeds, target)
		out := filepath.Join(dir, target)
		require.NoError(t, os.MkdirAll(out, 0o755))
		for _, seed := range seeds {
			sum := sha256.Sum256(seed)
			body := fmt.Sprintf("go test fuzz v1\n[]byte(%q)\n", seed)
			require.NoError(t, os.WriteFile(filepath.Join(out, hex.EncodeToString(sum[:8])), []byte(body), 0o644))
		}
		t.Logf("%s: %d seeds", target, len(seeds))
	}
}

// withoutKey drops one key and its value from the top-level map of a record.
func withoutKey(t *testing.T, record []byte, drop string) []byte {
	t.Helper()
	r := fast.NewReader(record)
	count, err := r.MapLen()
	require.NoError(t, err)
	var pairs []byte
	kept := 0
	for range count {
		start := r.Pos()
		key, _, err := r.Key()
		require.NoError(t, err)
		require.NoError(t, r.Skip())
		if string(key) != drop {
			pairs = append(pairs, record[start:r.Pos()]...)
			kept++
		}
	}
	require.Less(t, kept, 256)
	out := make([]byte, 0, 2+len(pairs)+r.Len())
	if kept < 24 {
		out = append(out, 0xa0|byte(kept))
	} else {
		out = append(out, 0xb8, byte(kept))
	}
	return append(append(out, pairs...), r.Remaining()...)
}

// loadGolden returns the golden vectors, with a declared class unwrapped to the class.
func loadGolden(t *testing.T) map[string][]byte {
	t.Helper()
	raw, err := os.ReadFile(golden)
	require.NoError(t, err)
	var vectors map[string]string
	require.NoError(t, json.Unmarshal(raw, &vectors))
	out := map[string][]byte{}
	for name, value := range vectors {
		b, err := hex.DecodeString(value)
		require.NoError(t, err, name)
		if strings.HasPrefix(name, "DeclaredClassDefinition") {
			// A byte string holding the declaring block number and then the class.
			r := fast.NewReader(b)
			payload, err := r.BytesNoCopy()
			require.NoError(t, err, name)
			b = payload[8:]
		}
		out[name] = b
	}
	return out
}

// loadRecords returns the records of the benchmark corpus by file name, headers without
// their events bloom. corpus "-" means none.
func loadRecords(t *testing.T, corpus string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	if corpus == "-" {
		return out
	}
	files, err := filepath.Glob(filepath.Join(corpus, "*.bin"))
	require.NoError(t, err)
	for _, file := range files {
		b, err := os.ReadFile(file)
		require.NoError(t, err)
		name := strings.TrimSuffix(filepath.Base(file), ".bin")
		if strings.HasPrefix(name, "header") {
			b = withoutKey(t, b, "EventsBloom")
			var h core.Header
			require.NoError(t, fxamacker.Unmarshal(b, &h), file)
		}
		out[name] = b
	}
	return out
}
