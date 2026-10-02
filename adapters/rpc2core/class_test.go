package rpc2core_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/adapters/rpc2core"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedClass is what a client recovers from a served class. The wire format carries no
// pythonic hints, the server compacts the hints, and a Cairo 0 ABI crosses the wire as
// generic JSON, which re-encodes it.
func expectedClass(t *testing.T, class core.ClassDefinition) core.ClassDefinition {
	t.Helper()
	switch v := class.(type) {
	case *core.SierraClass:
		expected := *v
		compiled := *v.Compiled
		compiled.PythonicHints = nil
		var hints bytes.Buffer
		require.NoError(t, json.Compact(&hints, v.Compiled.Hints))
		compiled.Hints = hints.Bytes()
		expected.Compiled = &compiled
		return &expected
	case *core.DeprecatedCairoClass:
		expected := *v
		var abi any
		require.NoError(t, json.Unmarshal(v.Abi, &abi))
		var err error
		expected.Abi, err = json.Marshal(abi)
		require.NoError(t, err)
		return &expected
	default:
		t.Fatalf("unexpected class type %T", class)
		return nil
	}
}

func TestAdaptClass(t *testing.T) {
	sierraClassHash := felt.NewUnsafeFromString[felt.Felt](
		"0x3cc90db763e736ca9b6c581ea4008408842b1a125947ab087438676a7e40b7b",
	)
	deprecatedClassHash := felt.NewUnsafeFromString[felt.Felt](
		"0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6",
	)
	gateway := feederGateway(t, &networks.Sepolia)
	classes := make(map[felt.Felt]core.ClassDefinition)
	for _, classHash := range []*felt.Felt{sierraClassHash, deprecatedClassHash} {
		class, err := gateway.Class(t.Context(), classHash)
		require.NoError(t, err)
		classes[*classHash] = class
	}
	handler, _ := storeBlocks(t, &networks.Sepolia, 1, classes)
	latest := rpcv10.BlockIDLatest()

	tests := []struct {
		name      string
		classHash *felt.Felt
		compiled  bool
	}{
		{"sierra", sierraClassHash, true},
		{"deprecated cairo", deprecatedClassHash, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			class, rpcErr := handler.Class(&latest, test.classHash)
			require.Nil(t, rpcErr)
			served := overWire(t, class)
			require.Equal(t, test.compiled, rpc2core.IsSierraClass(served))

			var casm *rpcv10.CompiledCasmResponse
			if test.compiled {
				compiled, rpcErr := handler.CompiledCasm(test.classHash)
				require.Nil(t, rpcErr)
				casm = overWire(t, &compiled)
			}

			adapted, err := rpc2core.AdaptClass(served, casm)
			require.NoError(t, err)
			assert.Equal(t, expectedClass(t, classes[*test.classHash]), adapted)

			hash, err := adapted.Hash()
			require.NoError(t, err)
			assert.Equal(t, *test.classHash, hash)
		})
	}
}

func sierraClass(entryPoints rpcv10.ClassEntryPointsByType) *rpcv10.Class {
	return &rpcv10.Class{
		SierraProgram:        felt.Slice[felt.Felt]{felt.One, felt.One, felt.One},
		ContractClassVersion: "0.1.0",
		EntryPoints:          entryPoints,
		Abi:                  "[]",
	}
}

func casmClass(bytecodeLength int, segmentLengths []int) *rpcv10.CompiledCasmResponse {
	return &rpcv10.CompiledCasmResponse{
		Prime:                  "0x800000000000011000000000000000000000000000000000000000000000001",
		CompilerVersion:        "2.6.0",
		Bytecode:               make(felt.Slice[felt.Felt], bytecodeLength),
		Hints:                  json.RawMessage("[]"),
		BytecodeSegmentLengths: segmentLengths,
	}
}

func TestAdaptClassSegmentLengths(t *testing.T) {
	leaf := func(length uint64) core.SegmentLengths {
		return core.SegmentLengths{Length: length}
	}

	tests := []struct {
		name           string
		bytecodeLength int
		segmentLengths []int
		expected       core.SegmentLengths
	}{
		{"absent", 3, nil, leaf(0)},
		{"empty", 3, []int{}, leaf(0)},
		{"zero next to bytecode", 3, []int{0}, leaf(0)},
		{"zero next to empty bytecode", 0, []int{0}, leaf(0)},
		{"whole bytecode", 3, []int{3}, leaf(3)},
		{
			"leaves", 3,
			[]int{1, 2},
			core.SegmentLengths{Children: []core.SegmentLengths{leaf(1), leaf(2)}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapted, err := rpc2core.AdaptClass(
				sierraClass(rpcv10.ClassEntryPointsByType{}),
				casmClass(test.bytecodeLength, test.segmentLengths),
			)
			require.NoError(t, err)
			assert.Equal(t, test.expected, adapted.(*core.SierraClass).Compiled.BytecodeSegmentLengths)
		})
	}
}

func TestAdaptClassErrors(t *testing.T) {
	index := uint64(0)
	program := "program"

	tests := []struct {
		name  string
		class *rpcv10.Class
		casm  *rpcv10.CompiledCasmResponse
		err   string
	}{
		{
			name: "nil class",
			err:  "nil class",
		},
		{
			name: "sierra abi is not a string",
			class: &rpcv10.Class{
				SierraProgram: felt.Slice[felt.Felt]{felt.One, felt.One, felt.One},
				Abi:           42,
			},
			err: "sierra class abi is int, want string",
		},
		{
			name:  "sierra program empty",
			class: &rpcv10.Class{},
			err:   "sierra program size is too small",
		},
		{
			name:  "sierra program shorter than its version",
			class: &rpcv10.Class{SierraProgram: felt.Slice[felt.Felt]{felt.One, felt.One}},
			err:   "sierra program size is too small",
		},
		{
			name: "sierra entry point missing function_idx",
			class: sierraClass(rpcv10.ClassEntryPointsByType{
				External: []rpcv10.ClassEntryPoint{{Selector: &felt.One}},
			}),
			err: "EXTERNAL entry point 0 is missing function_idx",
		},
		{
			name: "sierra entry point missing selector",
			class: sierraClass(rpcv10.ClassEntryPointsByType{
				L1Handler: []rpcv10.ClassEntryPoint{{Index: &index}},
			}),
			err: "L1_HANDLER entry point 0 is missing selector",
		},
		{
			name:  "sierra missing compiled class",
			class: sierraClass(rpcv10.ClassEntryPointsByType{}),
			err:   "sierra class is missing its compiled class",
		},
		{
			name:  "casm prime is not a number",
			class: sierraClass(rpcv10.ClassEntryPointsByType{}),
			casm:  &rpcv10.CompiledCasmResponse{Prime: "prime"},
			err:   `compiled class prime "prime" is not a number`,
		},
		{
			name:  "casm segment negative",
			class: sierraClass(rpcv10.ClassEntryPointsByType{}),
			casm:  casmClass(3, []int{-1, 4}),
			err:   "bytecode segment 0 has negative length -1",
		},
		{
			name:  "casm segment beyond bytecode",
			class: sierraClass(rpcv10.ClassEntryPointsByType{}),
			casm:  casmClass(3, []int{1, 5}),
			err:   "bytecode segment 1 has length 5 but only 2 bytecode felts remain",
		},
		{
			name:  "casm segments shorter than bytecode",
			class: sierraClass(rpcv10.ClassEntryPointsByType{}),
			casm:  casmClass(3, []int{1, 1}),
			err:   "bytecode segment lengths sum to 2 felts but the bytecode has 3",
		},
		{
			name:  "deprecated abi cannot be encoded",
			class: &rpcv10.Class{Program: program, Abi: make(chan int)},
			err:   "encoding legacy class abi",
		},
		{
			name: "deprecated entry point missing selector",
			class: &rpcv10.Class{Program: program, EntryPoints: rpcv10.ClassEntryPointsByType{
				Constructor: []rpcv10.ClassEntryPoint{{Offset: &felt.One}},
			}},
			err: "CONSTRUCTOR entry point 0 is missing selector",
		},
		{
			name: "deprecated entry point missing offset",
			class: &rpcv10.Class{Program: program, EntryPoints: rpcv10.ClassEntryPointsByType{
				External: []rpcv10.ClassEntryPoint{{Selector: &felt.One}},
			}},
			err: "EXTERNAL entry point 0 is missing offset",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapted, err := rpc2core.AdaptClass(test.class, test.casm)
			require.ErrorContains(t, err, test.err)
			assert.Nil(t, adapted)
		})
	}
}
