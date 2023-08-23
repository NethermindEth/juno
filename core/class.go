package core

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

var (
	_ Class = (*Cairo0Class)(nil)
	_ Class = (*Cairo1Class)(nil)
)

// Class unambiguously defines a [Contract]'s semantics.
type Class interface {
	Version() uint64
}

// Cairo0Class unambiguously defines a [Contract]'s semantics.
type Cairo0Class struct {
	Abi json.RawMessage
	// External functions defined in the class.
	Externals []EntryPoint
	// Functions that receive L1 messages. See
	// https://www.cairo-lang.org/docs/hello_starknet/l1l2.html#receiving-a-message-from-l1
	L1Handlers []EntryPoint
	// Constructors for the class. Currently, only one is allowed.
	Constructors []EntryPoint
	// Base64 encoding of compressed Program
	Program string
}

// EntryPoint uniquely identifies a Cairo function to execute.
type EntryPoint struct {
	// starknet_keccak hash of the function signature.
	Selector *felt.Felt
	// The offset of the instruction in the class's bytecode.
	Offset *felt.Felt
}

func (c *Cairo0Class) Version() uint64 {
	return 0
}

// Cairo1Class unambiguously defines a [Contract]'s semantics.
type Cairo1Class struct {
	Abi         string
	AbiHash     *felt.Felt
	EntryPoints struct {
		Constructor []SierraEntryPoint
		External    []SierraEntryPoint
		L1Handler   []SierraEntryPoint
	}
	Program         []*felt.Felt
	ProgramHash     *felt.Felt
	SemanticVersion string
	Compiled        CompiledClass
}

type CompiledClass struct {
	Bytecode        []*felt.Felt
	PythonicHints   json.RawMessage
	CompilerVersion string
	Hints           json.RawMessage
	EntryPoints     CompiledEntryPoints
	Prime           string
}

type CompiledEntryPoints struct {
	External    []CompiledEntryPoint
	L1Handler   []CompiledEntryPoint
	Constructor []CompiledEntryPoint
}

type CompiledEntryPoint struct {
	Offset   *felt.Felt
	Builtins []string
	Selector *felt.Felt
}

type SierraEntryPoint struct {
	Index    uint64
	Selector *felt.Felt
}

func (c *Cairo1Class) Version() uint64 {
	return 1
}

func (c *Cairo1Class) Hash() *felt.Felt {
	return crypto.PoseidonArray(
		new(felt.Felt).SetBytes([]byte("CONTRACT_CLASS_V"+c.SemanticVersion)),
		crypto.PoseidonArray(flattenSierraEntryPoints(c.EntryPoints.External)...),
		crypto.PoseidonArray(flattenSierraEntryPoints(c.EntryPoints.L1Handler)...),
		crypto.PoseidonArray(flattenSierraEntryPoints(c.EntryPoints.Constructor)...),
		c.AbiHash,
		c.ProgramHash,
	)
}

func (c *CompiledClass) CompiledClassHash() *felt.Felt {
	return crypto.PoseidonArray(
		new(felt.Felt).SetBytes([]byte("COMPILED_CLASS_V1")),
		crypto.PoseidonArray(compiledEntryPoints(c.EntryPoints.External)...),
		crypto.PoseidonArray(compiledEntryPoints(c.EntryPoints.L1Handler)...),
		crypto.PoseidonArray(compiledEntryPoints(c.EntryPoints.Constructor)...),
		crypto.PoseidonArray(c.Bytecode...),
	)
}

func flattenSierraEntryPoints(entryPoints []SierraEntryPoint) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*2)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first because the order
		// influences the class hash.
		result[2*i] = entryPoint.Selector
		result[2*i+1] = new(felt.Felt).SetUint64(entryPoint.Index)
	}
	return result
}

func compiledEntryPoints(entryPoints []CompiledEntryPoint) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*3)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first, then Offset is second because the order
		// influences the class hash.
		result[3*i] = entryPoint.Selector
		result[3*i+1] = entryPoint.Offset
		builtins := make([]*felt.Felt, len(entryPoint.Builtins))
		for idx, buil := range entryPoint.Builtins {
			builtins[idx] = new(felt.Felt).SetBytes([]byte(buil))
		}
		result[3*i+2] = crypto.PoseidonArray(builtins...)
	}

	return result
}

func VerifyClassHashes(classes map[felt.Felt]Class) error {
	for hash, class := range classes {
		cairo1Class, ok := class.(*Cairo1Class)
		// cairo0 classes are deprecated and hard to verify their hash, just ignore them
		if !ok {
			return nil
		}

		cHash := cairo1Class.Hash()
		if !cHash.Equal(&hash) {
			return fmt.Errorf("cannot verify class hash: calculated hash %v, received hash %v", cHash.String(), hash.String())
		}
	}
	return nil
}
