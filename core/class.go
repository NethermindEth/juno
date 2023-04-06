package core

import (
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
	Hash() *felt.Felt
}

// Cairo0Class unambiguously defines a [Contract]'s semantics.
type Cairo0Class struct {
	Abi any
	// External functions defined in the class.
	Externals []EntryPoint
	// Functions that receive L1 messages. See
	// https://www.cairo-lang.org/docs/hello_starknet/l1l2.html#receiving-a-message-from-l1
	L1Handlers []EntryPoint
	// Constructors for the class. Currently, only one is allowed.
	Constructors []EntryPoint
	// An ascii-encoded array of builtin names imported by the class.
	Builtins []*felt.Felt
	// The starknet_keccak hash of the ".json" file compiler output.
	ProgramHash *felt.Felt
	Bytecode    []*felt.Felt
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

func (c *Cairo0Class) Hash() *felt.Felt {
	return crypto.PedersenArray(
		&felt.Zero,
		crypto.PedersenArray(flatten(c.Externals)...),
		crypto.PedersenArray(flatten(c.L1Handlers)...),
		crypto.PedersenArray(flatten(c.Constructors)...),
		crypto.PedersenArray(c.Builtins...),
		c.ProgramHash,
		crypto.PedersenArray(c.Bytecode...),
	)
}

func flatten(entryPoints []EntryPoint) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*2)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first because the order
		// influences the class hash.
		result[2*i] = entryPoint.Selector
		result[2*i+1] = entryPoint.Offset
	}
	return result
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

type CantVerifyClassHashError struct {
	c           Class
	hashFailure error
	next        *CantVerifyClassHashError
}

func (e CantVerifyClassHashError) Unwrap() error {
	if e.next != nil {
		return *e.next
	}
	return nil
}

func (e CantVerifyClassHashError) Error() string {
	return fmt.Sprintf("cannot verify class hash: %s", e.hashFailure)
}

func (e CantVerifyClassHashError) Class() Class {
	return e.c
}

func verifyClassHash(c Class, hash *felt.Felt) *CantVerifyClassHashError {
	if c == nil {
		return &CantVerifyClassHashError{
			hashFailure: fmt.Errorf("class is nil"),
		}
	}

	cHash := c.Hash()
	if !cHash.Equal(hash) {
		return &CantVerifyClassHashError{
			c:           c,
			hashFailure: fmt.Errorf("calculated hash: %v, received hash: %v", cHash.String(), hash.String()),
		}
	}

	return nil
}

func VerifyClassHashes(classes map[felt.Felt]Class) error {
	var head *CantVerifyClassHashError
	for hash, class := range classes {
		if err := verifyClassHash(class, &hash); err != nil {
			err.next = head
			head = err
		}
	}

	if head != nil {
		return *head
	}

	return nil
}
