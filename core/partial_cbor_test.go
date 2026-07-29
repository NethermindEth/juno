package core

import (
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

// cborKeys returns a struct's CBOR map keys from tag names (blind to options like keyasint). Embed
// flattening assumes untagged by-value embeds (as these types use), matching the encoder.
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

// TestPartialSkeletonMatchesHeader fails if the skeleton omits a Header field (its key would then
// hit the allocating unmatched-key path).
func TestPartialSkeletonMatchesHeader(t *testing.T) {
	headerKeys := cborKeys(reflect.TypeFor[Header]())
	skeletonKeys := cborKeys(reflect.TypeFor[discardedHeaderSkeleton]())
	require.Equal(t, headerKeys, skeletonKeys,
		"discardedHeaderSkeleton must name every Header field (fix discardedCBOR fields to match)")
}

// TestHeaderProjectionsCoverEveryWireKey decodes with unknown-field errors on, so the real decoder
// flags any unmatched wire key — catching tag-option drift (keyasint, toarray) that cborKeys can't.
// The strict DecMode lacks the encoder's tag set; valid only because Header has no tagged types.
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

// TestHeaderProjectionsCoverEveryKey asserts each projection covers every Header wire key.
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

// Stand-in source/projection pairs used to prove the guards above actually fail on drift.

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

// TestReflectionGuardCatchesFieldDrift proves the cborKeys guard fails on a dropped/renamed field.
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

// TestStrictGuardCatchesKeyAsIntDrift proves the strict guard catches a keyasint change that
// cborKeys is blind to — the reason the strict guard uses the real decoder.
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

// bytesPerDecode reports average bytes allocated per call (TotalAlloc delta, unaffected by GC).
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

// headerProjectionCase pairs each discard projection with the field-omitting shape it replaces.
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

// TestDiscardReducesReadAllocations is the canary that the discard trick still pays off: for every
// projection the discard form must allocate strictly fewer objects and bytes than the naive
// field-omitting form. If the two converge — unmatched keys became free upstream, or the trick
// stopped helping — this fails, signalling the optimization is dead.
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

// sampleHeader is a fully populated Header (production-size EventsBloom) with small distinct
// values, so its encoding exercises all 16 keys and decode assertions can check them.
func sampleHeader() *Header {
	return &Header{
		Hash:             felt.NewFromUint64[felt.Felt](1),
		ParentHash:       felt.NewFromUint64[felt.Felt](2),
		Number:           100,
		GlobalStateRoot:  felt.NewFromUint64[felt.Felt](3),
		SequencerAddress: felt.NewFromUint64[felt.Felt](4),
		TransactionCount: 10,
		EventCount:       50,
		Timestamp:        123456,
		ProtocolVersion:  "0.13.2",
		EventsBloom:      bloom.New(EventsBloomLength, EventsBloomHashFuncs),
		L1GasPriceETH:    felt.NewFromUint64[felt.Felt](5),
		Signatures:       [][]*felt.Felt{{felt.NewFromUint64[felt.Felt](6)}},
		L1GasPriceSTRK:   felt.NewFromUint64[felt.Felt](7),
		L1DAMode:         Blob,
		L1DataGasPrice: &GasPrice{
			PriceInWei: felt.NewFromUint64[felt.Felt](8),
			PriceInFri: felt.NewFromUint64[felt.Felt](9),
		},
		L2GasPrice: &GasPrice{
			PriceInWei: felt.NewFromUint64[felt.Felt](10),
			PriceInFri: felt.NewFromUint64[felt.Felt](11),
		},
	}
}

// sampleHeaderBytes marshals a live Header, so the wire key set tracks the struct — a new field
// appears here automatically.
func sampleHeaderBytes(tb testing.TB) []byte {
	tb.Helper()
	data, err := encoder.Marshal(sampleHeader())
	require.NoError(tb, err)
	return data
}

// TestProjectionsDecodeShadowedField proves the shadowing field receives the value, not just that
// the key sets line up — a change in cbor's embed precedence would slip past the key-set guards.
func TestProjectionsDecodeShadowedField(t *testing.T) {
	header := sampleHeader()
	data := sampleHeaderBytes(t)

	var hash headerHashProjection
	require.NoError(t, encoder.Unmarshal(data, &hash))
	require.True(t, hash.Hash.Equal(header.Hash))

	var stateRoot headerGlobalStateRootProjection
	require.NoError(t, encoder.Unmarshal(data, &stateRoot))
	require.True(t, stateRoot.GlobalStateRoot.Equal(header.GlobalStateRoot))

	var txCount headerTransactionCountProjection
	require.NoError(t, encoder.Unmarshal(data, &txCount))
	require.Equal(t, header.TransactionCount, txCount.TransactionCount)

	var timestamp headerTimestampProjection
	require.NoError(t, encoder.Unmarshal(data, &timestamp))
	require.NotNil(t, timestamp.Timestamp)
	require.Equal(t, header.Timestamp, *timestamp.Timestamp)

	var eventsBloom headerEventsBloomProjection
	require.NoError(t, encoder.Unmarshal(data, &eventsBloom))
	require.NotNil(t, eventsBloom.EventsBloom)

	var hashAndRoot headerHashAndStateRootProjection
	require.NoError(t, encoder.Unmarshal(data, &hashAndRoot))
	require.True(t, hashAndRoot.Hash.Equal(header.Hash))
	require.True(t, hashAndRoot.GlobalStateRoot.Equal(header.GlobalStateRoot))
}

// BenchmarkPartialHeaderProjections benchmarks each header projection, field_omitting vs discard.
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
