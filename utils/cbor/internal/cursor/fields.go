package cursor

import (
	"encoding/binary"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

// Results of NextField that are not field numbers.
const (
	// Done means the map of the struct has no pairs left. It is closed.
	Done = -1
	// Unknown means the key names no field. Its value is next, for the caller to skip.
	Unknown = -2
)

// maxFields is how many fields a Struct can track; the repeated-key check is one bit per field.
const maxFields = 64

// Fields matches the keys of a struct's map to field numbers: an exact name first, then a name
// that differs only in case.
type Fields struct {
	// order holds each key as the encoder writes it, head and name, in the order a canonical
	// encoder sorts them: shorter first, then bytewise.
	order []fieldKey
	// names holds the wire name of each field, by field number.
	names []string
	// all has the bit of every field set.
	all uint64
}

// keyWords is how many 8 byte words of a key are compared as words.
const keyWords = 3

type fieldKey struct {
	// words holds the first bytes of encoded, little-endian and padded with zeros, and masks
	// selects the bytes that belong to it.
	words, masks [keyWords]uint64
	encoded      string
	field        int
	// bit is the field's bit in Struct.seen, and short whether words holds all of encoded.
	bit   uint64
	short bool
}

func newFieldKey(encoded string, field int) fieldKey {
	out := fieldKey{encoded: encoded, field: field, bit: 1 << field, short: len(encoded) <= keyWords*8}
	for i := 0; i < len(encoded) && i < keyWords*8; i++ {
		out.words[i/8] |= uint64(encoded[i]) << (i % 8 * 8)
		out.masks[i/8] |= 0xff << (i % 8 * 8)
	}
	return out
}

// NewFields returns the Fields of a struct whose field number i has the wire name names[i].
// The names must be distinct ignoring case, ASCII, and shorter than 256 bytes.
func NewFields(names ...string) *Fields {
	if len(names) > maxFields {
		panic("fast: too many fields")
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
			panic("fast: field name too long: " + name)
		}
		for _, c := range []byte(name) {
			if c >= utf8.RuneSelf {
				panic("fast: field name is not ASCII: " + name)
			}
		}
		lower := strings.ToLower(name)
		if folded[lower] {
			panic("fast: field names repeat ignoring case: " + name)
		}
		folded[lower] = true
		out.order[i] = newFieldKey(head+name, i)
	}
	// The head of an encoded name holds its length, so comparing the bytes puts shorter names first,
	// as a canonical encoder does.
	slices.SortFunc(out.order, func(a, b fieldKey) int {
		return strings.Compare(a.encoded, b.encoded)
	})
	return out
}

// Order returns the wire names in canonical order. It is exported for the tests.
func (f *Fields) Order() []string {
	out := make([]string, len(f.order))
	for i, key := range f.order {
		out[i] = f.names[key.field]
	}
	return out
}

// Struct is the progress of reading one struct's map.
type Struct struct {
	fields *Fields
	// left is the number of pairs not yet read.
	left int
	// next is where in fields.order the next key is expected.
	next int
	// seen has a bit set for each field read.
	seen uint64
	// open is whether a map was opened, which Done closes; a null struct opens none.
	open bool
}

// OpenStruct reads the head of a struct's map. Null reads as a struct with no pairs: the struct
// is left as it was.
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

// NextField reads the next key of the struct's map and returns the number of the field it names,
// Unknown, or Done once no pairs are left. On an error it returns Done and the error.
//
// Stored maps are canonical, so the key is first compared as encoded with the fields after the
// last one matched, which needs no UTF-8 check. Any other key is matched by name, then ignoring
// case. A field named twice is ErrUnsupported.
func (c *Cursor) NextField(s *Struct) (int, error) {
	// The common case first: the key is the one expected next, and its field is not yet read.
	if s.left > 0 && s.next < len(s.fields.order) && c.pos+keyWords*8 <= len(c.buf) {
		key := &s.fields.order[s.next]
		buf := c.buf[c.pos : c.pos+keyWords*8]
		if (binary.LittleEndian.Uint64(buf)^key.words[0])&key.masks[0]|
			(binary.LittleEndian.Uint64(buf[8:])^key.words[1])&key.masks[1]|
			(binary.LittleEndian.Uint64(buf[16:])^key.words[2])&key.masks[2]|
			s.seen&key.bit == 0 && key.short {
			c.pos += len(key.encoded)
			s.left--
			s.next++
			s.seen |= key.bit
			return key.field, nil
		}
	}
	return c.nextField(s)
}

// NextWanted is NextField for a struct read in part, whose fields are only the ones wanted. It
// skips the pairs of every other key, checking their syntax, and returns the number of the next
// field named, or Done at the end of the map. It does not expect canonical order, and once every
// field is read it stops matching keys. As fxamacker does, the first of two keys naming a field
// wins: the second is skipped.
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

// nextField looks for the key among all the fields after the last one matched, then by name,
// as match does.
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
		c.pos += len(key.encoded)
		s.next = i + 1
		return s.mark(key.field)
	}
	return c.otherField(s)
}

// find returns where the key at the start of buf is in the order of the fields, looking only
// after the last one matched, or -1.
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
			if keyHead := byte(key.words[0]); keyHead != head {
				if keyHead > head {
					break // every key from here on sorts after this one
				}
				continue
			}
			if (w0^key.words[0])&key.masks[0]|
				(w1^key.words[1])&key.masks[1]|
				(w2^key.words[2])&key.masks[2] == 0 &&
				(key.short || len(buf) >= len(key.encoded) &&
					string(buf[keyWords*8:len(key.encoded)]) == key.encoded[keyWords*8:]) {
				return i
			}
		}
		return -1
	}
	for i := s.next; i < len(order) && len(buf) > 0; i++ {
		key := order[i].encoded
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
	return s.mark(field)
}

func (s *Struct) mark(field int) (int, error) {
	bit := uint64(1) << field
	if s.seen&bit != 0 {
		return Done, ErrUnsupported
	}
	s.seen |= bit
	return field, nil
}

// match finds the field a key names exactly or, failing that, ignoring case. Folding ASCII letters
// alone gives the result of strings.EqualFold, because a rune outside ASCII that folds to an ASCII
// letter takes more than one byte. Names are distinct ignoring case, so at most one matches. Most
// keys here name no field, so neither pass allocates or copies.
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
	for i := range len(name) {
		a, b := key[i], name[i]
		if 'A' <= a && a <= 'Z' {
			a += 'a' - 'A'
		}
		if 'A' <= b && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

// Key reads a map key as the name of a struct field. A text key is checked as UTF-8 and returned
// as a sub-slice of the input. An integer key can match no field, so it is consumed and reported
// as not text. An integer too large for int64, or a key of any other type, is an error.
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
			return nil, false, ErrTruncated
		}
		end := c.pos + int(argument)
		key = c.buf[c.pos:end]
		if !utf8.Valid(key) {
			c.pos = start
			return nil, false, ErrSyntax
		}
		c.pos = end
		return key, true, nil
	case MajorUint, majorNegInt:
		if argument > math.MaxInt64 {
			c.pos = start
			return nil, false, ErrType
		}
		return nil, false, nil
	default:
		c.pos = start
		return nil, false, ErrType
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
