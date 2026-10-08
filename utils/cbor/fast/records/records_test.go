package records_test

import (
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/stretchr/testify/require"
)

func f(v uint64) *felt.Felt {
	// Stored felts are Montgomery limbs; vary the limb widths on purpose.
	return &felt.Felt{v, v << 20, v << 40, v * 0x1_0000_0001}
}

func feltSlice(n int) felt.Slice[felt.Felt] {
	out := make(felt.Slice[felt.Felt], n)
	for i := range out {
		out[i] = *f(uint64(i) + 1)
	}
	return out
}

func receipt() *core.TransactionReceipt {
	return &core.TransactionReceipt{
		Fee:     f(7),
		FeeUnit: core.STRK,
		Events: []*core.Event{
			{From: f(1), Keys: feltSlice(2), Data: feltSlice(5)},
			{From: f(2)},
			nil,
		},
		ExecutionResources: &core.ExecutionResources{
			BuiltinInstanceCounter: core.BuiltinInstanceCounter{Pedersen: 1, RangeCheck: 70000, Ecsda: 3, RangeCheck96: 1 << 40},
			MemoryHoles:            9,
			Steps:                  1234567,
			DataAvailability:       &core.DataAvailability{L1Gas: 1, L1DataGas: 2},
			TotalGasConsumed:       &core.GasConsumed{L1Gas: 1, L1DataGas: 2, L2Gas: 1 << 33},
		},
		L1ToL2Message: &core.L1ToL2Message{
			From:     eth.Address{1, 2, 3, 19: 9},
			Nonce:    f(5),
			Payload:  feltSlice(3),
			Selector: f(6),
			To:       f(7),
		},
		L2ToL1Message:   []*core.L2ToL1Message{{From: f(1), Payload: feltSlice(1), To: eth.Address{7}}},
		TransactionHash: f(99),
		Reverted:        true,
		RevertReason:    "out of gas: héllo",
	}
}

func sierraClass() *core.SierraClass {
	return &core.SierraClass{
		Abi:     `[{"type":"function"}]`,
		AbiHash: f(3),
		EntryPoints: core.SierraEntryPointsByType{
			Constructor: []core.SierraEntryPoint{{Index: 0, Selector: f(1)}},
			External:    []core.SierraEntryPoint{{Index: 1, Selector: f(2)}, {Index: 300, Selector: f(3)}},
		},
		Program:         feltSlice(1000),
		ProgramHash:     f(4),
		SemanticVersion: "0.1.0",
		Compiled: &core.CasmClass{
			Bytecode:        feltSlice(40),
			PythonicHints:   []byte(`[[0,["a"]]]`),
			CompilerVersion: "2.9.0",
			Hints:           []byte(`[]`),
			Prime:           new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 251), big.NewInt(1)),
			External:        []core.CasmEntryPoint{{Offset: 5, Builtins: []string{"range_check", "pedersen"}, Selector: f(8)}},
			Constructor:     []core.CasmEntryPoint{{Offset: 0}},
			BytecodeSegmentLengths: core.SegmentLengths{
				Length:   40,
				Children: []core.SegmentLengths{{Length: 10}, {Length: 30, Children: []core.SegmentLengths{{Length: 30}}}},
			},
		},
	}
}

// decodeBoth decodes data into fresh values of the same type through the wrapper, which
// takes the fast route, and through fxamacker directly.
func decodeBoth[T any](t *testing.T, data []byte) (viaWrapper, viaFx *T) {
	t.Helper()
	viaWrapper, viaFx = new(T), new(T)
	require.NoError(t, cbor.Unmarshal(data, viaWrapper))
	require.NoError(t, fxamacker.Unmarshal(data, viaFx))
	return viaWrapper, viaFx
}

func routed(name string) uint64 { return cbor.Stats()[name].Routed }

func TestReceiptMatchesFxamacker(t *testing.T) {
	for name, value := range map[string]*core.TransactionReceipt{
		"populated": receipt(),
		"zero":      {},
	} {
		t.Run(name, func(t *testing.T) {
			data, err := cbor.Marshal(value)
			require.NoError(t, err)

			before := routed("core.TransactionReceipt")
			got, want := decodeBoth[core.TransactionReceipt](t, data)
			require.Equal(t, want, got)
			require.Equal(t, before+1, routed("core.TransactionReceipt"), "must be decoded by the fast reader")
		})
	}
}

func TestSierraClassMatchesFxamacker(t *testing.T) {
	var class core.ClassDefinition = sierraClass()
	data, err := cbor.Marshal(&class)
	require.NoError(t, err)

	before := routed("core.ClassDefinition")
	var got, want core.ClassDefinition
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.NoError(t, fxamacker.Unmarshal(data, &want))
	require.Equal(t, want, got)
	require.Equal(t, before+1, routed("core.ClassDefinition"))
}

func TestCairo0ClassFallsBack(t *testing.T) {
	var class core.ClassDefinition = &core.DeprecatedCairoClass{Abi: []byte(`[]`), Program: "p"}
	data, err := cbor.Marshal(&class)
	require.NoError(t, err)

	stats := cbor.Stats()["core.ClassDefinition"]
	var got, want core.ClassDefinition
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.NoError(t, fxamacker.Unmarshal(data, &want))
	require.Equal(t, want, got)
	require.Equal(t, stats.Unsupported+1, cbor.Stats()["core.ClassDefinition"].Unsupported)
}

func TestNonZeroDestinationGoesToFxamacker(t *testing.T) {
	data, err := cbor.Marshal(receipt())
	require.NoError(t, err)

	stats := cbor.Stats()["core.TransactionReceipt"]
	dst := core.TransactionReceipt{Reverted: true, RevertReason: "stale"}
	require.NoError(t, cbor.Unmarshal(data, &dst))
	require.Equal(t, stats.NonZeroTarget+1, cbor.Stats()["core.TransactionReceipt"].NonZeroTarget)
	require.Equal(t, stats.Routed, cbor.Stats()["core.TransactionReceipt"].Routed)
}

func TestPointerDestination(t *testing.T) {
	data, err := cbor.Marshal(receipt())
	require.NoError(t, err)

	var got, want *core.TransactionReceipt
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.NoError(t, fxamacker.Unmarshal(data, &want))
	require.Equal(t, want, got)

	got = nil
	require.NoError(t, cbor.Unmarshal([]byte{0xf6}, &got))
	require.Nil(t, got)
}

func TestUnmarshalFirstRemainder(t *testing.T) {
	data, err := cbor.Marshal(receipt())
	require.NoError(t, err)

	trailer := []byte{0x01, 0x02, 0x03}
	var got core.TransactionReceipt
	rest, err := cbor.UnmarshalFirst(append(append([]byte{}, data...), trailer...), &got)
	require.NoError(t, err)
	require.Equal(t, trailer, rest)

	require.Error(t, cbor.Unmarshal(append(append([]byte{}, data...), trailer...), new(core.TransactionReceipt)))
}

func TestDecodedValuesDoNotAliasInput(t *testing.T) {
	data, err := cbor.Marshal(receipt())
	require.NoError(t, err)
	want := new(core.TransactionReceipt)
	require.NoError(t, fxamacker.Unmarshal(data, want))

	got := new(core.TransactionReceipt)
	require.NoError(t, cbor.Unmarshal(data, got))
	clear(data)
	require.Equal(t, want, got)

	classData, err := cbor.Marshal(func() *core.ClassDefinition { c := core.ClassDefinition(sierraClass()); return &c }())
	require.NoError(t, err)
	var wantClass, gotClass core.ClassDefinition
	require.NoError(t, fxamacker.Unmarshal(classData, &wantClass))
	require.NoError(t, cbor.Unmarshal(classData, &gotClass))
	clear(classData)
	require.Equal(t, wantClass, gotClass)
}

func TestMalformedInputIsAnErrorNotAFallback(t *testing.T) {
	data, err := cbor.Marshal(receipt())
	require.NoError(t, err)

	for _, cut := range []int{1, 2, 10, len(data) / 2, len(data) - 1} {
		stats := cbor.Stats()["core.TransactionReceipt"]
		require.Error(t, cbor.Unmarshal(data[:cut], new(core.TransactionReceipt)))
		after := cbor.Stats()["core.TransactionReceipt"]
		require.Equal(t, stats.Unsupported, after.Unsupported)
	}
}

func TestIndefiniteLengthFallsBackToFxamacker(t *testing.T) {
	// {"Fee": null, "Events": [_ ]} with an indefinite-length array: valid CBOR.
	data := []byte{0xa1, 0x66, 'E', 'v', 'e', 'n', 't', 's', 0x9f, 0xff}
	stats := cbor.Stats()["core.TransactionReceipt"]

	var got, want core.TransactionReceipt
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.NoError(t, fxamacker.Unmarshal(data, &want))
	require.Equal(t, want, got)
	require.Equal(t, stats.Unsupported+1, cbor.Stats()["core.TransactionReceipt"].Unsupported)
}
