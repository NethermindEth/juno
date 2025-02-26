package core

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

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
	SierraVersion() string
	Hash() (*felt.Felt, error)
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

func (c *Cairo0Class) Hash() (*felt.Felt, error) {
	return cairo0ClassHash(c)
}

func (c *Cairo0Class) SierraVersion() string {
	return "0.1.0"
}

// Cairo1Class unambiguously defines a [Contract]'s semantics.
type Cairo1Class struct {
	Abi     string
	AbiHash *felt.Felt
	// TODO: will implement this on a follow up PR commit to avoid the migration
	// EntryPoints     SierraEntryPointsByType
	EntryPoints struct {
		Constructor []SierraEntryPoint
		External    []SierraEntryPoint
		L1Handler   []SierraEntryPoint
	}
	Program         []*felt.Felt
	ProgramHash     *felt.Felt
	SemanticVersion string
	Compiled        *CompiledClass
}

type SegmentLengths struct {
	Children []SegmentLengths
	Length   uint64
}

type CompiledClass struct {
	Bytecode               []*felt.Felt
	PythonicHints          json.RawMessage
	CompilerVersion        string
	Hints                  json.RawMessage
	Prime                  *big.Int
	External               []CompiledEntryPoint
	L1Handler              []CompiledEntryPoint
	Constructor            []CompiledEntryPoint
	BytecodeSegmentLengths SegmentLengths
}

type CompiledEntryPoint struct {
	Offset   uint64
	Builtins []string
	Selector *felt.Felt
}

// TODO: will implement this on a follow up PR commit to avoid the migration
// type SierraEntryPointsByType struct {
// 	Constructor []SierraEntryPoint
// 	External    []SierraEntryPoint
// 	L1Handler   []SierraEntryPoint
// }

type SierraEntryPoint struct {
	Index    uint64
	Selector *felt.Felt
}

func (c *Cairo1Class) Version() uint64 {
	return 1
}

func (c *Cairo1Class) Hash() (*felt.Felt, error) {
	return crypto.PoseidonArray(
		new(felt.Felt).SetBytes([]byte("CONTRACT_CLASS_V"+c.SemanticVersion)),
		crypto.PoseidonArray(flattenSierraEntryPoints(c.EntryPoints.External)...),
		crypto.PoseidonArray(flattenSierraEntryPoints(c.EntryPoints.L1Handler)...),
		crypto.PoseidonArray(flattenSierraEntryPoints(c.EntryPoints.Constructor)...),
		c.AbiHash,
		c.ProgramHash,
	), nil
}

// Parse Sierra version from the JSON representation of the program.
//
// Sierra programs contain the version number in two possible formats.
// For pre-1.0-rc0 Cairo versions the program contains the Sierra version
// "0.1.0" as a shortstring in its first Felt (0x302e312e30 = "0.1.0").
// For all subsequent versions the version number is the first three felts
// representing the three parts of a semantic version number.
func (c *Cairo1Class) SierraVersion() string {
	// It is defined in this weird way because it is fast :)
	const base = 10
	var buf [32]byte
	b := buf[:0]
	b = strconv.AppendUint(b, c.Program[0].Uint64(), base)
	b = append(b, '.')
	b = strconv.AppendUint(b, c.Program[1].Uint64(), base)
	b = append(b, '.')
	b = strconv.AppendUint(b, c.Program[2].Uint64(), base)
	return string(b)
}

var compiledClassV1Prefix = new(felt.Felt).SetBytes([]byte("COMPILED_CLASS_V1"))

func (c *CompiledClass) Hash() *felt.Felt {
	var bytecodeHash *felt.Felt
	if len(c.BytecodeSegmentLengths.Children) == 0 {
		bytecodeHash = crypto.PoseidonArray(c.Bytecode...)
	} else {
		bytecodeHash = SegmentedBytecodeHash(c.Bytecode, c.BytecodeSegmentLengths.Children)
	}

	return crypto.PoseidonArray(
		compiledClassV1Prefix,
		crypto.PoseidonArray(flattenCompiledEntryPoints(c.External)...),
		crypto.PoseidonArray(flattenCompiledEntryPoints(c.L1Handler)...),
		crypto.PoseidonArray(flattenCompiledEntryPoints(c.Constructor)...),
		bytecodeHash,
	)
}

func SegmentedBytecodeHash(bytecode []*felt.Felt, segmentLengths []SegmentLengths) *felt.Felt {
	var startingOffset uint64
	var digestSegment func(segments []SegmentLengths) (uint64, *felt.Felt)
	digestSegment = func(segments []SegmentLengths) (uint64, *felt.Felt) {
		var totalLength uint64
		var digest crypto.PoseidonDigest
		for _, segment := range segments {
			var curSegmentLength uint64
			var curSegmentHash *felt.Felt

			if len(segment.Children) == 0 {
				curSegmentLength = segment.Length
				segmentBytecode := bytecode[startingOffset : startingOffset+segment.Length]
				curSegmentHash = crypto.PoseidonArray(segmentBytecode...)
			} else {
				curSegmentLength, curSegmentHash = digestSegment(segment.Children)
			}

			digest.Update(new(felt.Felt).SetUint64(curSegmentLength))
			digest.Update(curSegmentHash)

			startingOffset += curSegmentLength
			totalLength += curSegmentLength
		}
		digestRes := digest.Finish()
		return totalLength, digestRes.Add(digestRes, new(felt.Felt).SetUint64(1))
	}

	_, hash := digestSegment(segmentLengths)
	return hash
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

func flattenCompiledEntryPoints(entryPoints []CompiledEntryPoint) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*3)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first, then Offset is second because the order
		// influences the class hash.
		result[3*i] = entryPoint.Selector
		result[3*i+1] = new(felt.Felt).SetUint64(entryPoint.Offset)
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
		if _, ok := class.(*Cairo0Class); ok {
			// skip validation of cairo0 class hash
			continue
		}

		cHash, err := class.Hash()
		if err != nil {
			return err
		}

		if !cHash.Equal(&hash) {
			return fmt.Errorf("cannot verify class hash: calculated hash %v, received hash %v", cHash.String(), hash.String())
		}
	}

	return nil
}
