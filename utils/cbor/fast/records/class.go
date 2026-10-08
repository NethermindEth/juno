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

// ReadSierraClass reads the map of a stored Sierra class, after its tag.
func ReadSierraClass(r *fast.Reader, dst *core.SierraClass) error {
	return readFields(r, foldSierraClass, func(key string) (bool, error) {
		var err error
		switch key {
		case "Abi":
			err = readString(r, &dst.Abi)
		case "AbiHash":
			err = readFelt(r, &dst.AbiHash)
		case "EntryPoints":
			err = readSierraEntryPoints(r, &dst.EntryPoints)
		case "Program":
			err = readFeltSlice(r, &dst.Program)
		case "ProgramHash":
			err = readFelt(r, &dst.ProgramHash)
		case "SemanticVersion":
			err = readString(r, &dst.SemanticVersion)
		case "Compiled":
			err = readPtr(r, &dst.Compiled, readCasmClass)
		default:
			return false, nil
		}
		return true, err
	})
}

func readSierraEntryPoints(r *fast.Reader, dst *core.SierraEntryPointsByType) error {
	return readFields(r, foldSierraEntryPointsByType, func(key string) (bool, error) {
		var err error
		switch key {
		case "Constructor":
			err = readSlice(r, &dst.Constructor, readSierraEntryPoint)
		case "External":
			err = readSlice(r, &dst.External, readSierraEntryPoint)
		case "L1Handler":
			err = readSlice(r, &dst.L1Handler, readSierraEntryPoint)
		default:
			return false, nil
		}
		return true, err
	})
}

func readSierraEntryPoint(r *fast.Reader, dst *core.SierraEntryPoint) error {
	return readFields(r, foldSierraEntryPoint, func(key string) (bool, error) {
		var err error
		switch key {
		case "Index":
			err = readUint64(r, &dst.Index)
		case "Selector":
			err = readFelt(r, &dst.Selector)
		default:
			return false, nil
		}
		return true, err
	})
}

func readCasmClass(r *fast.Reader, dst *core.CasmClass) error {
	return readFields(r, foldCasmClass, func(key string) (bool, error) {
		var err error
		switch key {
		case "Bytecode":
			err = readFeltSlice(r, &dst.Bytecode)
		case "PythonicHints":
			err = readRaw(r, (*[]byte)(&dst.PythonicHints))
		case "CompilerVersion":
			err = readString(r, &dst.CompilerVersion)
		case "Hints":
			err = readRaw(r, (*[]byte)(&dst.Hints))
		case "Prime":
			err = readBigIntPtr(r, dst)
		case "External":
			err = readSlice(r, &dst.External, readCasmEntryPoint)
		case "L1Handler":
			err = readSlice(r, &dst.L1Handler, readCasmEntryPoint)
		case "Constructor":
			err = readSlice(r, &dst.Constructor, readCasmEntryPoint)
		case "BytecodeSegmentLengths":
			err = readSegmentLengths(r, &dst.BytecodeSegmentLengths, 0)
		default:
			return false, nil
		}
		return true, err
	})
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

func readCasmEntryPoint(r *fast.Reader, dst *core.CasmEntryPoint) error {
	return readFields(r, foldCasmEntryPoint, func(key string) (bool, error) {
		var err error
		switch key {
		case "Offset":
			err = readUint64(r, &dst.Offset)
		case "Builtins":
			err = readStringSlice(r, &dst.Builtins)
		case "Selector":
			err = readFelt(r, &dst.Selector)
		default:
			return false, nil
		}
		return true, err
	})
}

func readSegmentLengths(r *fast.Reader, dst *core.SegmentLengths, depth int) error {
	if depth > maxSegmentDepth {
		return fast.ErrDepth
	}
	return readFields(r, foldSegmentLengths, func(key string) (bool, error) {
		var err error
		switch key {
		case "Children":
			err = readSlice(r, &dst.Children, func(r *fast.Reader, child *core.SegmentLengths) error {
				return readSegmentLengths(r, child, depth+1)
			})
		case "Length":
			err = readUint64(r, &dst.Length)
		default:
			return false, nil
		}
		return true, err
	})
}
