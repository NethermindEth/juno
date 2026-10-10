package cbor

import (
	"errors"
	"reflect"
)

// taggedType is an implementation that interface reads decode by its tag.
type taggedType struct {
	tag    uint64
	t      reflect.Type
	decode func([]byte, any) error
}

// A type registered with RegisterType is written as a CBOR tag holding its tag
// number, followed by the value. Reads of the type, or of an interface it
// implements, check the tag and decode the content after it.
var (
	typeTags = map[reflect.Type]uint64{}
	// contentDecoders are the registered decoders of tagged types.
	contentDecoders = map[reflect.Type]func([]byte, any) error{}
)

var errTag = errors.New("cbor: missing or unexpected tag")

// A tag's initial byte holds its major type and either a small tag number or,
// from oneByteTag to eightByteTag, the size of the number that follows.
const (
	tagMajorType = 6
	oneByteTag   = 24
	eightByteTag = 27
)

// splitTag returns the tag number and content of a tagged item.
func splitTag(data []byte) (uint64, []byte, bool) {
	if len(data) == 0 || data[0]>>5 != tagMajorType {
		return 0, nil, false
	}
	info := data[0] & 0x1f
	tag, n := uint64(info), 0
	switch {
	case info >= oneByteTag && info <= eightByteTag:
		// The tag number follows in 1, 2, 4 or 8 big-endian bytes.
		n = 1 << (info - oneByteTag)
		if len(data) < 1+n {
			return 0, nil, false
		}
		tag = 0
		for _, b := range data[1 : 1+n] {
			tag = tag<<8 | uint64(b)
		}
	case info > eightByteTag:
		return 0, nil, false
	}
	content := data[1+n:]
	return tag, content, len(content) > 0
}

// withTag reads an item carrying tag with decode, which reads the content.
func withTag(tag uint64, decode func([]byte, any) error) func([]byte, any) error {
	return func(data []byte, v any) error {
		got, content, ok := splitTag(data)
		if !ok || got != tag {
			return errTag
		}
		return decode(content, v)
	}
}

// RegisterInterface routes reads of i by tag.
func RegisterInterface(i reflect.Type) {
	var types []taggedType
	for t, tag := range typeTags {
		decode := contentDecoders[t]
		if decode != nil && !t.Implements(i) && reflect.PointerTo(t).Implements(i) {
			types = append(types, taggedType{tag: tag, t: t, decode: decode})
		}
	}
	RegisterDecoder(i, func(data []byte, v any) error {
		return decodeInterface(i, types, data, v)
	})
}

// decodeInterface scans types rather than using a map since there are only a few.
func decodeInterface(i reflect.Type, types []taggedType, data []byte, v any) error {
	tag, content, ok := splitTag(data)
	if !ok {
		return errTag
	}
	for idx := range types {
		if types[idx].tag != tag {
			continue
		}
		value := reflect.New(types[idx].t)
		if err := types[idx].decode(content, value.Interface()); err != nil {
			return err
		}
		out := reflect.ValueOf(v).Elem()
		if out.Kind() == reflect.Pointer {
			out.Set(reflect.New(i))
			out = out.Elem()
		}
		out.Set(value)
		return nil
	}
	return errTag
}
