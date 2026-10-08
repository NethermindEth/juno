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
	var seen uint64
	return readFields(r, foldSierraClass, func(key string) (bool, error) {
		var err error
		switch key {
		case "Abi":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readString(r, &dst.Abi)
		case "AbiHash":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.AbiHash)
		case "EntryPoints":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readSierraEntryPoints(r, &dst.EntryPoints)
		case "Program":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.Program)
		case "ProgramHash":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.ProgramHash)
		case "SemanticVersion":
			if once(&seen, 6) {
				return false, fast.ErrUnsupported
			}
			err = readString(r, &dst.SemanticVersion)
		case "Compiled":
			if once(&seen, 7) {
				return false, fast.ErrUnsupported
			}
			err = readPtr(r, &dst.Compiled, readCasmClass)
		default:
			return false, nil
		}
		return true, err
	})
}

func readSierraEntryPoints(r *fast.Reader, dst *core.SierraEntryPointsByType) error {
	var seen uint64
	return readFields(r, foldSierraEntryPointsByType, func(key string) (bool, error) {
		var err error
		switch key {
		case "Constructor":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readSlice(r, &dst.Constructor, readSierraEntryPoint)
		case "External":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readSlice(r, &dst.External, readSierraEntryPoint)
		case "L1Handler":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readSlice(r, &dst.L1Handler, readSierraEntryPoint)
		default:
			return false, nil
		}
		return true, err
	})
}

func readSierraEntryPoint(r *fast.Reader, dst *core.SierraEntryPoint) error {
	var seen uint64
	return readFields(r, foldSierraEntryPoint, func(key string) (bool, error) {
		var err error
		switch key {
		case "Index":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Index)
		case "Selector":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.Selector)
		default:
			return false, nil
		}
		return true, err
	})
}

func readCasmClass(r *fast.Reader, dst *core.CasmClass) error {
	var seen uint64
	return readFields(r, foldCasmClass, func(key string) (bool, error) {
		var err error
		switch key {
		case "Bytecode":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.Bytecode)
		case "PythonicHints":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readRaw(r, (*[]byte)(&dst.PythonicHints))
		case "CompilerVersion":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readString(r, &dst.CompilerVersion)
		case "Hints":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readRaw(r, (*[]byte)(&dst.Hints))
		case "Prime":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readBigIntPtr(r, dst)
		case "External":
			if once(&seen, 6) {
				return false, fast.ErrUnsupported
			}
			err = readSlice(r, &dst.External, readCasmEntryPoint)
		case "L1Handler":
			if once(&seen, 7) {
				return false, fast.ErrUnsupported
			}
			err = readSlice(r, &dst.L1Handler, readCasmEntryPoint)
		case "Constructor":
			if once(&seen, 8) {
				return false, fast.ErrUnsupported
			}
			err = readSlice(r, &dst.Constructor, readCasmEntryPoint)
		case "BytecodeSegmentLengths":
			if once(&seen, 9) {
				return false, fast.ErrUnsupported
			}
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
	var seen uint64
	return readFields(r, foldCasmEntryPoint, func(key string) (bool, error) {
		var err error
		switch key {
		case "Offset":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Offset)
		case "Builtins":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readStringSlice(r, &dst.Builtins)
		case "Selector":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
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
	var seen uint64
	return readFields(r, foldSegmentLengths, func(key string) (bool, error) {
		var err error
		switch key {
		case "Children":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readSlice(r, &dst.Children, func(r *fast.Reader, child *core.SegmentLengths) error {
				return readSegmentLengths(r, child, depth+1)
			})
		case "Length":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Length)
		default:
			return false, nil
		}
		return true, err
	})
}
