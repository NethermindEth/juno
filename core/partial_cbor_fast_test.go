package core

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	bloom "github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

// projectionCase decodes data into a fresh projection through the wrapper and through fxamacker.
type projectionCase struct {
	name   string
	decode func(data []byte) (got, want any, gotErr, wantErr error)
}

func projectionCaseFor[T any]() projectionCase {
	return projectionCase{
		name: reflect.TypeFor[T]().String(),
		decode: func(data []byte) (got, want any, gotErr, wantErr error) {
			gotValue, wantValue := new(T), new(T)
			gotErr = cbor.Unmarshal(data, gotValue)
			wantErr = fxamacker.Unmarshal(data, wantValue)
			return gotValue, wantValue, gotErr, wantErr
		},
	}
}

func headerProjections() []projectionCase {
	return []projectionCase{
		projectionCaseFor[headerHashProjection](),
		projectionCaseFor[headerGlobalStateRootProjection](),
		projectionCaseFor[headerTransactionCountProjection](),
		projectionCaseFor[headerTimestampProjection](),
		projectionCaseFor[headerEventsBloomProjection](),
		projectionCaseFor[headerHashAndStateRootProjection](),
	}
}

func receiptProjections() []projectionCase {
	return []projectionCase{
		projectionCaseFor[receiptExecutionStatusProjection](),
		projectionCaseFor[receiptEventsProjection](),
	}
}

func transactionProjections() []projectionCase {
	return []projectionCase{projectionCaseFor[transactionHashProjection]()}
}

func routedCount(name string) uint64 { return cbor.Stats()[name].Routed }

// requireMatchesFxamacker decodes data into every projection, through the fast reader and through
// fxamacker, and requires equal values; routed says whether the fast reader must complete it.
func requireMatchesFxamacker(t *testing.T, data []byte, routed bool, projections []projectionCase) {
	t.Helper()
	for _, projection := range projections {
		before := routedCount(projection.name)
		got, want, gotErr, wantErr := projection.decode(data)
		require.NoError(t, wantErr, projection.name)
		require.NoError(t, gotErr, projection.name)
		require.Equal(t, want, got, projection.name)
		if routed {
			require.Equal(t, before+1, routedCount(projection.name), "%s was not routed", projection.name)
		}
	}
}

func marshalled(t testing.TB, v any) []byte {
	t.Helper()
	data, err := cbor.Marshal(v)
	require.NoError(t, err)
	return data
}

// testFelt has limbs of every width, as stored felts do.
func testFelt(v uint64) *felt.Felt {
	return &felt.Felt{v, v << 20, v << 40, v * 0x1_0000_0001}
}

func testFeltSlice(n int) felt.Slice[felt.Felt] {
	out := make(felt.Slice[felt.Felt], n)
	for i := range out {
		out[i] = *testFelt(uint64(i) + 1)
	}
	return out
}

func testVersion(v uint64) *TransactionVersion { return (*TransactionVersion)(testFelt(v)) }

func testBounds() map[Resource]ResourceBounds {
	return map[Resource]ResourceBounds{
		ResourceL1Gas:     {MaxAmount: 100, MaxPricePerUnit: testFelt(3)},
		ResourceL2Gas:     {MaxAmount: 1 << 40, MaxPricePerUnit: testFelt(4)},
		ResourceL1DataGas: {},
	}
}

// wideSampleTransactions has every transaction type with every field set, felts of every limb
// width, and an empty one.
func wideSampleTransactions() []Transaction {
	return []Transaction{
		&InvokeTransaction{
			TransactionHash: testFelt(1), CallData: testFeltSlice(12),
			TransactionSignature: testFeltSlice(2), MaxFee: testFelt(2), ContractAddress: testFelt(3),
			Version: testVersion(3), EntryPointSelector: testFelt(4), Nonce: testFelt(5),
			SenderAddress: testFelt(6), ResourceBounds: testBounds(), Tip: 9,
			PaymasterData: testFeltSlice(1), AccountDeploymentData: testFeltSlice(2),
			NonceDAMode: DAModeL2, FeeDAMode: DAModeL1, ProofFacts: testFeltSlice(2),
		},
		&DeclareTransaction{
			TransactionHash: testFelt(1), ClassHash: testFelt(2), SenderAddress: testFelt(3),
			MaxFee: testFelt(4), TransactionSignature: testFeltSlice(2), Nonce: testFelt(5),
			Version: testVersion(3), CompiledClassHash: testFelt(6), ResourceBounds: testBounds(),
			Tip: 1, PaymasterData: testFeltSlice(1), NonceDAMode: DAModeL1, FeeDAMode: DAModeL2,
		},
		&DeployTransaction{
			TransactionHash: testFelt(1), ContractAddressSalt: testFelt(2),
			ContractAddress: testFelt(3), ClassHash: testFelt(4),
			ConstructorCallData: testFeltSlice(4), Version: testVersion(0),
		},
		&DeployAccountTransaction{
			DeployTransaction: DeployTransaction{
				TransactionHash: testFelt(1), ContractAddressSalt: testFelt(2),
				ContractAddress: testFelt(3), ClassHash: testFelt(4),
				ConstructorCallData: testFeltSlice(4), Version: testVersion(3),
			},
			MaxFee: testFelt(5), TransactionSignature: testFeltSlice(2), Nonce: testFelt(6),
			ResourceBounds: testBounds(), Tip: 2, PaymasterData: testFeltSlice(1),
			NonceDAMode: DAModeL2, FeeDAMode: DAModeL2,
		},
		&L1HandlerTransaction{
			TransactionHash: testFelt(1), ContractAddress: testFelt(2),
			EntryPointSelector: testFelt(3), Nonce: testFelt(4), CallData: testFeltSlice(3),
			Version: testVersion(0),
		},
		&InvokeTransaction{},
	}
}

// transactionForms returns a transaction as stored, wrapped in its type's tag, and without the tag.
func transactionForms(t testing.TB, tx Transaction) (stored, untagged []byte) {
	t.Helper()
	stored = sampleTransactionBytes(t, tx)
	require.Equal(t, byte(0xda), stored[0], "a transaction tag takes four bytes")
	return stored, stored[5:]
}

// The canonical encoding of the key string s and a value, for hand-built maps.
func pair(t testing.TB, key string, value any) []byte {
	t.Helper()
	return append(marshalled(t, key), marshalled(t, value)...)
}

// cborMap builds a map from encoded pairs, in the given order and with any repeats.
func cborMap(pairs ...[]byte) []byte {
	out := []byte{0xb8, byte(len(pairs))}
	for _, p := range pairs {
		out = append(out, p...)
	}
	return out
}

func TestHeaderProjectionReadersMatchFxamacker(t *testing.T) {
	requireMatchesFxamacker(t, sampleHeaderBytes(t), true, headerProjections())
	requireMatchesFxamacker(t, marshalled(t, &Header{}), true, headerProjections())
}

func TestReceiptProjectionReadersMatchFxamacker(t *testing.T) {
	withNullEvent := sampleReceipt()
	withNullEvent.Events = append(withNullEvent.Events, nil, &Event{})
	noEvents := sampleReceipt()
	noEvents.Events = []*Event{}
	for _, receipt := range []*TransactionReceipt{sampleReceipt(), withNullEvent, noEvents, {}} {
		requireMatchesFxamacker(t, marshalled(t, receipt), true, receiptProjections())
	}
}

func TestTransactionHashProjectionReaderMatchesFxamacker(t *testing.T) {
	for _, tx := range wideSampleTransactions() {
		stored, untagged := transactionForms(t, tx)
		requireMatchesFxamacker(t, stored, true, transactionProjections())
		requireMatchesFxamacker(t, untagged, true, transactionProjections())
	}
}

// TestProjectionReadersFollowFxamackerKeys covers keys Juno never writes but fxamacker accepts:
// other case, repeats, unknown and integer keys, and nulls.
func TestProjectionReadersFollowFxamackerKeys(t *testing.T) {
	header := sampleHeader()
	headerCases := map[string][]byte{
		"case":   cborMap(pair(t, "hASH", header.Hash), pair(t, "globalstateroot", header.GlobalStateRoot)),
		"repeat": cborMap(pair(t, "Hash", header.Hash), pair(t, "hash", testFelt(9))),
		"repeated null": cborMap(
			pair(t, "Timestamp", nil), pair(t, "Timestamp", 5),
			pair(t, "TransactionCount", nil), pair(t, "TransactionCount", 7),
		),
		"unknown": cborMap(pair(t, "Unknown", []int{1}), pair(t, "Hash", header.Hash), pair(t, "Hashes", 1)),
		"integer key": cborMap(
			append(marshalled(t, 1), marshalled(t, header.Hash)...), pair(t, "EventsBloom", header.EventsBloom),
		),
		"nulls": cborMap(
			pair(t, "Hash", nil), pair(t, "GlobalStateRoot", nil), pair(t, "Timestamp", nil),
			pair(t, "EventsBloom", nil), pair(t, "TransactionCount", nil),
		),
		"empty": cborMap(),
	}
	for name, data := range headerCases {
		t.Run("header "+name, func(t *testing.T) {
			requireMatchesFxamacker(t, data, true, headerProjections())
		})
	}

	receiptCases := map[string][]byte{
		"case":   cborMap(pair(t, "reverted", true), pair(t, "REVERTREASON", "x"), pair(t, "events", nil)),
		"repeat": cborMap(pair(t, "Reverted", true), pair(t, "Reverted", false), pair(t, "RevertReason", "a")),
		"nulls": cborMap(
			pair(t, "Reverted", nil), pair(t, "RevertReason", nil),
			pair(t, "Events", []any{nil, map[string]any{"from": nil, "KEYS": nil, "Other": 1}}),
			pair(t, "TransactionHash", nil),
		),
	}
	for name, data := range receiptCases {
		t.Run("receipt "+name, func(t *testing.T) {
			requireMatchesFxamacker(t, data, true, receiptProjections())
		})
	}

	txCases := map[string][]byte{
		"case": append([]byte{0xda, 0x00, 0x01, 0x00, 0x00},
			cborMap(pair(t, "transactionhash", testFelt(1)), pair(t, "Nonce", testFelt(2)))...),
		"repeat":     cborMap(pair(t, "TransactionHash", nil), pair(t, "TransactionHash", testFelt(1))),
		"short felt": cborMap(pair(t, "TransactionHash", []uint64{1, 2})),
	}
	for name, data := range txCases {
		t.Run("transaction "+name, func(t *testing.T) {
			requireMatchesFxamacker(t, data, true, transactionProjections())
		})
	}
}

// TestProjectionReadersLeaveTagsToFxamacker checks a tag the readers do not look through goes
// to fxamacker, with the same outcome.
func TestProjectionReadersLeaveTagsToFxamacker(t *testing.T) {
	stored, untagged := transactionForms(t, wideSampleTransactions()[0])
	for _, wrapped := range [][]byte{
		append([]byte{0xc2}, untagged...),           // a bignum tag
		append([]byte{0xd9, 0xd9, 0xf7}, stored...), // a chain of tags
	} {
		for _, projection := range transactionProjections() {
			got, want, gotErr, wantErr := projection.decode(wrapped)
			require.Equal(t, wantErr == nil, gotErr == nil, "%v / %v", gotErr, wantErr)
			require.Equal(t, want, got)
		}
	}
	header := sampleHeader()
	withTag := cborMap(pair(t, "Hash", nil), append(marshalled(t, "GlobalStateRoot"),
		append([]byte{0xd8, 0x30}, marshalled(t, header.GlobalStateRoot)...)...))
	before := cbor.Stats()["core.headerGlobalStateRootProjection"].Unsupported
	requireMatchesFxamacker(t, withTag, false, headerProjections())
	require.Equal(t, before+1, cbor.Stats()["core.headerGlobalStateRootProjection"].Unsupported)
}

// TestProjectionReadersRejectMalformed checks the readers error on broken input, in the fields
// they read and in the ones they skip, without fxamacker's help.
func TestProjectionReadersRejectMalformed(t *testing.T) {
	all := append(append(headerProjections(), receiptProjections()...), transactionProjections()...)
	only := func(names ...string) []projectionCase {
		var out []projectionCase
		for _, projection := range all {
			for _, name := range names {
				if projection.name == "core."+name {
					out = append(out, projection)
				}
			}
		}
		return out
	}
	receipt := marshalled(t, sampleReceipt())
	cases := []struct {
		name        string
		data        []byte
		projections []projectionCase
	}{
		{"truncated header", sampleHeaderBytes(t)[:100], headerProjections()},
		{"truncated receipt", receipt[:50], receiptProjections()},
		{"trailing byte", append(receipt, 0x00), receiptProjections()},
		{"key not UTF-8", cborMap(append([]byte{0x62, 0xff, 0xfe}, marshalled(t, 1)...)), all},
		{"reserved head in a skipped value", cborMap(append(marshalled(t, "Unknown"), 0x1c)), all},
		{"revert reason", cborMap(pair(t, "RevertReason", 1)), only("receiptExecutionStatusProjection")},
		{"events", cborMap(pair(t, "Events", 1)), only("receiptEventsProjection")},
		{"event keys", cborMap(pair(t, "Events", []any{map[string]any{"Keys": "x"}})), only("receiptEventsProjection")},
		{"hash", cborMap(pair(t, "Hash", "x")), only("headerHashProjection", "headerHashAndStateRootProjection")},
		{"transaction count", cborMap(pair(t, "TransactionCount", -1)), only("headerTransactionCountProjection")},
		{"transaction hash", cborMap(pair(t, "TransactionHash", []any{"x"})), only("transactionHashProjection")},
		{"truncated tag", []byte{0xda, 0x00, 0x01}, only("transactionHashProjection")},
	}
	for _, tc := range cases {
		for _, projection := range tc.projections {
			_, _, gotErr, wantErr := projection.decode(tc.data)
			require.Error(t, wantErr, "%s: %s", tc.name, projection.name)
			require.Error(t, gotErr, "%s: %s", tc.name, projection.name)
		}
	}

	// A bloom of 2^64-1 bits in 24 bytes, which is not given to fxamacker: the bloom package would
	// try to allocate it.
	bigBloom := cborMap(append(marshalled(t, "EventsBloom"), 0x58, 0x18, 0, 0, 0, 0, 0, 0, 0, 1,
		0, 0, 0, 0, 0, 0, 0, 1, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff))
	var filter headerEventsBloomProjection
	require.ErrorIs(t, cbor.Unmarshal(bigBloom, &filter), errBloomSize)
}

// TestProjectionReadersCopyTheInput overwrites the input after a decode and checks the
// projections still hold their values.
func TestProjectionReadersCopyTheInput(t *testing.T) {
	data := marshalled(t, sampleReceipt())
	var events receiptEventsProjection
	var status receiptExecutionStatusProjection
	require.NoError(t, cbor.Unmarshal(data, &events))
	require.NoError(t, cbor.Unmarshal(data, &status))
	for i := range data {
		data[i] = 0
	}
	require.Equal(t, sampleReceipt().Events, events.Events)
	require.Equal(t, sampleReceipt().RevertReason, status.RevertReason)

	headerData := sampleHeaderBytes(t)
	var filter headerEventsBloomProjection
	require.NoError(t, cbor.Unmarshal(headerData, &filter))
	for i := range headerData {
		headerData[i] = 0xff
	}
	require.True(t, filter.EventsBloom.Test([]byte("sample-event")))
}

// TestProjectionFieldNamesDifferInMoreThanCase backs the readers' key matching: a key naming a
// skipped field exactly can then never name a wanted one ignoring case.
func TestProjectionFieldNamesDifferInMoreThanCase(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeFor[Header](),
		reflect.TypeFor[TransactionReceipt](),
		reflect.TypeFor[Event](),
		reflect.TypeFor[transactionHashProjection](),
	} {
		folded := map[string]string{}
		for key := range cborKeys(typ) {
			other, ok := folded[strings.ToLower(key)]
			require.False(t, ok, "%s: %s and %s differ only in case", typ, key, other)
			folded[strings.ToLower(key)] = key
		}
	}
}

// fuzzProjections decodes arbitrary bytes into each projection through the wrapper and through
// fxamacker and requires them to agree, as the record readers' fuzz targets do: both succeed with
// equal values, or both fail, except where fxamacker coerces a type the fast reader rejects.
//
// skip names key text that makes an input unsafe to give fxamacker: the bloom package allocates
// whatever bit count the bytes declare.
func fuzzProjections(f *testing.F, projections []projectionCase, skip string, seeds ...[]byte) {
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		unsafeForFxamacker := skip != "" && bytes.Contains(bytes.ToLower(data), []byte(skip))
		for _, projection := range projections {
			if unsafeForFxamacker && projection.name == "core.headerEventsBloomProjection" {
				continue
			}
			got, want, gotErr, wantErr := projection.decode(data)
			if wantErr == nil {
				var typeErr *fxamacker.UnmarshalTypeError
				if errors.As(gotErr, &typeErr) && typeErr.CBORType == "" {
					continue // the fast reader's own type mismatch
				}
				require.NoError(t, gotErr, "%s: fxamacker accepts what the wrapper rejects", projection.name)
				require.Equal(t, want, got, projection.name)
				continue
			}
			require.Error(t, gotErr, "%s: the wrapper accepts what fxamacker rejects", projection.name)
		}
	})
}

func FuzzHeaderProjections(f *testing.F) {
	noBloom := sampleHeader()
	noBloom.EventsBloom = nil
	small := bloom.New(64, 2)
	small.Add([]byte("x"))
	smallBloom := sampleHeader()
	smallBloom.EventsBloom = small
	fuzzProjections(f, headerProjections(), "eventsbloom",
		marshalled(f, noBloom), marshalled(f, smallBloom), marshalled(f, &Header{}))
}

func FuzzReceiptProjections(f *testing.F) {
	fuzzProjections(f, receiptProjections(), "",
		marshalled(f, sampleReceipt()), marshalled(f, &TransactionReceipt{}))
}

func FuzzTransactionHashProjection(f *testing.F) {
	var seeds [][]byte
	for _, tx := range wideSampleTransactions() {
		stored, untagged := transactionForms(f, tx)
		seeds = append(seeds, stored, untagged)
	}
	fuzzProjections(f, transactionProjections(), "", seeds...)
}
