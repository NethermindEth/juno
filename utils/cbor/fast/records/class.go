package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

// maxSegmentDepth bounds how deep bytecode segment lengths recurse. Each level takes a map and
// an array of the 32 nesting levels a record may have, and the record has three above them.
const maxSegmentDepth = 14

// ReadClassDefinition reads a stored class: a tag naming its type, then the class. Only
// Sierra classes are read here; the tag of anything else makes the read unsupported.
func ReadClassDefinition(sierraTag uint64) func(r *fast.Reader, dst *core.ClassDefinition) error {
	return func(r *fast.Reader, dst *core.ClassDefinition) error {
		if r.Null() {
			*dst = nil
			return nil
		}
		tag, err := r.Tag()
		if err != nil {
			return err
		}
		if tag != sierraTag {
			return fast.ErrUnsupported
		}
		class := new(core.SierraClass)
		if err := ReadSierraClass(r, class); err != nil {
			return err
		}
		*dst = class
		return nil
	}
}

const (
	sierraAbi = iota
	sierraAbiHash
	sierraEntryPoints
	sierraProgram
	sierraProgramHash
	sierraSemanticVersion
	sierraCompiled
)

var sierraClassFields = fieldsOf[core.SierraClass]([]string{
	sierraAbi:             "Abi",
	sierraAbiHash:         "AbiHash",
	sierraEntryPoints:     "EntryPoints",
	sierraProgram:         "Program",
	sierraProgramHash:     "ProgramHash",
	sierraSemanticVersion: "SemanticVersion",
	sierraCompiled:        "Compiled",
})

// ReadSierraClass reads the map of a stored Sierra class, after its tag.
func ReadSierraClass(r *fast.Reader, dst *core.SierraClass) error {
	s, err := r.OpenStruct(sierraClassFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case sierraAbi:
			err = readString(r, &dst.Abi)
		case sierraAbiHash:
			err = readFelt(r, &dst.AbiHash)
		case sierraEntryPoints:
			err = readSierraEntryPoints(r, &dst.EntryPoints)
		case sierraProgram:
			err = readFeltSlice(r, &dst.Program)
		case sierraProgramHash:
			err = readFelt(r, &dst.ProgramHash)
		case sierraSemanticVersion:
			err = readString(r, &dst.SemanticVersion)
		case sierraCompiled:
			err = readPtr(r, &dst.Compiled, readCasmClass)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	entryPointsConstructor = iota
	entryPointsExternal
	entryPointsL1Handler
)

var sierraEntryPointsFields = fieldsOf[core.SierraEntryPointsByType]([]string{
	entryPointsConstructor: "Constructor",
	entryPointsExternal:    "External",
	entryPointsL1Handler:   "L1Handler",
})

func readSierraEntryPoints(r *fast.Reader, dst *core.SierraEntryPointsByType) error {
	s, err := r.OpenStruct(sierraEntryPointsFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case entryPointsConstructor:
			err = readSlice(r, &dst.Constructor, readSierraEntryPoint)
		case entryPointsExternal:
			err = readSlice(r, &dst.External, readSierraEntryPoint)
		case entryPointsL1Handler:
			err = readSlice(r, &dst.L1Handler, readSierraEntryPoint)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	sierraEntryPointIndex = iota
	sierraEntryPointSelector
)

var sierraEntryPointFields = fieldsOf[core.SierraEntryPoint]([]string{
	sierraEntryPointIndex:    "Index",
	sierraEntryPointSelector: "Selector",
})

func readSierraEntryPoint(r *fast.Reader, dst *core.SierraEntryPoint) error {
	s, err := r.OpenStruct(sierraEntryPointFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case sierraEntryPointIndex:
			err = readUint64(r, &dst.Index)
		case sierraEntryPointSelector:
			err = readFelt(r, &dst.Selector)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	casmBytecode = iota
	casmPythonicHints
	casmCompilerVersion
	casmHints
	casmPrime
	casmExternal
	casmL1Handler
	casmConstructor
	casmBytecodeSegmentLengths
)

var casmClassFields = fieldsOf[core.CasmClass]([]string{
	casmBytecode:               "Bytecode",
	casmPythonicHints:          "PythonicHints",
	casmCompilerVersion:        "CompilerVersion",
	casmHints:                  "Hints",
	casmPrime:                  "Prime",
	casmExternal:               "External",
	casmL1Handler:              "L1Handler",
	casmConstructor:            "Constructor",
	casmBytecodeSegmentLengths: "BytecodeSegmentLengths",
})

func readCasmClass(r *fast.Reader, dst *core.CasmClass) error {
	s, err := r.OpenStruct(casmClassFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case casmBytecode:
			err = readFeltSlice(r, &dst.Bytecode)
		case casmPythonicHints:
			err = readRaw(r, (*[]byte)(&dst.PythonicHints))
		case casmCompilerVersion:
			err = readString(r, &dst.CompilerVersion)
		case casmHints:
			err = readRaw(r, (*[]byte)(&dst.Hints))
		case casmPrime:
			err = readBigIntPtr(r, dst)
		case casmExternal:
			err = readSlice(r, &dst.External, readCasmEntryPoint)
		case casmL1Handler:
			err = readSlice(r, &dst.L1Handler, readCasmEntryPoint)
		case casmConstructor:
			err = readSlice(r, &dst.Constructor, readCasmEntryPoint)
		case casmBytecodeSegmentLengths:
			err = readSegmentLengths(r, &dst.BytecodeSegmentLengths, 0)
		default:
			err = r.Skip()
		}
	}
	return err
}

func readBigIntPtr(r *fast.Reader, dst *core.CasmClass) error {
	if r.Null() {
		dst.Prime = nil
		return nil
	}
	value, err := r.BigInt()
	if err != nil {
		return err
	}
	dst.Prime = value
	return nil
}

const (
	casmEntryPointOffset = iota
	casmEntryPointBuiltins
	casmEntryPointSelector
)

var casmEntryPointFields = fieldsOf[core.CasmEntryPoint]([]string{
	casmEntryPointOffset:   "Offset",
	casmEntryPointBuiltins: "Builtins",
	casmEntryPointSelector: "Selector",
})

func readCasmEntryPoint(r *fast.Reader, dst *core.CasmEntryPoint) error {
	s, err := r.OpenStruct(casmEntryPointFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case casmEntryPointOffset:
			err = readUint64(r, &dst.Offset)
		case casmEntryPointBuiltins:
			err = readStringSlice(r, &dst.Builtins)
		case casmEntryPointSelector:
			err = readFelt(r, &dst.Selector)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	segmentChildren = iota
	segmentLength
)

var segmentLengthsFields = fieldsOf[core.SegmentLengths]([]string{
	segmentChildren: "Children",
	segmentLength:   "Length",
})

func readSegmentLengths(r *fast.Reader, dst *core.SegmentLengths, depth int) error {
	if depth > maxSegmentDepth {
		return fast.ErrDepth
	}
	s, err := r.OpenStruct(segmentLengthsFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case segmentChildren:
			err = readSlice(r, &dst.Children, func(r *fast.Reader, child *core.SegmentLengths) error {
				return readSegmentLengths(r, child, depth+1)
			})
		case segmentLength:
			err = readUint64(r, &dst.Length)
		default:
			err = r.Skip()
		}
	}
	return err
}
