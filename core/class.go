package core

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/crypto/blake2s"
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
	Abi json.RawMessage `cbor:"1,keyasint,omitempty"`
	// External functions defined in the class.
	Externals []DeprecatedEntryPoint `cbor:"2,keyasint,omitempty"`
	// Functions that receive L1 messages. See
	// https://www.cairo-lang.org/docs/hello_starknet/l1l2.html#receiving-a-message-from-l1
	L1Handlers []DeprecatedEntryPoint `cbor:"3,keyasint,omitempty"`
	// Constructors for the class. Currently, only one is allowed.
	Constructors []DeprecatedEntryPoint `cbor:"4,keyasint,omitempty"`
	// Base64 encoding of compressed Program
	Program string `cbor:"5,keyasint,omitempty"`
}

type DeprecatedEntryPoint struct {
	// starknet_keccak hash of the function signature.
	Selector *felt.Felt `cbor:"1,keyasint,omitempty"`
	// The offset of the instruction in the class's bytecode.
	Offset *felt.Felt `cbor:"2,keyasint,omitempty"`
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
	Abi         string                  `cbor:"1,keyasint,omitempty"`
	AbiHash     *felt.Felt              `cbor:"2,keyasint,omitempty"`
	EntryPoints SierraEntryPointsByType `cbor:"3,keyasint,omitempty"`
	Program     []*felt.Felt            `cbor:"4,keyasint,omitempty"`
	ProgramHash *felt.Felt              `cbor:"5,keyasint,omitempty"`
	// TODO: Remove this semantic version on a follow up PR. Let's put Sierra version instead
	SemanticVersion string     `cbor:"6,keyasint,omitempty"`
	Compiled        *CasmClass `cbor:"7,keyasint,omitempty"`
}

type SegmentLengths struct {
	Children []SegmentLengths `cbor:"1,keyasint,omitempty"`
	Length   uint64           `cbor:"2,keyasint,omitempty"`
}

type CasmClass struct {
	Bytecode               []*felt.Felt     `cbor:"1,keyasint,omitempty"`
	PythonicHints          json.RawMessage  `cbor:"2,keyasint,omitempty"`
	CompilerVersion        string           `cbor:"3,keyasint,omitempty"`
	Hints                  json.RawMessage  `cbor:"4,keyasint,omitempty"`
	Prime                  *big.Int         `cbor:"5,keyasint,omitempty"`
	External               []CasmEntryPoint `cbor:"6,keyasint,omitempty"`
	L1Handler              []CasmEntryPoint `cbor:"7,keyasint,omitempty"`
	Constructor            []CasmEntryPoint `cbor:"8,keyasint,omitempty"`
	BytecodeSegmentLengths SegmentLengths   `cbor:"9,keyasint,omitempty"`
}

type CasmEntryPoint struct {
	Offset   uint64     `cbor:"1,keyasint,omitempty"`
	Builtins []string   `cbor:"2,keyasint,omitempty"`
	Selector *felt.Felt `cbor:"3,keyasint,omitempty"`
}

type SierraEntryPointsByType struct {
	Constructor []SierraEntryPoint `cbor:"1,keyasint,omitempty"`
	External    []SierraEntryPoint `cbor:"2,keyasint,omitempty"`
	L1Handler   []SierraEntryPoint `cbor:"3,keyasint,omitempty"`
}

type SierraEntryPoint struct {
	Index    uint64     `cbor:"1,keyasint,omitempty"`
	Selector *felt.Felt `cbor:"2,keyasint,omitempty"`
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

// CasmHashVersion represents the version of the hash function
// used to compute the compiled class hash
type CasmHashVersion int

const (
	// HashVersionV1 uses Poseidon hash
	HashVersionV1 CasmHashVersion = iota + 1
	// HashVersionV2 uses Blake2s hash
	HashVersionV2
)

// Hasher wraps hash algorithm operations
type Hasher interface {
	HashArray(felts ...*felt.Felt) felt.Felt
	NewDigest() crypto.Digest
}

type poseidonHasher struct{}

func (h poseidonHasher) HashArray(felts ...*felt.Felt) felt.Felt {
	return crypto.PoseidonArray(felts...)
}

func (h poseidonHasher) NewDigest() crypto.Digest {
	return &crypto.PoseidonDigest{}
}

type blake2sHasher struct{}

func (h blake2sHasher) HashArray(felts ...*felt.Felt) felt.Felt {
	hash := blake2s.Blake2sArray(felts...)
	return felt.Felt(hash)
}

func (h blake2sHasher) NewDigest() crypto.Digest {
	digest := blake2s.NewDigest()
	return &digest
}

func NewCasmHasher(version CasmHashVersion) Hasher {
	switch version {
	case HashVersionV2:
		return blake2sHasher{}
	case HashVersionV1:
		return poseidonHasher{}
	default:
		return blake2sHasher{}
	}
}

// todo(rdr): this is only used in one place, why is it a global var :(.Fix it
var compiledClassV1Prefix = felt.NewFromBytes[felt.Felt]([]byte("COMPILED_CLASS_V1"))

// Hash computes the class hash using the specified hash version
func (c *CasmClass) Hash(version CasmHashVersion) felt.Felt {
	h := NewCasmHasher(version)

	var bytecodeHash felt.Felt
	if len(c.BytecodeSegmentLengths.Children) == 0 {
		bytecodeHash = h.HashArray(c.Bytecode...)
	} else {
		bytecodeHash = SegmentedBytecodeHash(c.Bytecode, c.BytecodeSegmentLengths.Children, h)
	}

	externalEntryPointsHash := h.HashArray(flattenCompiledEntryPoints(c.External, h)...)
	l1HandlerEntryPointsHash := h.HashArray(flattenCompiledEntryPoints(c.L1Handler, h)...)
	constructorHash := h.HashArray(flattenCompiledEntryPoints(c.Constructor, h)...)

	return h.HashArray(
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
	h Hasher,
) felt.Felt {
	var startingOffset uint64
	var digestSegment func(segments []SegmentLengths) (uint64, felt.Felt)
	digestSegment = func(segments []SegmentLengths) (uint64, felt.Felt) {
		var totalLength uint64
		digest := h.NewDigest()

		for _, segment := range segments {
			var curSegmentLength uint64
			var curSegmentHash felt.Felt

			if len(segment.Children) == 0 {
				curSegmentLength = segment.Length
				segmentBytecode := bytecode[startingOffset : startingOffset+segment.Length]
				curSegmentHash = h.HashArray(segmentBytecode...)
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
		result[2*i+1] = felt.NewFromUint64[felt.Felt](entryPoint.Index)
	}
	return result
}

func flattenCompiledEntryPoints(entryPoints []CasmEntryPoint, h Hasher) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*3)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first, then Offset is second because the order
		// influences the class hash.
		result[3*i] = entryPoint.Selector
		result[3*i+1] = felt.NewFromUint64[felt.Felt](entryPoint.Offset)
		builtins := make([]*felt.Felt, len(entryPoint.Builtins))
		for idx, buil := range entryPoint.Builtins {
			builtins[idx] = felt.NewFromBytes[felt.Felt]([]byte(buil))
		}
		builtinsHash := h.HashArray(builtins...)
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
