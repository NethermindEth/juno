package cbor

import (
	"errors"
	"io"
	"reflect"

	fxcbor "github.com/fxamacker/cbor/v2"
)

// firstTagNum sits in the range IANA leaves unassigned, 65536 to 15309735.
// https://www.iana.org/assignments/cbor-tags/cbor-tags.xhtml
const firstTagNum uint64 = 65536

// fxamackerCodec encodes with github.com/fxamacker/cbor/v2.
type fxamackerCodec struct {
	tags    fxcbor.TagSet
	tagNum  uint64
	encMode fxcbor.EncMode
	decMode fxcbor.DecMode
}

var _ engine = (*fxamackerCodec)(nil)

func newFxamackerCodec() *fxamackerCodec {
	c := &fxamackerCodec{tags: fxcbor.NewTagSet(), tagNum: firstTagNum}
	c.buildModes()
	return c
}

func (c *fxamackerCodec) buildModes() {
	var err error
	c.encMode, err = fxcbor.CanonicalEncOptions().EncModeWithTags(c.tags)
	if err != nil {
		panic(err)
	}

	c.decMode, err = fxcbor.DecOptions{
		MaxArrayElements: 10485760, // Set to a reasonably high value, 10MiB
	}.DecModeWithTags(c.tags)
	if err != nil {
		panic(err)
	}
}

// registerType gives a unique CBOR tag to a type.
func (c *fxamackerCodec) registerType(rType reflect.Type) error {
	if err := c.tags.Add(
		fxcbor.TagOptions{EncTag: fxcbor.EncTagRequired, DecTag: fxcbor.DecTagRequired},
		rType,
		c.tagNum,
	); err != nil {
		return err
	}
	c.buildModes()
	c.tagNum++
	return nil
}

func (c *fxamackerCodec) marshal(v any) ([]byte, error) {
	return c.encMode.Marshal(v)
}

func (c *fxamackerCodec) unmarshal(data []byte, v any) error {
	return c.decMode.Unmarshal(data, v)
}

func (c *fxamackerCodec) unmarshalFirst(data []byte, v any) ([]byte, error) {
	return c.decMode.UnmarshalFirst(data, v)
}

func (c *fxamackerCodec) newEncoder(w io.Writer) Encoder {
	return c.encMode.NewEncoder(w)
}

func (c *fxamackerCodec) isTypeMismatch(err error) bool {
	target := new(fxcbor.UnmarshalTypeError)
	return errors.As(err, &target)
}

func (c *fxamackerCodec) unmarshalStrict(data []byte, v any) error {
	mode, err := fxcbor.DecOptions{
		ExtraReturnErrors: fxcbor.ExtraDecErrorUnknownField,
	}.DecMode()
	if err != nil {
		return err
	}
	return mode.Unmarshal(data, v)
}
