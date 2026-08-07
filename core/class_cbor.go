package core

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/encoder/cborlite"
)

// Hand-written CBOR decoding for class definitions.
//
// A stored class is by far the largest value Juno reads out of the database, and
// the generic decoder walks all of it twice: once to prove it is well formed, then
// again to fill the value. The decoders below read it once, in a single pass.
//
// This file holds only the mapping from a class onto its CBOR shape. Reading the
// bytes is [cborlite]'s job, and its package comment carries the rules that make a
// single pass safe. Felts and felt slices go to the fast paths in the felt package.
//
// Every decoder writes its output only after the whole item has been read, so a
// shape the fast path does not recognise can fall back to the generic decoder
// without leaving a partially decoded value behind.

// Tags the encoder registry assigns to the class definitions, derived from the
// order the types are registered in. TestDeclaredClassCBORTags guards them.
const (
	tagDeprecatedCairoClass = 65541
	tagSierraClass          = 65542
)

// UnmarshalCBOR decodes a declared class without going through the reflection
// based decoder. It falls back to the generic path on any shape it does not
// recognise, so it stays correct for values written by other encoders.
func (d *DeclaredClassDefinition) UnmarshalCBOR(data []byte) error {
	if d.unmarshalCBORFast(data) {
		return nil
	}
	return d.unmarshalCBORGeneric(data)
}

// unmarshalCBORGeneric falls back to the reflection decoder for any shape the fast
// path does not recognise. It handles both on-disk shapes (see unmarshalCBORFast
// for why there are two):
//
//   - byte string: the MarshalBinary wrapper the current encoder writes. Mirror it
//     — unwrap the byte string, then hand the payload to UnmarshalBinary.
//   - map: the self-describing {At, Class} struct older builds stored. Decode
//     straight into the struct through an alias that drops UnmarshalCBOR so the
//     reflection decoder does not recurse back into this method.
func (d *DeclaredClassDefinition) unmarshalCBORGeneric(data []byte) error {
	if major, _, _, ok := cborlite.Head(data, 0); ok && major == cborlite.BytesMajor {
		var payload []byte
		if err := encoder.Unmarshal(data, &payload); err != nil {
			return err
		}
		return d.UnmarshalBinary(payload)
	}

	type plain DeclaredClassDefinition
	return encoder.Unmarshal(data, (*plain)(d))
}

// unmarshalCBORFast decodes a declared class and dispatches on the top-level CBOR
// type, because a declared class exists on disk in two different shapes.
//
// Why two shapes — and why the MAP path is the one that actually matters:
//
// DeclaredClassDefinition implements encoding.BinaryMarshaler, so the *current*
// encoder writes it as a CBOR byte string wrapping [8-byte block number ‖ class].
// It is tempting to treat that byte string as "the" stored shape (an earlier
// version of this file did, guarded by a test that round-trips encoder.Marshal).
// But that test only proves what the *current* code writes — not what is on a
// real node's disk.
//
// Classes are immutable and never rewritten, and the mainnet DB has been filled
// over a long time by builds that predate MarshalBinary. Those builds stored the
// struct directly, so the value on disk is a self-describing CBOR map
// {"At": uint, "Class": <tagged class>}. On a synced node virtually every stored
// class is the map shape; the byte-string shape is essentially never seen.
//
// If the fast path only handled the byte string, it would reject every real
// stored class, every read would fall through to the reflection-based generic
// decoder, and this entire hand-written decoder would be dead code on production
// data — which defeats its whole purpose. The map path is therefore the load-
// bearing one. TestDeclaredClassUnmarshalCBORMapShape guards it against the shape
// a real node actually stores.
func (d *DeclaredClassDefinition) unmarshalCBORFast(data []byte) bool {
	major, _, _, ok := cborlite.Head(data, 0)
	if !ok {
		return false
	}

	switch major {
	case cborlite.MapMajor:
		return d.unmarshalCBORFastMap(data)
	case cborlite.BytesMajor:
		return d.unmarshalCBORFastBinary(data)
	default:
		return false
	}
}

// unmarshalCBORFastMap decodes the self-describing {At, Class} struct map that
// pre-MarshalBinary builds stored — the dominant shape on a synced mainnet node.
func (d *DeclaredClassDefinition) unmarshalCBORFastMap(data []byte) bool {
	var out DeclaredClassDefinition

	next, ok := cborlite.StructMap(data, 0, func(key, data []byte, offset int) (int, bool) {
		var ok bool
		switch string(key) {
		case "At":
			out.At, offset, ok = cborlite.Uint64(data, offset)
		case "Class":
			out.Class, offset, ok = decodeClassDefinitionCBOR(data, offset)
		default:
			// Unknown fields are skipped, matching the generic decoder.
			return cborlite.Skip(data, offset)
		}
		return offset, ok
	})
	if !ok || next != len(data) {
		return false
	}

	*d = out
	return true
}

// unmarshalCBORFastBinary decodes the MarshalBinary byte string the current
// encoder writes: an 8-byte big-endian block number followed by the CBOR class.
func (d *DeclaredClassDefinition) unmarshalCBORFastBinary(data []byte) bool {
	payload, offset, ok := cborlite.ByteString(data, 0)
	if !ok || offset != len(data) || len(payload) < minDeclaredClassSize {
		return false
	}

	class, next, ok := decodeClassDefinitionCBOR(payload[minDeclaredClassSize:], 0)
	if !ok || next != len(payload)-minDeclaredClassSize {
		return false
	}

	d.At = binary.BigEndian.Uint64(payload[:minDeclaredClassSize])
	d.Class = class
	return true
}

// decodeClassDefinitionCBOR reads the tag the registry attached to the concrete
// class type and decodes the value that follows it.
func decodeClassDefinitionCBOR(data []byte, offset int) (ClassDefinition, int, bool) {
	major, tag, next, ok := cborlite.Head(data, offset)
	if !ok || major != cborlite.TagMajor {
		return nil, 0, false
	}

	switch tag {
	case tagSierraClass:
		var class SierraClass
		if next, ok = class.decodeCBOR(data, next); !ok {
			return nil, 0, false
		}
		return &class, next, true
	case tagDeprecatedCairoClass:
		var class DeprecatedCairoClass
		if next, ok = class.decodeCBOR(data, next); !ok {
			return nil, 0, false
		}
		return &class, next, true
	default:
		return nil, 0, false
	}
}

func (c *SierraClass) decodeCBOR(data []byte, offset int) (int, bool) {
	var class SierraClass

	next, ok := cborlite.StructMap(data, offset, func(key, data []byte, offset int) (int, bool) {
		var ok bool
		switch string(key) {
		case "Abi":
			class.Abi, offset, ok = cborlite.String(data, offset)
		case "AbiHash":
			class.AbiHash, offset, ok = cborFeltPointer(data, offset)
		case "EntryPoints":
			return class.EntryPoints.decodeCBOR(data, offset)
		case "Program":
			class.Program, offset, ok = cborFeltSlice(data, offset)
		case "ProgramHash":
			class.ProgramHash, offset, ok = cborFeltPointer(data, offset)
		case "SemanticVersion":
			class.SemanticVersion, offset, ok = cborlite.String(data, offset)
		case "Compiled":
			class.Compiled, offset, ok = decodeCasmClassPointerCBOR(data, offset)
		default:
			// Unknown fields are skipped, matching the generic decoder.
			return cborlite.Skip(data, offset)
		}
		return offset, ok
	})
	if !ok {
		return 0, false
	}

	*c = class
	return next, true
}

func (e *SierraEntryPointsByType) decodeCBOR(data []byte, offset int) (int, bool) {
	var entryPoints SierraEntryPointsByType

	next, ok := cborlite.StructMap(data, offset, func(key, data []byte, offset int) (int, bool) {
		var ok bool
		switch string(key) {
		case "Constructor":
			entryPoints.Constructor, offset, ok = cborSierraEntryPoints(data, offset)
		case "External":
			entryPoints.External, offset, ok = cborSierraEntryPoints(data, offset)
		case "L1Handler":
			entryPoints.L1Handler, offset, ok = cborSierraEntryPoints(data, offset)
		default:
			return cborlite.Skip(data, offset)
		}
		return offset, ok
	})
	if !ok {
		return 0, false
	}

	*e = entryPoints
	return next, true
}

func (e *SierraEntryPoint) decodeCBOR(data []byte, offset int) (int, bool) {
	var entryPoint SierraEntryPoint

	next, ok := cborlite.StructMap(data, offset, func(key, data []byte, offset int) (int, bool) {
		var ok bool
		switch string(key) {
		case "Index":
			entryPoint.Index, offset, ok = cborlite.Uint64(data, offset)
		case "Selector":
			entryPoint.Selector, offset, ok = cborFeltPointer(data, offset)
		default:
			return cborlite.Skip(data, offset)
		}
		return offset, ok
	})
	if !ok {
		return 0, false
	}

	*e = entryPoint
	return next, true
}

func decodeCasmClassPointerCBOR(data []byte, offset int) (*CasmClass, int, bool) {
	if next, isNull := cborlite.ReadNull(data, offset); isNull {
		return nil, next, true
	}

	var class CasmClass
	next, ok := class.decodeCBOR(data, offset)
	if !ok {
		return nil, 0, false
	}
	return &class, next, true
}

func (c *CasmClass) decodeCBOR(data []byte, offset int) (int, bool) {
	var class CasmClass

	next, ok := cborlite.StructMap(data, offset, func(key, data []byte, offset int) (int, bool) {
		var ok bool
		switch string(key) {
		case "Bytecode":
			class.Bytecode, offset, ok = cborFeltSlice(data, offset)
		case "PythonicHints":
			class.PythonicHints, offset, ok = cborlite.ByteStringCopy(data, offset)
		case "CompilerVersion":
			class.CompilerVersion, offset, ok = cborlite.String(data, offset)
		case "Hints":
			class.Hints, offset, ok = cborlite.ByteStringCopy(data, offset)
		case "Prime":
			class.Prime, offset, ok = cborlite.BigInt(data, offset)
		case "External":
			class.External, offset, ok = cborCasmEntryPoints(data, offset)
		case "L1Handler":
			class.L1Handler, offset, ok = cborCasmEntryPoints(data, offset)
		case "Constructor":
			class.Constructor, offset, ok = cborCasmEntryPoints(data, offset)
		case "BytecodeSegmentLengths":
			return class.BytecodeSegmentLengths.decodeCBOR(data, offset)
		default:
			return cborlite.Skip(data, offset)
		}
		return offset, ok
	})
	if !ok {
		return 0, false
	}

	*c = class
	return next, true
}

func (e *CasmEntryPoint) decodeCBOR(data []byte, offset int) (int, bool) {
	var entryPoint CasmEntryPoint

	next, ok := cborlite.StructMap(data, offset, func(key, data []byte, offset int) (int, bool) {
		var ok bool
		switch string(key) {
		case "Offset":
			entryPoint.Offset, offset, ok = cborlite.Uint64(data, offset)
		case "Builtins":
			entryPoint.Builtins, offset, ok = cborlite.StringSlice(data, offset)
		case "Selector":
			entryPoint.Selector, offset, ok = cborFeltPointer(data, offset)
		default:
			return cborlite.Skip(data, offset)
		}
		return offset, ok
	})
	if !ok {
		return 0, false
	}

	*e = entryPoint
	return next, true
}

func (s *SegmentLengths) decodeCBOR(data []byte, offset int) (int, bool) {
	var segment SegmentLengths

	next, ok := cborlite.StructMap(data, offset, func(key, data []byte, offset int) (int, bool) {
		var ok bool
		switch string(key) {
		case "Children":
			segment.Children, offset, ok = cborSegmentLengths(data, offset)
		case "Length":
			segment.Length, offset, ok = cborlite.Uint64(data, offset)
		default:
			return cborlite.Skip(data, offset)
		}
		return offset, ok
	})
	if !ok {
		return 0, false
	}

	*s = segment
	return next, true
}

func (c *DeprecatedCairoClass) decodeCBOR(data []byte, offset int) (int, bool) {
	var class DeprecatedCairoClass

	next, ok := cborlite.StructMap(data, offset, func(key, data []byte, offset int) (int, bool) {
		var ok bool
		switch string(key) {
		case "Abi":
			class.Abi, offset, ok = cborlite.ByteStringCopy(data, offset)
		case "Externals":
			class.Externals, offset, ok = cborDeprecatedEntryPoints(data, offset)
		case "L1Handlers":
			class.L1Handlers, offset, ok = cborDeprecatedEntryPoints(data, offset)
		case "Constructors":
			class.Constructors, offset, ok = cborDeprecatedEntryPoints(data, offset)
		case "Program":
			class.Program, offset, ok = cborlite.String(data, offset)
		default:
			return cborlite.Skip(data, offset)
		}
		return offset, ok
	})
	if !ok {
		return 0, false
	}

	*c = class
	return next, true
}

func (e *DeprecatedEntryPoint) decodeCBOR(data []byte, offset int) (int, bool) {
	var entryPoint DeprecatedEntryPoint

	next, ok := cborlite.StructMap(data, offset, func(key, data []byte, offset int) (int, bool) {
		var ok bool
		switch string(key) {
		case "Selector":
			entryPoint.Selector, offset, ok = cborFeltPointer(data, offset)
		case "Offset":
			entryPoint.Offset, offset, ok = cborFeltPointer(data, offset)
		default:
			return cborlite.Skip(data, offset)
		}
		return offset, ok
	})
	if !ok {
		return 0, false
	}

	*e = entryPoint
	return next, true
}

/****************************************************
		Readers for the types a class is made of
*****************************************************/

// cborFeltPointer decodes a single felt through the felt package fast path, which
// reports how many bytes it consumed so this stays a single pass.
func cborFeltPointer(data []byte, offset int) (*felt.Felt, int, bool) {
	if next, isNull := cborlite.ReadNull(data, offset); isNull {
		return nil, next, true
	}

	value := new(felt.Felt)
	consumed, ok := felt.DecodeCBORPrefix(data[offset:], value)
	if !ok {
		return nil, 0, false
	}
	return value, offset + consumed, true
}

// cborFeltSlice decodes a felt slice through the felt package fast path, which
// reads the whole array in one pass.
func cborFeltSlice(data []byte, offset int) (felt.Slice[felt.Felt], int, bool) {
	if next, isNull := cborlite.ReadNull(data, offset); isNull {
		return nil, next, true
	}

	var slice felt.Slice[felt.Felt]
	consumed, ok := felt.DecodeSliceCBORPrefix(data[offset:], &slice)
	if !ok {
		return nil, 0, false
	}
	return slice, offset + consumed, true
}

func cborSierraEntryPoints(data []byte, offset int) ([]SierraEntryPoint, int, bool) {
	return cborlite.StructSlice(data, offset, (*SierraEntryPoint).decodeCBOR)
}

func cborCasmEntryPoints(data []byte, offset int) ([]CasmEntryPoint, int, bool) {
	return cborlite.StructSlice(data, offset, (*CasmEntryPoint).decodeCBOR)
}

func cborDeprecatedEntryPoints(data []byte, offset int) ([]DeprecatedEntryPoint, int, bool) {
	return cborlite.StructSlice(data, offset, (*DeprecatedEntryPoint).decodeCBOR)
}

func cborSegmentLengths(data []byte, offset int) ([]SegmentLengths, int, bool) {
	return cborlite.StructSlice(data, offset, (*SegmentLengths).decodeCBOR)
}
