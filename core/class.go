package core

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
)

var (
	_ ClassDefinition = (*DeprecatedCairoClass)(nil)
	_ ClassDefinition = (*SierraClass)(nil)
)

const minDeclaredClassSize = 8

// Single felt identifying the number "0.1.0" as a short string
var SierraVersion010 felt.Felt = felt.Felt(
	[4]uint64{
		18446737451840584193,
		18446744073709551615,
		18446744073709551615,
		576348180530977296,
	})

type ClassDefinition interface {
	SierraVersion() string
	Hash() (felt.Felt, error)
}

type DeprecatedCairoClass struct {
	Abi json.RawMessage
	// External functions defined in the class.
	Externals []DeprecatedEntryPoint
	// Functions that receive L1 messages. See
	// https://www.cairo-lang.org/docs/hello_starknet/l1l2.html#receiving-a-message-from-l1
	L1Handlers []DeprecatedEntryPoint
	// Constructors for the class. Currently, only one is allowed.
	Constructors []DeprecatedEntryPoint
	// Base64 encoding of compressed Program
	Program string
}

type DeprecatedEntryPoint struct {
	// starknet_keccak hash of the function signature.
	Selector *felt.Felt
	// The offset of the instruction in the class's bytecode.
	Offset *felt.Felt
}

func (c *DeprecatedCairoClass) Version() uint64 {
	return 0
}

func (c *DeprecatedCairoClass) Hash() (felt.Felt, error) {
	return deprecatedCairoClassHash(c)
}

func (c *DeprecatedCairoClass) SierraVersion() string {
	return "0.0.0"
}

type SierraClass struct {
	Abi         string
	AbiHash     *felt.Felt
	EntryPoints SierraEntryPointsByType
	Program     []*felt.Felt
	ProgramHash *felt.Felt
	// TODO: Remove this semantic version on a follow up PR. Let's put Sierra version instead
	SemanticVersion string
	Compiled        *CasmClass
}

type SegmentLengths struct {
	Children []SegmentLengths
	Length   uint64
}

type CasmClass struct {
	Bytecode               []*felt.Felt
	PythonicHints          json.RawMessage
	CompilerVersion        string
	Hints                  json.RawMessage
	Prime                  *big.Int
	External               []CasmEntryPoint
	L1Handler              []CasmEntryPoint
	Constructor            []CasmEntryPoint
	BytecodeSegmentLengths SegmentLengths
}

type CasmEntryPoint struct {
	Offset   uint64
	Builtins []string
	Selector *felt.Felt
}

type SierraEntryPointsByType struct {
	Constructor []SierraEntryPoint
	External    []SierraEntryPoint
	L1Handler   []SierraEntryPoint
}

type SierraEntryPoint struct {
	Index    uint64
	Selector *felt.Felt
}

func (c *SierraClass) Version() uint64 {
	return 1
}

func (c *SierraClass) Hash() (felt.Felt, error) {
	externalEntryPointsHash := crypto.PoseidonArray(
		flattenSierraEntryPoints(c.EntryPoints.External)...,
	)
	l1HandlerEntryPointsHash := crypto.PoseidonArray(
		flattenSierraEntryPoints(c.EntryPoints.L1Handler)...,
	)
	constructorHash := crypto.PoseidonArray(
		flattenSierraEntryPoints(c.EntryPoints.Constructor)...,
	)
	return crypto.PoseidonArray(
		felt.NewFromBytes[felt.Felt]([]byte("CONTRACT_CLASS_V"+c.SemanticVersion)),
		&externalEntryPointsHash,
		&l1HandlerEntryPointsHash,
		&constructorHash,
		c.AbiHash,
		c.ProgramHash,
	), nil
}

// todo(rdr): Make the SierraVersion returned here a sem ver
// Returns the Sierra version for the Cairo 1 class
//
// Sierra programs contain the version number in two possible formats.
// For pre-1.0-rc0 Cairo versions the program contains the Sierra version
// "0.1.0" as a shortstring in its first Felt (0x302e312e30 = "0.1.0").
// For all subsequent versions the version number is the first three felts
// representing the three parts of a semantic version number.
func (c *SierraClass) SierraVersion() string {
	if c.Program[0].Equal(&SierraVersion010) {
		return "0.1.0"
	}

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

// todo(rdr): this is only used in one place, why is it a global var :(.Fix it
var compiledClassV1Prefix = new(felt.Felt).SetBytes([]byte("COMPILED_CLASS_V1"))

func (c *CasmClass) Hash() felt.Felt {
	var bytecodeHash felt.Felt
	if len(c.BytecodeSegmentLengths.Children) == 0 {
		bytecodeHash = crypto.PoseidonArray(c.Bytecode...)
	} else {
		bytecodeHash = SegmentedBytecodeHash(c.Bytecode, c.BytecodeSegmentLengths.Children)
	}

	externalEntryPointsHash := crypto.PoseidonArray(flattenCompiledEntryPoints(c.External)...)
	l1HandlerEntryPointsHash := crypto.PoseidonArray(flattenCompiledEntryPoints(c.L1Handler)...)
	constructorHash := crypto.PoseidonArray(flattenCompiledEntryPoints(c.Constructor)...)
	return crypto.PoseidonArray(
		compiledClassV1Prefix,
		&externalEntryPointsHash,
		&l1HandlerEntryPointsHash,
		&constructorHash,
		&bytecodeHash,
	)
}

func SegmentedBytecodeHash(
	bytecode []*felt.Felt,
	segmentLengths []SegmentLengths,
) felt.Felt {
	var startingOffset uint64
	var digestSegment func(segments []SegmentLengths) (uint64, felt.Felt)
	digestSegment = func(segments []SegmentLengths) (uint64, felt.Felt) {
		var totalLength uint64
		var digest crypto.PoseidonDigest
		for _, segment := range segments {
			var curSegmentLength uint64
			var curSegmentHash felt.Felt

			if len(segment.Children) == 0 {
				curSegmentLength = segment.Length
				segmentBytecode := bytecode[startingOffset : startingOffset+segment.Length]
				curSegmentHash = crypto.PoseidonArray(segmentBytecode...)
			} else {
				curSegmentLength, curSegmentHash = digestSegment(segment.Children)
			}

			curSegmentLengthFelt := felt.FromUint64[felt.Felt](curSegmentLength)
			digest.Update(&curSegmentLengthFelt)
			digest.Update(&curSegmentHash)

			startingOffset += curSegmentLength
			totalLength += curSegmentLength
		}
		digestRes := digest.Finish()
		digestRes.Add(&digestRes, &felt.One)
		return totalLength, digestRes
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

func flattenCompiledEntryPoints(entryPoints []CasmEntryPoint) []*felt.Felt {
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
		builtinsHash := crypto.PoseidonArray(builtins...)
		result[3*i+2] = &builtinsHash
	}

	return result
}

func VerifyClassHashes(classes map[felt.Felt]ClassDefinition) error {
	for hash, class := range classes {
		if _, ok := class.(*DeprecatedCairoClass); ok {
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

// DeclaredClassDefinition represents a class definition and the block number where it was declared
type DeclaredClassDefinition struct {
	At    uint64 // block number at which the class was declared
	Class ClassDefinition
}

func (d *DeclaredClassDefinition) MarshalBinary() ([]byte, error) {
	classEnc, err := encoder.Marshal(d.Class)
	if err != nil {
		return nil, err
	}

	size := 8 + len(classEnc)
	buf := make([]byte, size)
	binary.BigEndian.PutUint64(buf[:8], d.At)
	copy(buf[8:], classEnc)

	return buf, nil
}

func (d *DeclaredClassDefinition) UnmarshalBinary(data []byte) error {
	if len(data) < minDeclaredClassSize {
		return errors.New("data too short to unmarshal DeclaredClass")
	}

	d.At = binary.BigEndian.Uint64(data[:8])
	return encoder.Unmarshal(data[8:], &d.Class)
}
