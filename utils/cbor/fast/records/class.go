package records

import (
	"math/big"

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

func readBigIntPtr(r *fast.Reader, dst **big.Int) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value, err := r.BigInt()
	if err != nil {
		return err
	}
	*dst = value
	return nil
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
