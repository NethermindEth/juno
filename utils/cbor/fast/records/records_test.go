package records_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fast/records"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	bloom "github.com/bits-and-blooms/bloom/v3"
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

func TestReceiptMatchesFxamacker(t *testing.T) {
	for name, value := range map[string]*core.TransactionReceipt{
		"populated": receipt(),
		"zero":      {},
	} {
		t.Run(name, func(t *testing.T) {
			data, err := cbor.Marshal(value)
			require.NoError(t, err)

			got, want := decodeBoth[core.TransactionReceipt](t, data)
			require.Equal(t, want, got)
		})
	}
}

func TestSierraClassMatchesFxamacker(t *testing.T) {
	var class core.ClassDefinition = sierraClass()
	data, err := cbor.Marshal(&class)
	require.NoError(t, err)

	var got, want core.ClassDefinition
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.NoError(t, fxamacker.Unmarshal(data, &want))
	require.Equal(t, want, got)
}

func TestCairo0ClassMatchesFxamacker(t *testing.T) {
	var class core.ClassDefinition = &core.DeprecatedCairoClass{Abi: []byte(`[]`), Program: "p"}
	data, err := cbor.Marshal(&class)
	require.NoError(t, err)

	var got, want core.ClassDefinition
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.NoError(t, fxamacker.Unmarshal(data, &want))
	require.Equal(t, want, got)
}

func TestNonZeroDestinationMatchesFxamacker(t *testing.T) {
	data, err := cbor.Marshal(receipt())
	require.NoError(t, err)

	got := core.TransactionReceipt{Reverted: true, RevertReason: "stale"}
	want := got
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.NoError(t, fxamacker.Unmarshal(data, &want))
	require.Equal(t, want, got)
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

func TestMalformedInputIsAnError(t *testing.T) {
	data, err := cbor.Marshal(receipt())
	require.NoError(t, err)

	for _, cut := range []int{1, 2, 10, len(data) / 2, len(data) - 1} {
		require.Error(t, cbor.Unmarshal(data[:cut], new(core.TransactionReceipt)))
	}
}

func version(v uint64) *core.TransactionVersion { return (*core.TransactionVersion)(f(v)) }

func bounds() map[core.Resource]core.ResourceBounds {
	return map[core.Resource]core.ResourceBounds{
		core.ResourceL1Gas:     {MaxAmount: 100, MaxPricePerUnit: f(3)},
		core.ResourceL2Gas:     {MaxAmount: 1 << 40, MaxPricePerUnit: f(4)},
		core.ResourceL1DataGas: {},
	}
}

func transactions() map[string]core.Transaction {
	return map[string]core.Transaction{
		"invoke v3": &core.InvokeTransaction{
			TransactionHash: f(1), CallData: feltSlice(12), TransactionSignature: feltSlice(2),
			MaxFee: f(2), ContractAddress: f(3), Version: version(3), EntryPointSelector: f(4),
			Nonce: f(5), SenderAddress: f(6), ResourceBounds: bounds(), Tip: 9,
			PaymasterData: feltSlice(1), AccountDeploymentData: feltSlice(2),
			NonceDAMode: core.DAModeL2, FeeDAMode: core.DAModeL1, ProofFacts: feltSlice(2),
		},
		"invoke v1": &core.InvokeTransaction{
			TransactionHash: f(1), CallData: feltSlice(3), TransactionSignature: feltSlice(2),
			MaxFee: f(2), Version: version(1), Nonce: f(5), SenderAddress: f(6),
		},
		"declare": &core.DeclareTransaction{
			TransactionHash: f(1), ClassHash: f(2), SenderAddress: f(3), MaxFee: f(4),
			TransactionSignature: feltSlice(2), Nonce: f(5), Version: version(3),
			CompiledClassHash: f(6), ResourceBounds: bounds(), Tip: 1,
			PaymasterData: feltSlice(1), NonceDAMode: core.DAModeL1, FeeDAMode: core.DAModeL2,
		},
		"deploy": &core.DeployTransaction{
			TransactionHash: f(1), ContractAddressSalt: f(2), ContractAddress: f(3), ClassHash: f(4),
			ConstructorCallData: feltSlice(4), Version: version(0),
		},
		"deploy account": &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash: f(1), ContractAddressSalt: f(2), ContractAddress: f(3), ClassHash: f(4),
				ConstructorCallData: feltSlice(4), Version: version(3),
			},
			MaxFee: f(5), TransactionSignature: feltSlice(2), Nonce: f(6),
			ResourceBounds: bounds(), Tip: 2, PaymasterData: feltSlice(1),
			NonceDAMode: core.DAModeL2, FeeDAMode: core.DAModeL2,
		},
		"l1 handler": &core.L1HandlerTransaction{
			TransactionHash: f(1), ContractAddress: f(2), EntryPointSelector: f(3), Nonce: f(4),
			CallData: feltSlice(3), Version: version(0),
		},
		"empty invoke": &core.InvokeTransaction{},
	}
}

func TestTransactionMatchesFxamacker(t *testing.T) {
	for name, tx := range transactions() {
		t.Run(name, func(t *testing.T) {
			data, err := cbor.Marshal(&tx)
			require.NoError(t, err)

			var got, want core.Transaction
			require.NoError(t, cbor.Unmarshal(data, &got))
			require.NoError(t, fxamacker.Unmarshal(data, &want))
			require.Equal(t, want, got)
		})
	}
}

func header() *core.Header {
	filter := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	filter.Add([]byte("event"))
	return &core.Header{
		Hash: f(1), ParentHash: f(2), Number: 12345, GlobalStateRoot: f(3), SequencerAddress: f(4),
		TransactionCount: 70, EventCount: 300, Timestamp: 1_700_000_000, ProtocolVersion: "0.14.1",
		EventsBloom: filter, L1GasPriceETH: f(5), L1GasPriceSTRK: f(6),
		Signatures: [][]*felt.Felt{{f(1), f(2)}, {}, nil, {nil}},
		L1DAMode:   core.Blob, L1DataGasPrice: &core.GasPrice{PriceInWei: f(7), PriceInFri: f(8)},
		L2GasPrice: &core.GasPrice{PriceInWei: f(9)},
	}
}

func TestHeaderMatchesFxamacker(t *testing.T) {
	for name, value := range map[string]*core.Header{"populated": header(), "zero": {}} {
		t.Run(name, func(t *testing.T) {
			data, err := cbor.Marshal(value)
			require.NoError(t, err)

			got, want := decodeBoth[core.Header](t, data)
			require.Equal(t, want, got)
		})
	}
}

// A header written before v0.13 names its gas prices GasPrice and GasPriceSTRK, which fxamacker
// matches to the fields gasprice and gaspricestrk ignoring case. ExtraData has no field.
func TestHeaderLegacyKeys(t *testing.T) {
	zero := []byte{0x84, 0, 0, 0, 0}
	data := []byte{0xa4}
	for _, entry := range []struct {
		key   string
		value []byte
	}{
		{"Number", []byte{0x05}},
		{"GasPrice", append([]byte{}, zero...)},
		{"GasPriceSTRK", []byte{0x84, 0, 0, 0, 0x07}},
		{"ExtraData", []byte{0x40}},
	} {
		data = append(data, byte(0x60+len(entry.key)))
		data = append(data, entry.key...)
		data = append(data, entry.value...)
	}

	got, want := decodeBoth[core.Header](t, data)
	require.Equal(t, want, got)
	require.EqualValues(t, 5, got.Number)
	require.NotNil(t, got.L1GasPriceETH)
	require.NotNil(t, got.L1GasPriceSTRK)
}

func stateUpdate() *core.StateUpdate {
	return &core.StateUpdate{
		BlockHash: f(1), NewRoot: f(2), OldRoot: f(3),
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*f(1): {*f(10): f(11), *f(12): f(13)},
				*f(2): {},
				*f(3): nil,
			},
			Nonces:            map[felt.Felt]*felt.Felt{*f(1): f(2), *f(3): f(4)},
			DeployedContracts: map[felt.Felt]*felt.Felt{*f(5): f(6)},
			DeclaredV0Classes: []*felt.Felt{f(7), f(8)},
			DeclaredV1Classes: map[felt.Felt]*felt.Felt{*f(9): f(10)},
			ReplacedClasses:   map[felt.Felt]*felt.Felt{*f(11): f(12)},
			MigratedClasses:   map[felt.SierraClassHash]felt.CasmClassHash{felt.SierraClassHash(*f(1)): felt.CasmClassHash(*f(2))},
		},
	}
}

func TestStateUpdateMatchesFxamacker(t *testing.T) {
	update := stateUpdate()
	for name, value := range map[string]*core.StateUpdate{"populated": update, "zero": {}, "empty diff": {StateDiff: &core.StateDiff{}}} {
		t.Run(name, func(t *testing.T) {
			data, err := cbor.Marshal(value)
			require.NoError(t, err)

			got, want := decodeBoth[core.StateUpdate](t, data)
			require.Equal(t, want, got)
		})
	}
}

func TestBloomDeclaringMoreBitsThanItHoldsIsRejected(t *testing.T) {
	data, err := cbor.Marshal(header())
	require.NoError(t, err)

	// The bit count sits after m and k, in the bloom filter's byte string, which starts
	// after the key "EventsBloom" and the byte string's own head.
	key := []byte("EventsBloom")
	at := bytes.Index(data, key) + len(key)
	require.Equal(t, byte(0x59), data[at], "a two byte length byte string")
	count := at + 3 + 2*8
	for i := range 8 {
		data[count+i] = 0
	}
	data[count] = 0x04 // 2^58 bits

	require.ErrorIs(t, cbor.Unmarshal(data, new(core.Header)), records.ErrBloomSize)
}

// allocsOf counts allocations per decode of data into a fresh T with decode.
func allocsOf[T any](data []byte, decode func([]byte, any) error) float64 {
	return testing.AllocsPerRun(50, func() {
		var dst T
		if err := decode(data, &dst); err != nil {
			panic(err)
		}
	})
}

// Elements of a pointer slice share an array only while the bytes left could hold them; small
// ones past that are allocated one by one.
func TestSmallEventsMatchFxamacker(t *testing.T) {
	// {"Events": [{}, null, {"From": null}, {}]}
	data := []byte{0xa1, 0x66, 'E', 'v', 'e', 'n', 't', 's', 0x84, 0xa0, 0xf6, 0xa1, 0x64, 'F', 'r', 'o', 'm', 0xf6, 0xa0}
	got, want := decodeBoth[core.TransactionReceipt](t, data)
	require.Equal(t, want, got)
	require.Len(t, got.Events, 4)
	require.Nil(t, got.Events[1])
	require.NotSame(t, got.Events[0], got.Events[2])
}

// TestSharedArraysKeepValuesIndependent checks that values read into one shared array are
// still distinct: writing through one decoded pointer changes nothing else, and null elements
// stay nil.
func TestSharedArraysKeepValuesIndependent(t *testing.T) {
	update := stateUpdate()
	update.StateDiff.Nonces[*f(20)] = nil
	update.StateDiff.DeclaredV0Classes = append(update.StateDiff.DeclaredV0Classes, nil, f(21))
	update.StateDiff.StorageDiffs[*f(4)] = map[felt.Felt]*felt.Felt{*f(14): f(15), *f(16): nil, *f(17): f(18)}
	data, err := cbor.Marshal(update)
	require.NoError(t, err)
	got, want := decodeBoth[core.StateUpdate](t, data)
	require.Equal(t, want, got)

	receiptData, err := cbor.Marshal(receipt())
	require.NoError(t, err)
	gotReceipt, wantReceipt := decodeBoth[core.TransactionReceipt](t, receiptData)
	require.Equal(t, wantReceipt, gotReceipt)
	// An event written through its pointer leaves its neighbours alone.
	*gotReceipt.Events[0] = core.Event{}
	require.Equal(t, wantReceipt.Events[1:], gotReceipt.Events[1:])

	diff := got.StateDiff
	var felts []*felt.Felt
	for _, storage := range diff.StorageDiffs {
		for _, value := range storage {
			felts = append(felts, value)
		}
	}
	for _, m := range []map[felt.Felt]*felt.Felt{diff.Nonces, diff.DeployedContracts, diff.DeclaredV1Classes, diff.ReplacedClasses} {
		for _, value := range m {
			felts = append(felts, value)
		}
	}
	felts = append(felts, diff.DeclaredV0Classes...)
	for _, event := range gotReceipt.Events {
		if event != nil {
			felts = append(felts, event.From)
		}
	}
	for i, value := range felts {
		if value == nil {
			continue
		}
		*value = felt.Felt{1 << 63, uint64(i)}
	}
	for i, value := range felts {
		if value != nil {
			require.Equal(t, felt.Felt{1 << 63, uint64(i)}, *value, "felt %d was overwritten through another pointer", i)
		}
	}
}
