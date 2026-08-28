package encoder

import (
	"errors"
	"io"
	"reflect"

	fxcbor "github.com/fxamacker/cbor/v2"
)

// firstTagNum sits in the range IANA leaves unassigned, 65536 to 15309735.
// https://www.iana.org/assignments/cbor-tags/cbor-tags.xhtml
const firstTagNum uint64 = 65536

// fxamackerEngine encodes with github.com/fxamacker/cbor/v2.
type fxamackerEngine struct {
	tags       fxcbor.TagSet
	tagNum     uint64
	encMode    fxcbor.EncMode
	decMode    fxcbor.DecMode
	strictMode fxcbor.DecMode
}

var _ Engine = (*fxamackerEngine)(nil)

func newFxamackerEngine() *fxamackerEngine {
	c := &fxamackerEngine{tags: fxcbor.NewTagSet(), tagNum: firstTagNum}
	c.buildModes()

	var err error
	c.strictMode, err = fxcbor.DecOptions{
		ExtraReturnErrors: fxcbor.ExtraDecErrorUnknownField,
	}.DecMode()
	if err != nil {
		panic(err)
	}

	return c
}

func (c *fxamackerEngine) buildModes() {
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

func (c *fxamackerEngine) NewEncoder(w io.Writer) Encoder {
	return c.encMode.NewEncoder(w)
}

func (c *fxamackerEngine) Marshal(v any) ([]byte, error) {
	return c.encMode.Marshal(v)
}

func (c *fxamackerEngine) Unmarshal(data []byte, v any) error {
	return c.decMode.Unmarshal(data, v)
}

func (c *fxamackerEngine) UnmarshalFirst(data []byte, v any) ([]byte, error) {
	return c.decMode.UnmarshalFirst(data, v)
}

func (c *fxamackerEngine) UnmarshalStrict(data []byte, v any) error {
	return c.strictMode.Unmarshal(data, v)
}

// RegisterType gives a unique CBOR tag to a type.
func (c *fxamackerEngine) RegisterType(rType reflect.Type) error {
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

func (c *fxamackerEngine) IsTypeMismatch(err error) bool {
	target := new(fxcbor.UnmarshalTypeError)
	return errors.As(err, &target)
}
