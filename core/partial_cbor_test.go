package core

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	bloom "github.com/bits-and-blooms/bloom/v3"
	cbor "github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

// cborKeys returns the CBOR map keys a struct encodes to: field name unless a cbor tag renames it,
// cbor:"-" dropped, unexported skipped, embedded structs flattened.
func cborKeys(t reflect.Type) map[string]struct{} {
	keys := map[string]struct{}{}
	for field := range t.Fields() {
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			for key := range cborKeys(field.Type) {
				keys[key] = struct{}{}
			}
			continue
		}
		if field.PkgPath != "" { // unexported
			continue
		}
		name := field.Name
		if tag := field.Tag.Get("cbor"); tag != "" {
			tagName, _, _ := strings.Cut(tag, ",")
			if tagName == "-" {
				continue
			}
			if tagName != "" {
				name = tagName
			}
		}
		keys[name] = struct{}{}
	}
	return keys
}

// TestPartialSkeletonMatchesHeader guards that the skeleton names every Header field; a missing one
// sends its wire key down the allocating unmatched-key path. A new Header field fails here.
func TestPartialSkeletonMatchesHeader(t *testing.T) {
	headerKeys := cborKeys(reflect.TypeFor[Header]())
	skeletonKeys := cborKeys(reflect.TypeFor[discardedHeaderSkeleton]())
	require.Equal(t, headerKeys, skeletonKeys,
		"discardedHeaderSkeleton must name every Header field (fix discardedCBOR fields to match)")
}

// TestHeaderProjectionsCoverEveryWireKey is the strict guard: it decodes a real header into each
// projection with unknown-field errors on, so the real decoder flags any unmatched wire key. Unlike
// cborKeys it also catches tag-option drift (keyasint, toarray) with an unchanged field name.
func TestHeaderProjectionsCoverEveryWireKey(t *testing.T) {
	strict, err := cbor.DecOptions{ExtraReturnErrors: cbor.ExtraDecErrorUnknownField}.DecMode()
	require.NoError(t, err)

	data := sampleHeaderBytes(t)
	for _, target := range []any{
		&discardedHeaderSkeleton{},
		&headerHashProjection{},
		&headerGlobalStateRootProjection{},
		&headerTransactionCountProjection{},
		&headerTimestampProjection{},
		&headerEventsBloomProjection{},
		&headerHashAndStateRootProjection{},
	} {
		require.NoErrorf(t, strict.Unmarshal(data, target),
			"%T leaves a Header wire key unmatched (field added, or a tag name/option changed)", target)
	}

	// The guard is not vacuous: a projection that omits fields (the pre-fix shape) must be rejected.
	var omitting struct{ Hash *felt.Felt }
	require.Error(t, strict.Unmarshal(data, &omitting),
		"strict mode must reject a projection that does not cover every Header wire key")
}

// TestHeaderProjectionsCoverEveryKey asserts each concrete projection covers every wire key of
// Header, so no key hits the allocating unmatched path.
func TestHeaderProjectionsCoverEveryKey(t *testing.T) {
	headerKeys := cborKeys(reflect.TypeFor[Header]())
	for _, projection := range []reflect.Type{
		reflect.TypeFor[headerHashProjection](),
		reflect.TypeFor[headerGlobalStateRootProjection](),
		reflect.TypeFor[headerTransactionCountProjection](),
		reflect.TypeFor[headerTimestampProjection](),
		reflect.TypeFor[headerEventsBloomProjection](),
		reflect.TypeFor[headerHashAndStateRootProjection](),
	} {
		require.Equal(t, headerKeys, cborKeys(projection),
			"%s must cover exactly the Header keys", projection)
	}
}

// Local stand-ins for a source and its projection, used to prove the guards above actually fail on
// drift, the same machinery would catch the same drift on Header, with no production change.

type driftSource struct {
	A *felt.Felt
	B *felt.Felt
}

type driftSkeletonMissingField struct { // a skeleton that "forgot" field B
	A discardedCBOR
}

type driftSourceRenamedTag struct {
	A *felt.Felt `cbor:"alpha"`
}

type driftSkeletonWrongTag struct { // same field, wrong tag name
	A discardedCBOR `cbor:"beta"`
}

type driftSourceKeyAsInt struct {
	A *felt.Felt `cbor:"1,keyasint"` // wire key is the integer 1
}

type driftProjectionStringKey struct {
	A discardedCBOR `cbor:"1"` // expects the string key "1" — keyasint dropped
}

type driftProjectionIntKey struct {
	A discardedCBOR `cbor:"1,keyasint"` // correctly matches the integer wire key
}

// TestReflectionGuardCatchesFieldDrift proves the cborKeys equality guard fails when a projection
// drops or renames a field vs its source (the "new Header field, stale skeleton" regression).
func TestReflectionGuardCatchesFieldDrift(t *testing.T) {
	require.NotEqual(t,
		cborKeys(reflect.TypeFor[driftSource]()),
		cborKeys(reflect.TypeFor[driftSkeletonMissingField]()),
		"a skeleton missing a field must not match its source's key set")

	require.NotEqual(t,
		cborKeys(reflect.TypeFor[driftSourceRenamedTag]()),
		cborKeys(reflect.TypeFor[driftSkeletonWrongTag]()),
		"a renamed tag must change the key set")
}

// TestStrictGuardCatchesKeyAsIntDrift proves the strict guard catches a keyasint change (string to
// integer wire key) that cborKeys is blind to — hence the strict guard uses the real decoder.
func TestStrictGuardCatchesKeyAsIntDrift(t *testing.T) {
	// Blind spot: cborKeys compares only the tag name, so keyasint vs plain look identical.
	require.Equal(t,
		cborKeys(reflect.TypeFor[driftSourceKeyAsInt]()),
		cborKeys(reflect.TypeFor[driftProjectionStringKey]()),
		"cborKeys cannot see the keyasint option — this is its documented blind spot")

	// Ground truth: encode with the integer key, then strict-decode.
	data, err := encoder.Marshal(&driftSourceKeyAsInt{A: new(felt.Felt).SetUint64(7)})
	require.NoError(t, err)
	strict, err := cbor.DecOptions{ExtraReturnErrors: cbor.ExtraDecErrorUnknownField}.DecMode()
	require.NoError(t, err)

	// The string-key projection does not match the integer wire key → strict decode errors.
	require.Error(t, strict.Unmarshal(data, &driftProjectionStringKey{}),
		"strict decode must reject an integer wire key that the projection expects as a string")
	// The matching keyasint projection decodes cleanly.
	require.NoError(t, strict.Unmarshal(data, &driftProjectionIntKey{}),
		"a projection whose keyasint matches the source must decode without error")
}

// bytesPerDecode reports average bytes allocated per call, via
// cumulative runtime counters that GC does not reset.
func bytesPerDecode(f func()) float64 {
	const runs = 500
	f() // warm up: pay any one-time allocation before measuring
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range runs {
		f()
	}
	runtime.ReadMemStats(&after)
	return float64(after.TotalAlloc-before.TotalAlloc) / runs
}

// headerProjectionCase pairs each partial header projection (discard) with the pre-fix
// field-omitting projection it replaces, shared by the allocation guard and the benchmark.
type headerProjectionCase struct {
	name          string
	fieldOmitting func([]byte) // pre-fix: omits fields → allocating unmatched-key path
	discard       func([]byte) // current: names every field, discards the unwanted ones
}

func headerProjectionCases() []headerProjectionCase {
	return []headerProjectionCase{
		{
			"hash",
			func(d []byte) { var h struct{ Hash *felt.Felt }; _ = encoder.Unmarshal(d, &h) },
			func(d []byte) { var h headerHashProjection; _ = encoder.Unmarshal(d, &h) },
		},
		{
			"global_state_root",
			func(d []byte) { var h struct{ GlobalStateRoot *felt.Felt }; _ = encoder.Unmarshal(d, &h) },
			func(d []byte) { var h headerGlobalStateRootProjection; _ = encoder.Unmarshal(d, &h) },
		},
		{
			"transaction_count",
			func(d []byte) { var h struct{ TransactionCount uint64 }; _ = encoder.Unmarshal(d, &h) },
			func(d []byte) { var h headerTransactionCountProjection; _ = encoder.Unmarshal(d, &h) },
		},
		{
			"timestamp",
			func(d []byte) { var h struct{ Timestamp *uint64 }; _ = encoder.Unmarshal(d, &h) },
			func(d []byte) { var h headerTimestampProjection; _ = encoder.Unmarshal(d, &h) },
		},
		{
			"events_bloom",
			func(d []byte) {
				var h struct{ EventsBloom *bloom.BloomFilter }
				_ = encoder.Unmarshal(d, &h)
			},
			func(d []byte) { var h headerEventsBloomProjection; _ = encoder.Unmarshal(d, &h) },
		},
		{
			"hash_and_state_root",
			func(d []byte) {
				var h struct {
					Hash            *felt.Felt
					GlobalStateRoot *felt.Felt
				}
				_ = encoder.Unmarshal(d, &h)
			},
			func(d []byte) { var h headerHashAndStateRootProjection; _ = encoder.Unmarshal(d, &h) },
		},
	}
}

// TestDiscardReducesReadAllocations guards, for every partial header projection, that the discard
// form allocates fewer objects and bytes than the pre-fix field-omitting form on the same header.
func TestDiscardReducesReadAllocations(t *testing.T) {
	data := sampleHeaderBytes(t)
	for _, tc := range headerProjectionCases() {
		t.Run(tc.name, func(t *testing.T) {
			omittingAllocs := testing.AllocsPerRun(300, func() { tc.fieldOmitting(data) })
			discardAllocs := testing.AllocsPerRun(300, func() { tc.discard(data) })
			omittingBytes := bytesPerDecode(func() { tc.fieldOmitting(data) })
			discardBytes := bytesPerDecode(func() { tc.discard(data) })

			t.Logf("allocs %.0f → %.0f   bytes %.0f → %.0f B",
				omittingAllocs, discardAllocs, omittingBytes, discardBytes)

			require.Less(t, discardAllocs, omittingAllocs, "discard must allocate fewer objects")
			require.Less(t, discardBytes, omittingBytes, "discard must allocate fewer bytes")
		})
	}
}

// TestHeaderHashDecodeAllocTripwire pins the absolute header/hash decode alloc counts as a canary.
// They depend on the decoder and Header's key set, so a change means an upstream decoder shift
// (a fxamacker/cbor upgrade) or a Header field added/removed. Re-verify, then bump if intended.
func TestHeaderHashDecodeAllocTripwire(t *testing.T) {
	data := sampleHeaderBytes(t)
	fieldOmitting := testing.AllocsPerRun(300, func() {
		var h struct{ Hash *felt.Felt }
		_ = encoder.Unmarshal(data, &h)
	})
	discard := testing.AllocsPerRun(300, func() {
		var h headerHashProjection
		_ = encoder.Unmarshal(data, &h)
	})
	require.Equal(t, 32.0, fieldOmitting, "field-omitting alloc count changed; see comment above")
	require.Equal(t, 2.0, discard, "discard alloc count changed; see comment above")
}

// sampleHeaderBytes returns the CBOR encoding of a real mainnet header (block 9706496).
func sampleHeaderBytes(tb testing.TB) []byte {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "header_9706496.cbor"))
	require.NoError(tb, err)
	return data
}

// BenchmarkPartialHeaderProjections covers every partial header projection, before (field-omitting)
// and after (discard).
func BenchmarkPartialHeaderProjections(b *testing.B) {
	data := sampleHeaderBytes(b)
	for _, tc := range headerProjectionCases() {
		b.Run(tc.name, func(b *testing.B) {
			b.Run("field_omitting", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					tc.fieldOmitting(data)
				}
			})
			b.Run("discard", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					tc.discard(data)
				}
			})
		})
	}
}
