//go:build sweep

// Package sweep decodes every routed record of a mainnet database with both fxamacker and the
// wrapper and requires equal results. It only runs with the sweep build tag; see run.sh.
package sweep

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
)

const defaultDB = "/mnt/dev/juno-benchmark/mainnet-12945535/data"

// maxReported bounds how many mismatches are printed in full.
const maxReported = 20

// scribble overwrites the wrapper's input after it decodes, so a value that kept a reference
// into the input no longer equals fxamacker's.
const scribble = 0xa5

type job struct {
	kind string // bucket being swept
	key  []byte
	raw  []byte
}

type kindStats struct {
	count     uint64
	fxTime    time.Duration
	fastTime  time.Duration
	maxSize   int
	sizeTotal uint64
}

type worker struct {
	stats map[string]*kindStats
	buf   []byte
}

type sweeper struct {
	mismatches atomic.Uint64
	failures   atomic.Uint64
	reportMu   sync.Mutex
	report     []string
	processed  atomic.Uint64
	sierraTag  uint64
}

func (s *sweeper) failf(kind string, key []byte, format string, args ...any) {
	s.failures.Add(1)
	s.reportMu.Lock()
	defer s.reportMu.Unlock()
	if len(s.report) < maxReported {
		s.report = append(s.report, fmt.Sprintf("%s key=%x: %s", kind, key, fmt.Sprintf(format, args...)))
	}
}

func (w *worker) stat(kind string) *kindStats {
	st, ok := w.stats[kind]
	if !ok {
		st = new(kindStats)
		w.stats[kind] = st
	}
	return st
}

// compare decodes raw into a fresh T (or *T when viaPointer) with fxamacker and with the
// wrapper, then scribbles over the wrapper's input and requires deep equality. classify, if
// set, refines kind from fxamacker's value.
func compare[T any](s *sweeper, w *worker, kind string, key, raw []byte, viaPointer bool, classify ...func(any) string) {
	if cap(w.buf) < len(raw) {
		w.buf = make([]byte, len(raw))
	}
	buf := w.buf[:len(raw)]
	copy(buf, raw)

	var fxDst, fastDst any
	if viaPointer {
		fxDst, fastDst = new(*T), new(*T)
	} else {
		fxDst, fastDst = new(T), new(T)
	}
	start := time.Now()
	fxErr := fxamacker.Unmarshal(raw, fxDst)
	mid := time.Now()
	fastErr := cbor.Unmarshal(buf, fastDst)
	end := time.Now()
	for i := range buf {
		buf[i] = scribble
	}

	if fxErr == nil && len(classify) > 0 {
		kind = classify[0](reflect.ValueOf(fxDst).Elem().Interface())
	}
	st := w.stat(kind)
	st.count++
	st.sizeTotal += uint64(len(raw))
	st.maxSize = max(st.maxSize, len(raw))
	st.fxTime += mid.Sub(start)
	st.fastTime += end.Sub(mid)

	if fxErr != nil || fastErr != nil {
		s.failf(kind, key, "fxamacker err=%v wrapper err=%v", fxErr, fastErr)
		return
	}
	a, b := reflect.ValueOf(fxDst).Elem().Interface(), reflect.ValueOf(fastDst).Elem().Interface()
	if !reflect.DeepEqual(a, b) {
		s.mismatches.Add(1)
		s.failf(kind, key, "mismatch:\n%s", diff(a, b))
	}
}

// diff renders testify's diff of two values.
func diff(want, got any) string {
	rec := &recorder{}
	assert.Equal(rec, want, got)
	return rec.msg
}

type recorder struct{ msg string }

func (r *recorder) Errorf(format string, args ...any) { r.msg = fmt.Sprintf(format, args...) }

func (s *sweeper) process(w *worker, j job) {
	defer s.processed.Add(1)
	switch j.kind {
	case "header":
		compare[core.Header](s, w, "header", j.key, j.raw, len(j.raw)%2 == 1)
	case "state":
		compare[core.StateUpdate](s, w, "state", j.key, j.raw, len(j.raw)%2 == 1)
	case "class":
		s.class(w, j)
	case "blocktx":
		s.blockTransactions(w, j)
	case "deprecated-receipt":
		compare[core.TransactionReceipt](s, w, "receipt(deprecated bucket)", j.key, j.raw, true)
	case "deprecated-tx":
		s.transaction(w, "(deprecated bucket)", j.key, j.raw)
	default:
		panic(j.kind)
	}
}

func (s *sweeper) class(w *worker, j job) {
	outer := fast.NewReader(j.raw)
	payload, err := outer.Bytes()
	if err != nil || outer.Len() != 0 {
		s.failf("class", j.key, "class envelope: %v", err)
		return
	}
	if len(payload) < 8 {
		s.failf("class", j.key, "class envelope too short: %d bytes", len(payload))
		return
	}
	body := payload[8:]
	r := fast.NewReader(body)
	if tag, err := r.Tag(); err != nil || tag != s.sierraTag {
		// Cairo 0: the wrapper reads the tag, gives up and falls back. Still compared.
		compare[core.ClassDefinition](s, w, "class Cairo 0 (fallback)", j.key, body, false)
		return
	}
	compare[core.ClassDefinition](s, w, "class Sierra", j.key, body, false)
}

func (s *sweeper) transaction(w *worker, suffix string, key, raw []byte) {
	// Count by concrete type, as fxamacker decoded it.
	compare[core.Transaction](s, w, "transaction"+suffix, key, raw, false, func(v any) string {
		name := strings.TrimSuffix(strings.TrimPrefix(fmt.Sprintf("%T", v), "*core."), "Transaction")
		return "transaction " + name + suffix
	})
}

// indexes reads the offsets that start a BlockTransactions entry through the wrapper, as Juno
// does, from a copy it then overwrites, and requires fxamacker's value and remainder.
func (s *sweeper) indexes(w *worker, j job, want core.BlockTransactionsIndexes, wantRest int) {
	buf := bytes.Clone(j.raw)
	var got core.BlockTransactionsIndexes
	rest, err := cbor.UnmarshalFirst(buf, &got)
	restLen := len(rest)
	for i := range buf {
		buf[i] = scribble
	}
	w.stat("block indexes").count++
	switch {
	case err != nil:
		s.failf("block indexes", j.key, "wrapper err=%v", err)
	case restLen != wantRest:
		s.failf("block indexes", j.key, "remainder %d bytes, fxamacker %d", restLen, wantRest)
	case !reflect.DeepEqual(want, got):
		s.mismatches.Add(1)
		s.failf("block indexes", j.key, "mismatch:\n%s", diff(want, got))
	}
}

func (s *sweeper) blockTransactions(w *worker, j job) {
	var idx core.BlockTransactionsIndexes
	rest, err := fxamacker.UnmarshalFirst(j.raw, &idx)
	if err != nil {
		s.failf("blocktx", j.key, "indexes: %v", err)
		return
	}
	s.indexes(w, j, idx, len(rest))
	end := func(offsets []int, i, last int) int {
		if i+1 < len(offsets) {
			return offsets[i+1]
		}
		return last
	}
	txEnd := len(rest)
	if len(idx.Receipts) > 0 {
		txEnd = idx.Receipts[0]
	}
	for i, start := range idx.Transactions {
		stop := end(idx.Transactions, i, txEnd)
		if start < 0 || stop > len(rest) || start > stop {
			s.failf("blocktx", j.key, "bad transaction offsets %d..%d of %d", start, stop, len(rest))
			return
		}
		s.transaction(w, "", j.key, rest[start:stop])
	}
	for i, start := range idx.Receipts {
		stop := end(idx.Receipts, i, len(rest))
		if start < 0 || stop > len(rest) || start > stop {
			s.failf("blocktx", j.key, "bad receipt offsets %d..%d of %d", start, stop, len(rest))
			return
		}
		// Juno reads receipts into a *TransactionReceipt; alternate to cover both routes.
		compare[core.TransactionReceipt](s, w, "receipt", j.key, rest[start:stop], i%2 == 0)
	}
	w.stat("block (BlockTransactions)").count++
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			panic(fmt.Sprintf("%s=%q: %v", name, v, err))
		}
		return n
	}
	return def
}

// TestMainnetSweep decodes every header, state update, transaction, receipt and class.
//
//	SWEEP_DB      database directory (opened read-only)
//	SWEEP_KINDS   comma-separated subset of class,header,state,blocktx,deprecated (default all)
//	SWEEP_STRIDE  sweep every Nth block only (default 1: every block); classes are always all
//	SWEEP_FROM    first block number (default 0)
//	SWEEP_TO      block number to stop before (default: the first missing block)
//	SWEEP_WORKERS decoding goroutines (default GOMAXPROCS)
func TestMainnetSweep(t *testing.T) {
	cfg := loadConfig(t)
	p, err := pebble.Open(cfg.path, &pebble.Options{ReadOnly: true})
	if err != nil {
		t.Fatalf("open %s: %v (a running sweep or node holds its lock)", cfg.path, err)
	}
	defer p.Close()

	tag, ok := cbor.TagOf(reflect.TypeFor[core.SierraClass]())
	if !ok {
		t.Fatal("SierraClass has no tag")
	}
	s := &sweeper{sierraTag: tag}
	started := time.Now()

	jobs := make(chan job, 256)
	var wg sync.WaitGroup
	results := make([]*worker, cfg.workers)
	for i := range results {
		w := &worker{stats: map[string]*kindStats{}}
		results[i] = w
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				s.process(w, j)
			}
		}()
	}
	var phase atomic.Value
	phase.Store("start")
	done := make(chan struct{})
	go s.progress(&phase, started, done)

	queueAll(t, p, cfg, jobs, &phase)
	close(jobs)
	wg.Wait()
	close(done)
	elapsed := time.Since(started)

	total := merge(results)
	out := summary(cfg, elapsed, total, s)
	t.Log("\n" + out)
	if f := os.Getenv("SWEEP_SUMMARY"); f != "" {
		if err := os.WriteFile(f, []byte(out), 0o600); err != nil {
			t.Error(err)
		}
	}
	for _, line := range s.report {
		t.Error(line)
	}
	if n := s.failures.Load(); n != 0 {
		t.Errorf("%d records failed (%d mismatches)", n, s.mismatches.Load())
	}
}

type config struct {
	path             string
	stride, from, to uint64
	ranged           bool
	workers          int
	kinds            map[string]bool
}

func (c config) want(kind string) bool { return len(c.kinds) == 0 || c.kinds[kind] }

func loadConfig(t *testing.T) config {
	cfg := config{path: os.Getenv("SWEEP_DB"), kinds: map[string]bool{}}
	if cfg.path == "" {
		cfg.path = defaultDB
	}
	if _, err := os.Stat(cfg.path); err != nil {
		t.Skipf("no database at %s: %v", cfg.path, err)
	}
	cfg.stride = uint64(envInt("SWEEP_STRIDE", 1))
	cfg.from, cfg.to = uint64(envInt("SWEEP_FROM", 0)), uint64(envInt("SWEEP_TO", -1))
	cfg.ranged = cfg.from != 0 || os.Getenv("SWEEP_TO") != ""
	cfg.workers = envInt("SWEEP_WORKERS", 0)
	if cfg.workers <= 0 {
		cfg.workers = runtime.GOMAXPROCS(0)
	}
	for _, k := range strings.Split(os.Getenv("SWEEP_KINDS"), ",") {
		if k = strings.TrimSpace(k); k != "" {
			cfg.kinds[k] = true
		}
	}
	return cfg
}

func (s *sweeper) progress(phase *atomic.Value, started time.Time, done <-chan struct{}) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-done:
			return
		case <-tick.C:
			fmt.Fprintf(os.Stderr, "sweep: %s elapsed=%s jobs=%d failures=%d\n",
				phase.Load(), time.Since(started).Round(time.Second), s.processed.Load(), s.failures.Load())
		}
	}
}

// queueAll queues the records of every bucket the configuration asks for.
func queueAll(t *testing.T, p *pebble.DB, cfg config, jobs chan<- job, phase *atomic.Value) {
	if cfg.want("class") {
		scan(t, p, "class", db.Class, jobs, phase)
	}
	for _, b := range []struct {
		kind   string
		bucket db.Bucket
	}{{"header", db.BlockHeadersByNumber}, {"state", db.StateUpdatesByBlockNumber}, {"blocktx", db.BlockTransactions}} {
		if !cfg.want(b.kind) {
			continue
		}
		if cfg.stride > 1 || cfg.ranged {
			strided(t, p, b.kind, b.bucket, cfg, jobs, phase)
		} else {
			scan(t, p, b.kind, b.bucket, jobs, phase)
		}
	}
	if cfg.want("deprecated") {
		scan(t, p, "deprecated-receipt", db.DeprecatedReceiptsByBlockNumberAndIndex, jobs, phase)
		scan(t, p, "deprecated-tx", db.DeprecatedTransactionsByBlockNumberAndIndex, jobs, phase)
	}
}

// scan queues every record of a bucket.
func scan(t *testing.T, p *pebble.DB, kind string, bucket db.Bucket, jobs chan<- job, phase *atomic.Value) {
	phase.Store(kind)
	begin := time.Now()
	n := 0
	it, err := p.NewIter(&pebble.IterOptions{LowerBound: bucket.Key(), UpperBound: []byte{byte(bucket) + 1}})
	if err != nil {
		t.Fatal(err)
	}
	for it.First(); it.Valid(); it.Next() {
		jobs <- job{kind: kind, key: bytes.Clone(it.Key()), raw: bytes.Clone(it.Value())}
		n++
	}
	if err := it.Error(); err != nil {
		t.Fatal(err)
	}
	if err := it.Close(); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "sweep: queued %d %s records in %s\n", n, kind, time.Since(begin).Round(time.Millisecond))
}

func merge(results []*worker) map[string]*kindStats {
	total := map[string]*kindStats{}
	for _, w := range results {
		for k, st := range w.stats {
			acc, ok := total[k]
			if !ok {
				acc = new(kindStats)
				total[k] = acc
			}
			acc.count += st.count
			acc.sizeTotal += st.sizeTotal
			acc.maxSize = max(acc.maxSize, st.maxSize)
			acc.fxTime += st.fxTime
			acc.fastTime += st.fastTime
		}
	}
	return total
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func summary(cfg config, elapsed time.Duration, total map[string]*kindStats, s *sweeper) string {
	var out strings.Builder
	fmt.Fprintf(&out, "sweep of %s (blocks %d..%d, stride %d, %d workers) took %s\n",
		cfg.path, cfg.from, cfg.to, cfg.stride, cfg.workers, elapsed.Round(time.Second))
	fmt.Fprintf(&out, "%-36s %12s %10s %10s %12s %12s %8s\n", "kind", "records", "avg B", "max B", "fx avg", "wrapper avg", "speedup")
	for _, k := range sortedKeys(total) {
		st := total[k]
		switch {
		case st.count == 0:
		case st.sizeTotal == 0:
			fmt.Fprintf(&out, "%-36s %12d\n", k, st.count)
		default:
			fx := st.fxTime / time.Duration(st.count)
			fa := st.fastTime / time.Duration(st.count)
			fmt.Fprintf(&out, "%-36s %12d %10d %10d %12s %12s %7.2fx\n", k, st.count, st.sizeTotal/st.count, st.maxSize,
				fx, fa, float64(st.fxTime)/float64(max(st.fastTime, 1)))
		}
	}
	fmt.Fprintf(&out, "mismatches=%d failures=%d\n", s.mismatches.Load(), s.failures.Load())
	return out.String()
}

// strided queues the entries of every stride-th block in [from, to) of a block-keyed bucket by
// point lookups, which is much faster than iterating the whole bucket for a sample, and lets a
// sweep be split into ranges. It stops at the first missing block.
func strided(t *testing.T, p *pebble.DB, kind string, bucket db.Bucket, cfg config, jobs chan<- job, phase *atomic.Value) {
	phase.Store(kind)
	begin := time.Now()
	n := 0
	for number := cfg.from; number < cfg.to; number += cfg.stride {
		var key []byte
		if bucket == db.BlockTransactions {
			encoded, err := fxamacker.Marshal(number)
			if err != nil {
				t.Fatal(err)
			}
			key = bucket.Key(encoded)
		} else {
			key = bucket.Key(binary.BigEndian.AppendUint64(nil, number))
		}
		v, closer, err := p.Get(key)
		if errors.Is(err, pebble.ErrNotFound) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		jobs <- job{kind: kind, key: key, raw: bytes.Clone(v)}
		if err := closer.Close(); err != nil {
			t.Fatal(err)
		}
		n++
	}
	fmt.Fprintf(os.Stderr, "sweep: queued %d %s records in %s\n", n, kind, time.Since(begin).Round(time.Millisecond))
}
