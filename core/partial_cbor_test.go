package core

import (
	"fmt"
	"reflect"
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

// assertSkeletonNamesAllFields fails if the skeleton's CBOR key set differs from its source's — a
// dropped or renamed field would send that key back to the allocating unmatched-key path.
func assertSkeletonNamesAllFields(t *testing.T, source, skeleton reflect.Type) {
	t.Helper()
	require.Equalf(t, cborKeys(source), cborKeys(skeleton),
		"%s must name every %s field (fix discardedCBOR fields to match)", skeleton, source)
}

// assertProjectionsCoverSource fails if any projection's CBOR key set differs from its source's.
func assertProjectionsCoverSource(t *testing.T, source reflect.Type, projections ...reflect.Type) {
	t.Helper()
	sourceKeys := cborKeys(source)
	for _, projection := range projections {
		require.Equalf(t, sourceKeys, cborKeys(projection),
			"%s must cover exactly the %s keys", projection, source)
	}
}

// assertProjectionsCoverEveryWireKey strict-decodes real wire bytes into each projection so the
// decoder flags any unmatched key, catching top-level tag-option drift (keyasint, toarray) that
// cborKeys can't. Only top-level keys matter: discarded fields throw their nested value away, so
// the projection is only responsible for lining up the keys it names.
//
// Non-vacuous: see TestStrictGuardCatchesKeyAsIntDrift.
func assertProjectionsCoverEveryWireKey(t *testing.T, data []byte, projections ...any) {
	t.Helper()
	strict, err := cbor.DecOptions{ExtraReturnErrors: cbor.ExtraDecErrorUnknownField}.DecMode()
	require.NoError(t, err)
	for _, projection := range projections {
		require.NoErrorf(t, strict.Unmarshal(data, projection),
			"%T leaves a wire key unmatched (field added, or a tag name/option changed)", projection)
	}
}

// TestPartialSkeletonMatchesHeader fails if the skeleton omits a Header field (its key would then
// hit the allocating unmatched-key path).
func TestPartialSkeletonMatchesHeader(t *testing.T) {
	assertSkeletonNamesAllFields(t,
		reflect.TypeFor[Header](),
		reflect.TypeFor[discardedHeaderSkeleton](),
	)
}

// TestHeaderProjectionsCoverEveryWireKey guards against tag-option drift that cborKeys is blind to.
func TestHeaderProjectionsCoverEveryWireKey(t *testing.T) {
	assertProjectionsCoverEveryWireKey(t, sampleHeaderBytes(t),
		&discardedHeaderSkeleton{},
		&headerHashProjection{},
		&headerGlobalStateRootProjection{},
		&headerTransactionCountProjection{},
		&headerTimestampProjection{},
		&headerEventsBloomProjection{},
		&headerHashAndStateRootProjection{},
	)
}

// TestHeaderProjectionsCoverEveryKey asserts each projection covers every Header wire key.
func TestHeaderProjectionsCoverEveryKey(t *testing.T) {
	assertProjectionsCoverSource(t, reflect.TypeFor[Header](),
		reflect.TypeFor[headerHashProjection](),
		reflect.TypeFor[headerGlobalStateRootProjection](),
		reflect.TypeFor[headerTransactionCountProjection](),
		reflect.TypeFor[headerTimestampProjection](),
		reflect.TypeFor[headerEventsBloomProjection](),
		reflect.TypeFor[headerHashAndStateRootProjection](),
	)
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
// projection the discard form must allocate strictly fewer objects than the naive field-omitting
// form. If the two converge — unmatched keys became free upstream, or the trick stopped helping —
// this fails, signalling the optimization is dead.
func TestDiscardReducesReadAllocations(t *testing.T) {
	data := sampleHeaderBytes(t)
	for _, tc := range headerProjectionCases() {
		t.Run(tc.name, func(t *testing.T) {
			omittingAllocs := testing.AllocsPerRun(300, func() { tc.fieldOmitting(data) })
			discardAllocs := testing.AllocsPerRun(300, func() { tc.discard(data) })
			t.Logf("allocs %.0f → %.0f", omittingAllocs, discardAllocs)
			require.Less(t, discardAllocs, omittingAllocs, "discard must allocate fewer objects")
		})
	}
}

// sampleHeader is a fully populated Header (production-size EventsBloom) with small distinct
// values, so its encoding exercises all 16 keys and decode assertions can check them.
func sampleHeader() *Header {
	eventsBloom := bloom.New(EventsBloomLength, EventsBloomHashFuncs)
	eventsBloom.Add([]byte("sample-event"))
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
		EventsBloom:      eventsBloom,
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

	const shadowMsg = "shadowing field must receive the wire value, not discardedCBOR"

	var hash headerHashProjection
	require.NoError(t, encoder.Unmarshal(data, &hash))
	require.Equal(t, header.Hash, hash.Hash, shadowMsg)

	var stateRoot headerGlobalStateRootProjection
	require.NoError(t, encoder.Unmarshal(data, &stateRoot))
	require.Equal(t, header.GlobalStateRoot, stateRoot.GlobalStateRoot, shadowMsg)

	var txCount headerTransactionCountProjection
	require.NoError(t, encoder.Unmarshal(data, &txCount))
	require.Equal(t, header.TransactionCount, txCount.TransactionCount, shadowMsg)

	var timestamp headerTimestampProjection
	require.NoError(t, encoder.Unmarshal(data, &timestamp))
	require.NotNil(t, timestamp.Timestamp, shadowMsg)
	require.Equal(t, header.Timestamp, *timestamp.Timestamp, shadowMsg)

	var eventsBloom headerEventsBloomProjection
	require.NoError(t, encoder.Unmarshal(data, &eventsBloom))
	require.NotNil(t, eventsBloom.EventsBloom, shadowMsg)
	require.True(t, eventsBloom.EventsBloom.Test([]byte("sample-event")),
		"decoded bloom must carry the added element, not be a fresh empty filter")

	var hashAndRoot headerHashAndStateRootProjection
	require.NoError(t, encoder.Unmarshal(data, &hashAndRoot))
	require.Equal(t, header.Hash, hashAndRoot.Hash, shadowMsg)
	require.Equal(t, header.GlobalStateRoot, hashAndRoot.GlobalStateRoot, shadowMsg)
}

// TestProjectionsAreDecodeOnly proves marshaling a projection fails loudly rather than emitting a
// corrupt record from its discarded fields.
func TestProjectionsAreDecodeOnly(t *testing.T) {
	_, err := encoder.Marshal(&headerHashProjection{Hash: felt.NewFromUint64[felt.Felt](1)})
	require.ErrorIs(t, err, errDiscardedCBORMarshal)
}

// --- Receipt projections ---

// sampleReceipt is a reverted receipt with distinct values, so its encoding exercises every
// TransactionReceipt key and the execution-status assertions can check Reverted and RevertReason.
func sampleReceipt() *TransactionReceipt {
	return &TransactionReceipt{
		Fee:             felt.NewFromUint64[felt.Felt](1),
		TransactionHash: felt.NewFromUint64[felt.Felt](2),
		Reverted:        true,
		RevertReason:    "sample revert reason",
	}
}

// sampleReceiptBytes marshals a live TransactionReceipt, so the wire key set tracks the struct.
func sampleReceiptBytes(tb testing.TB) []byte {
	tb.Helper()
	data, err := encoder.Marshal(sampleReceipt())
	require.NoError(tb, err)
	return data
}

// TestPartialSkeletonMatchesReceipt fails if the skeleton omits a TransactionReceipt field (its key
// would then hit the allocating unmatched-key path).
func TestPartialSkeletonMatchesReceipt(t *testing.T) {
	assertSkeletonNamesAllFields(t,
		reflect.TypeFor[TransactionReceipt](),
		reflect.TypeFor[discardedReceiptSkeleton](),
	)
}

// TestReceiptProjectionCoversEveryKey asserts the execution-status projection covers every
// TransactionReceipt wire key.
func TestReceiptProjectionCoversEveryKey(t *testing.T) {
	assertProjectionsCoverSource(t, reflect.TypeFor[TransactionReceipt](),
		reflect.TypeFor[receiptExecutionStatusProjection]())
}

// TestReceiptProjectionCoversEveryWireKey guards against tag-option drift that cborKeys
// is blind to.
func TestReceiptProjectionCoversEveryWireKey(t *testing.T) {
	assertProjectionsCoverEveryWireKey(t, sampleReceiptBytes(t),
		&discardedReceiptSkeleton{},
		&receiptExecutionStatusProjection{},
	)
}

// TestExecutionStatusProjectionDecodesShadowedFields proves the shadowing fields receive the wire
// values, not discardedCBOR — a change in cbor's embed precedence would slip past the key guards.
func TestExecutionStatusProjectionDecodesShadowedFields(t *testing.T) {
	receipt := sampleReceipt()
	var projection receiptExecutionStatusProjection
	require.NoError(t, encoder.Unmarshal(sampleReceiptBytes(t), &projection))
	require.Equal(t, receipt.Reverted, projection.Reverted,
		"Reverted must receive the wire value, not discardedCBOR")
	require.Equal(t, receipt.RevertReason, projection.RevertReason,
		"RevertReason must receive the wire value, not discardedCBOR")
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

// BenchmarkExecutionStatusProjection compares decoding the execution-status subset via the naive
// field-omitting struct (every unwanted key hits the allocating unmatched-key path) against the
// discard projection (every key named, unwanted ones discarded without allocation).
func BenchmarkExecutionStatusProjection(b *testing.B) {
	data := sampleReceiptBytes(b)
	b.Run("field_omitting", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var r struct {
				Reverted     bool
				RevertReason string
			}
			_ = encoder.Unmarshal(data, &r)
		}
	})
	b.Run("discard", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var r receiptExecutionStatusProjection
			_ = encoder.Unmarshal(data, &r)
		}
	})
}

// --- Transaction projections ---

// sampleTransactions returns a fully-populated instance of every transaction type, so each encoding
// carries every key that type can write — including InvokeTransaction.ProofFacts, whose omitempty
// would drop the key if left nil.
func sampleTransactions() []Transaction {
	felts := []felt.Felt{*felt.NewFromUint64[felt.Felt](1), *felt.NewFromUint64[felt.Felt](2)}
	deploy := DeployTransaction{
		TransactionHash:     felt.NewFromUint64[felt.Felt](1),
		ContractAddressSalt: felt.NewFromUint64[felt.Felt](2),
		ContractAddress:     felt.NewFromUint64[felt.Felt](3),
		ClassHash:           felt.NewFromUint64[felt.Felt](4),
		ConstructorCallData: felts,
		Version:             new(TransactionVersion).SetUint64(3),
	}
	resourceBounds := map[Resource]ResourceBounds{ResourceL1Gas: {MaxAmount: 1, MaxPricePerUnit: felt.NewFromUint64[felt.Felt](2)}}
	return []Transaction{
		&deploy,
		&DeployAccountTransaction{
			DeployTransaction:    deploy,
			MaxFee:               felt.NewFromUint64[felt.Felt](5),
			TransactionSignature: felts,
			Nonce:                felt.NewFromUint64[felt.Felt](6),
			ResourceBounds:       resourceBounds,
			Tip:                  7,
			PaymasterData:        felts,
			NonceDAMode:          DAModeL1,
			FeeDAMode:            DAModeL1,
		},
		&InvokeTransaction{
			TransactionHash:       felt.NewFromUint64[felt.Felt](1),
			CallData:              felts,
			TransactionSignature:  felts,
			MaxFee:                felt.NewFromUint64[felt.Felt](2),
			ContractAddress:       felt.NewFromUint64[felt.Felt](3),
			Version:               felt.NewFromUint64[TransactionVersion](3),
			EntryPointSelector:    felt.NewFromUint64[felt.Felt](4),
			Nonce:                 felt.NewFromUint64[felt.Felt](5),
			SenderAddress:         felt.NewFromUint64[felt.Felt](6),
			ResourceBounds:        resourceBounds,
			Tip:                   7,
			PaymasterData:         felts,
			AccountDeploymentData: felts,
			NonceDAMode:           DAModeL1,
			FeeDAMode:             DAModeL1,
			ProofFacts:            felts,
		},
		&DeclareTransaction{
			TransactionHash:       felt.NewFromUint64[felt.Felt](1),
			ClassHash:             felt.NewFromUint64[felt.Felt](2),
			SenderAddress:         felt.NewFromUint64[felt.Felt](3),
			MaxFee:                felt.NewFromUint64[felt.Felt](4),
			TransactionSignature:  felts,
			Nonce:                 felt.NewFromUint64[felt.Felt](5),
			Version:               felt.NewFromUint64[TransactionVersion](3),
			CompiledClassHash:     felt.NewFromUint64[felt.Felt](6),
			ResourceBounds:        resourceBounds,
			Tip:                   7,
			PaymasterData:         felts,
			AccountDeploymentData: felts,
			NonceDAMode:           DAModeL1,
			FeeDAMode:             DAModeL1,
		},
		&L1HandlerTransaction{
			TransactionHash:    felt.NewFromUint64[felt.Felt](1),
			ContractAddress:    felt.NewFromUint64[felt.Felt](2),
			EntryPointSelector: felt.NewFromUint64[felt.Felt](3),
			Nonce:              felt.NewFromUint64[felt.Felt](4),
			CallData:           felts,
			Version:            felt.NewFromUint64[TransactionVersion](3),
		},
	}
}

// TestTransactionHashProjectionNamesEveryTransactionKey fails if the projection's key set differs
// from the union across every transaction type: a missing key would send it to the allocating
// unmatched-key path, and an extra key means a transaction field was renamed or removed.
func TestTransactionHashProjectionNamesEveryTransactionKey(t *testing.T) {
	union := map[string]struct{}{}
	for _, transaction := range sampleTransactions() {
		for key := range cborKeys(reflect.TypeOf(transaction).Elem()) {
			union[key] = struct{}{}
		}
	}
	require.Equal(t, union, cborKeys(reflect.TypeFor[transactionHashProjection]()),
		"transactionHashProjection must name exactly the union of every transaction type's keys")
}

// cborMajorTypeTag is the CBOR major type of a tag, held in the initial byte's top 3 bits.
const cborMajorTypeTag = 6

// sampleTransactionBytes marshals a transaction as production stores it, asserting the record really
// is tag-wrapped. The projection relies on the decoder ignoring that tag, so a record that arrived
// untagged here would silently stop covering the case the production reader hits.
func sampleTransactionBytes(tb testing.TB, transaction Transaction) []byte {
	tb.Helper()
	data, err := encoder.Marshal(transaction)
	require.NoError(tb, err)
	require.EqualValues(tb, cborMajorTypeTag, data[0]>>5,
		"transaction records must be tag-wrapped (needs encoder/registry linked into this binary)")
	return data
}

// TestTransactionHashProjectionCoversEveryWireKey strict-decodes each transaction type's real wire
// bytes, so the decoder flags any unmatched key — catching tag-option drift cborKeys is blind to.
func TestTransactionHashProjectionCoversEveryWireKey(t *testing.T) {
	for _, transaction := range sampleTransactions() {
		t.Run(fmt.Sprintf("%T", transaction), func(t *testing.T) {
			assertProjectionsCoverEveryWireKey(t, sampleTransactionBytes(t, transaction),
				&transactionHashProjection{})
		})
	}
}

// TestTransactionHashProjectionDecodesHash proves the hash field receives the wire value for every
// transaction type, including the one whose hash arrives through an embedded struct. It also pins
// the behaviour the reader leans on: the projection is not a registered type, so the decoder walks
// past the record's tag into its content rather than rejecting it.
func TestTransactionHashProjectionDecodesHash(t *testing.T) {
	for _, transaction := range sampleTransactions() {
		t.Run(fmt.Sprintf("%T", transaction), func(t *testing.T) {
			var projection transactionHashProjection
			require.NoError(t, encoder.Unmarshal(sampleTransactionBytes(t, transaction), &projection))
			require.Equal(t, *transaction.Hash(), projection.TransactionHash,
				"TransactionHash must receive the wire value, not discardedCBOR")
		})
	}
}
