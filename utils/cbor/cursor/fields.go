package cursor

import (
	"encoding/binary"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

// Done is returned by NextField and NextWanted when the struct's map has no pairs left, and is
// closed. Unknown is returned by NextField when the key names no field, and its value is next, to
// skip.
const (
	Done    = -1
	Unknown = -2
)

// maxFields is how many fields a Struct can track, one bit each.
const maxFields = 64

// Fields matches the keys of a struct's map to field numbers, by an exact name first and then
// ignoring case. order holds each key as encoded, in canonical order (shorter first, then
// bytewise), names the wire name of each field by number, and all has the bit of every field set.
type Fields struct {
	order []fieldKey
	names []string
	all   uint64
}

// keyWords is how many 8 byte words of a key are compared as words.
const keyWords = 3

// StaticField is a precomputed lookup entry emitted by a reader generator.
// Words and Masks contain the first 24 encoded bytes in little-endian words.
// Index is the field number, Bit is 1<<Index, and Short says those words cover
// the entire encoded key. Callers must not modify entries after installing them.
type StaticField struct {
	Words, Masks [keyWords]uint64
	Encoded      string
	Index        int
	Bit          uint64
	Short        bool
}

type fieldKey = StaticField

// NewStaticFields installs precomputed lookup metadata without sorting or folding
// names at runtime. It is intended for generated readers. names must satisfy
// NewFields' requirements; order must contain exactly one correctly encoded entry
// per name, sorted by Encoded, with matching words, masks, index, bit and short flag.
// Neither slice may be modified after this call.
func NewStaticFields(names []string, order []StaticField) *Fields {
	return &Fields{order: order, names: names, all: 1<<len(names) - 1}
}

func newFieldKey(encoded string, field int) fieldKey {
	out := fieldKey{Encoded: encoded, Index: field, Bit: 1 << field, Short: len(encoded) <= keyWords*8}
	for i := 0; i < len(encoded) && i < keyWords*8; i++ {
		out.Words[i/8] |= uint64(encoded[i]) << (i % 8 * 8)
		out.Masks[i/8] |= 0xff << (i % 8 * 8)
	}
	return out
}

// NewFields returns the Fields of a struct whose field i has the wire name names[i]. The names
// must be ASCII, under 256 bytes and distinct ignoring case.
func NewFields(names ...string) *Fields {
	if len(names) > maxFields {
		panic("cursor: too many fields")
	}
	out := &Fields{order: make([]fieldKey, len(names)), names: names, all: 1<<len(names) - 1}
	folded := make(map[string]bool, len(names))
	for i, name := range names {
		var head string
		switch {
		case len(name) < info1Byte:
			head = string([]byte{majorString | byte(len(name))})
		case len(name) < 1<<8:
			head = string([]byte{majorString | info1Byte, byte(len(name))})
		default:
			panic("cursor: field name too long: " + name)
		}
		for _, c := range []byte(name) {
			if c >= utf8.RuneSelf {
				panic("cursor: field name is not ASCII: " + name)
			}
		}
		lower := strings.ToLower(name)
		if folded[lower] {
			panic("cursor: field names repeat ignoring case: " + name)
		}
		folded[lower] = true
		out.order[i] = newFieldKey(head+name, i)
	}
	// The head of an encoded name holds its length, so comparing the bytes puts shorter names first,
	// as a canonical encoder does.
	slices.SortFunc(out.order, func(a, b fieldKey) int {
		return strings.Compare(a.Encoded, b.Encoded)
	})
	return out
}

// Struct is the progress of reading one struct's map. It holds the pairs left, where in
// fields.order the next key is expected, a bit for each field read, and whether a map was opened,
// which Done closes.
type Struct struct {
	fields *Fields
	left   int
	next   int
	seen   uint64
	open   bool
}

// OpenStruct reads the head of a struct's map. A null struct has no pairs.
func (c *Cursor) OpenStruct(fields *Fields) (Struct, error) {
	if c.Null() {
		return Struct{fields: fields}, nil
	}
	count, err := c.MapLen()
	if err != nil {
		return Struct{fields: fields}, err
	}
	return Struct{fields: fields, left: count, open: true}, nil
}

// NextField returns the field the next key names, Unknown if it names none, or Done once no pairs
// are left. A field named twice is ErrUnsupported. Stored maps are canonical, so the key expected
// next is compared first.
func (c *Cursor) NextField(s *Struct) (int, error) {
	// The common case comes first, when the key is the one expected next and its field is not yet
	// read.
	if s.left > 0 && s.next < len(s.fields.order) && c.pos+keyWords*8 <= len(c.buf) {
		key := &s.fields.order[s.next]
		buf := c.buf[c.pos : c.pos+keyWords*8]
		if (binary.LittleEndian.Uint64(buf)^key.Words[0])&key.Masks[0]|
			(binary.LittleEndian.Uint64(buf[8:])^key.Words[1])&key.Masks[1]|
			(binary.LittleEndian.Uint64(buf[16:])^key.Words[2])&key.Masks[2]|
			s.seen&key.Bit == 0 && key.Short {
			c.pos += len(key.Encoded)
			s.left--
			s.next++
			s.seen |= key.Bit
			return key.Index, nil
		}
	}
	return c.nextField(s)
}

// NextWanted is NextField for a struct read in part. It skips the pairs of other keys, in any
// order, and the first of two keys naming a field wins.
func (c *Cursor) NextWanted(s *Struct) (int, error) {
	for ; s.left > 0; s.left-- {
		key, isText, err := c.Key()
		if err != nil {
			return Done, err
		}
		if isText && s.seen != s.fields.all {
			if field, ok := s.fields.match(key); ok && s.seen&(1<<field) == 0 {
				s.seen |= 1 << field
				s.left--
				return field, nil
			}
		}
		if err := c.Skip(); err != nil {
			return Done, err
		}
	}
	if s.open {
		s.open = false
		c.Leave()
	}
	return Done, nil
}

// nextField looks for the key among the fields after the last one matched, then by name.
func (c *Cursor) nextField(s *Struct) (int, error) {
	if s.left == 0 {
		if s.open {
			s.open = false
			c.Leave()
		}
		return Done, nil
	}
	s.left--
	if i := s.find(c.buf[c.pos:]); i >= 0 {
		key := &s.fields.order[i]
		c.pos += len(key.Encoded)
		s.next = i + 1
		return c.mark(s, key.Index)
	}
	return c.otherField(s)
}

// find returns where the key at the start of buf sits among the fields after the last one matched,
// or -1.
func (s *Struct) find(buf []byte) int {
	order := s.fields.order
	if len(buf) >= keyWords*8 {
		// Compare a word at a time; the bytes past a short key are masked off.
		w0 := binary.LittleEndian.Uint64(buf)
		w1 := binary.LittleEndian.Uint64(buf[8:])
		w2 := binary.LittleEndian.Uint64(buf[16:])
		head := byte(w0)
		for i := s.next; i < len(order); i++ {
			key := &order[i]
			if keyHead := byte(key.Words[0]); keyHead != head {
				if keyHead > head {
					break // every key from here on sorts after this one
				}
				continue
			}
			if (w0^key.Words[0])&key.Masks[0]|
				(w1^key.Words[1])&key.Masks[1]|
				(w2^key.Words[2])&key.Masks[2] == 0 &&
				(key.Short || len(buf) >= len(key.Encoded) &&
					string(buf[keyWords*8:len(key.Encoded)]) == key.Encoded[keyWords*8:]) {
				return i
			}
		}
		return -1
	}
	for i := s.next; i < len(order) && len(buf) > 0; i++ {
		key := order[i].Encoded
		if key[0] != buf[0] {
			if key[0] > buf[0] {
				break
			}
			continue
		}
		if len(buf) >= len(key) && string(buf[:len(key)]) == key {
			return i
		}
	}
	return -1
}

// otherField matches a key that is not where a canonical encoder would put it.
func (c *Cursor) otherField(s *Struct) (int, error) {
	key, isText, err := c.Key()
	if err != nil {
		return Done, err
	}
	if !isText {
		return Unknown, nil
	}
	field, ok := s.fields.match(key)
	if !ok {
		return Unknown, nil
	}
	return c.mark(s, field)
}

func (c *Cursor) mark(s *Struct, field int) (int, error) {
	bit := uint64(1) << field
	if s.seen&bit != 0 {
		return Done, c.fail(ErrUnsupported, "field %s named twice", s.fields.names[field])
	}
	s.seen |= bit
	return field, nil
}

// match finds the field a key names exactly or, failing that, ignoring case. Folding ASCII letters
// alone equals strings.EqualFold, because a rune outside ASCII that folds to an ASCII letter takes
// more than one byte. Names are distinct ignoring case, so at most one matches.
func (f *Fields) match(key []byte) (int, bool) {
	for i, name := range f.names {
		if string(key) == name {
			return i, true
		}
	}
	for i, name := range f.names {
		if len(key) == len(name) && equalFoldASCII(key, name) {
			return i, true
		}
	}
	return 0, false
}

// equalFoldASCII compares a key with an ASCII name of the same length, ignoring case.
func equalFoldASCII(key []byte, name string) bool {
	const lowerDelta byte = 'a' - 'A'
	for i := range len(name) {
		a, b := key[i], name[i]
		if 'A' <= a && a <= 'Z' {
			a += lowerDelta
		}
		if 'A' <= b && b <= 'Z' {
			b += lowerDelta
		}
		if a != b {
			return false
		}
	}
	return true
}

// Key reads a map key. A text key is checked as UTF-8 and returned as a sub-slice of the input; an
// integer key is consumed and reported as not text.
func (c *Cursor) Key() (key []byte, isText bool, err error) {
	// A short text key with its length in the head or the byte after it.
	buf := c.buf[c.pos:]
	if len(buf) > 0 {
		length, size := int(buf[0]-majorString), 1
		if length == info1Byte && len(buf) > 1 {
			length, size = int(buf[1]), 2
		}
		if length < info1Byte || size == 2 {
			if end := size + length; end <= len(buf) && ascii(buf[size:end]) {
				c.pos += end
				return buf[size:end], true, nil
			}
		}
	}
	return c.keySlow()
}

func (c *Cursor) keySlow() (key []byte, isText bool, err error) {
	start := c.pos
	major, argument, err := c.head()
	if err != nil {
		return nil, false, err
	}
	switch major {
	case majorString:
		if argument > uint64(c.Len()) {
			c.pos = start
			return nil, false, c.fail(ErrTruncated, "key longer than the input")
		}
		end := c.pos + int(argument)
		key = c.buf[c.pos:end]
		if !utf8.Valid(key) {
			c.pos = start
			return nil, false, c.fail(ErrSyntax, "key is not valid UTF-8")
		}
		c.pos = end
		return key, true, nil
	case MajorUint, majorNegInt:
		if argument > math.MaxInt64 {
			c.pos = start
			return nil, false, c.fail(ErrType, "integer key does not fit int64")
		}
		return nil, false, nil
	default:
		c.pos = start
		return nil, false, c.mismatch("a text or integer key", c.buf[start])
	}
}

// ascii reports whether s is all ASCII, eight bytes at a time.
func ascii(s []byte) bool {
	var bits uint64
	for ; len(s) >= 8; s = s[8:] {
		bits |= binary.LittleEndian.Uint64(s)
	}
	for _, c := range s {
		bits |= uint64(c)
	}
	return bits&0x8080808080808080 == 0
}
