package fast

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

// Fields matches the keys of a struct's map to field numbers the way fxamacker matches them to
// fields: an exact name first, then a name that differs only in case.
type Fields struct {
	// order holds each key as the encoder writes it, head and name, in the order a canonical
	// encoder sorts them: shorter first, then bytewise.
	order []fieldKey
	// names holds the wire name of each field, by field number.
	names []string
	// folded maps the lower-cased wire name of each field to its number.
	folded map[string]int
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
	out := &Fields{
		order:  make([]fieldKey, len(names)),
		names:  names,
		folded: make(map[string]int, len(names)),
	}
	for i, name := range names {
		var head string
		switch {
		case len(name) < info1Byte:
			head = string([]byte{MajorString | byte(len(name))})
		case len(name) < 1<<8:
			head = string([]byte{MajorString | info1Byte, byte(len(name))})
		default:
			panic("fast: field name too long: " + name)
		}
		for _, c := range []byte(name) {
			if c >= utf8.RuneSelf {
				panic("fast: field name is not ASCII: " + name)
			}
		}
		lower := strings.ToLower(name)
		if _, ok := out.folded[lower]; ok {
			panic("fast: field names repeat ignoring case: " + name)
		}
		out.folded[lower] = i
		out.order[i] = newFieldKey(head+name, i)
	}
	// The head of an encoded name holds its length, so comparing the bytes puts shorter names first,
	// as a canonical encoder does.
	slices.SortFunc(out.order, func(a, b fieldKey) int { return strings.Compare(a.encoded, b.encoded) })
	return out
}

// Order returns the wire names in the order a canonical encoder writes them. Only tests use it,
// to check a field list against what the encoder writes; the records package's tests are outside
// this package, so it is exported.
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

// OpenStruct reads the head of a struct's map. Null reads as a struct with no pairs, since
// fxamacker leaves the struct as it was.
func (r *Reader) OpenStruct(fields *Fields) (Struct, error) {
	if r.Null() {
		return Struct{}, nil
	}
	count, err := r.MapLen()
	if err != nil {
		return Struct{}, err
	}
	return Struct{fields: fields, left: count, open: true}, nil
}

// NextField reads the next key of the struct's map and returns the number of the field it names,
// Unknown, or Done once no pairs are left. On an error it returns Done and the error.
//
// Stored maps are canonical, so the key is first compared with the fields after the last one
// matched, as encoded, which needs no UTF-8 check: a key equal to an ASCII name is valid. Any
// other key is read and matched like fxamacker. A field named twice is ErrUnsupported, since
// fxamacker has its own rule for a repeated key.
func (r *Reader) NextField(s *Struct) (int, error) {
	// The common case first: the key is the one expected next, and its field is not yet read.
	if s.left > 0 && s.next < len(s.fields.order) && r.pos+keyWords*8 <= len(r.buf) {
		key := &s.fields.order[s.next]
		buf := r.buf[r.pos : r.pos+keyWords*8]
		if (binary.LittleEndian.Uint64(buf)^key.words[0])&key.masks[0]|
			(binary.LittleEndian.Uint64(buf[8:])^key.words[1])&key.masks[1]|
			(binary.LittleEndian.Uint64(buf[16:])^key.words[2])&key.masks[2]|
			s.seen&key.bit == 0 && key.short {
			r.pos += len(key.encoded)
			s.left--
			s.next++
			s.seen |= key.bit
			return key.field, nil
		}
	}
	return r.nextField(s)
}

// nextField looks for the key among all the fields after the last one matched, then like fxamacker.
func (r *Reader) nextField(s *Struct) (int, error) {
	if s.left == 0 {
		if s.open {
			s.open = false
			r.Leave()
		}
		return Done, nil
	}
	s.left--
	if i := s.find(r.buf[r.pos:]); i >= 0 {
		key := &s.fields.order[i]
		r.pos += len(key.encoded)
		s.next = i + 1
		return s.mark(key.field)
	}
	return r.otherField(s)
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
			if (w0^key.words[0])&key.masks[0]|(w1^key.words[1])&key.masks[1]|(w2^key.words[2])&key.masks[2] == 0 &&
				(key.short || len(buf) >= len(key.encoded) && string(buf[keyWords*8:len(key.encoded)]) == key.encoded[keyWords*8:]) {
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
func (r *Reader) otherField(s *Struct) (int, error) {
	key, isText, err := r.Key()
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

// match finds the field a key names exactly or, failing that, ignoring case. fxamacker folds
// case with strings.EqualFold but only between strings of the same byte length, and every rune
// that folds to an ASCII letter outside ASCII takes more than one byte, so ASCII folding is the same.
func (f *Fields) match(key []byte) (int, bool) {
	for i, name := range f.names {
		if string(key) == name {
			return i, true
		}
	}
	var buf [1 << 8]byte
	if len(key) > len(buf) {
		return 0, false
	}
	for i, c := range key {
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf[i] = c
	}
	field, ok := f.folded[string(buf[:len(key)])]
	return field, ok
}

// Key reads a map key the way fxamacker reads the keys of a struct. A text key is returned
// as a sub-slice of the input, valid only while the input is, after a UTF-8 check. An
// integer key can match no field, so it is consumed and reported as not text; one too large
// for int64 is an error, as is a key of any other type.
func (r *Reader) Key() (key []byte, isText bool, err error) {
	// A short text key with its length in the head or the byte after it.
	buf := r.buf[r.pos:]
	if len(buf) > 0 {
		length, size := int(buf[0]-MajorString), 1
		if length == info1Byte && len(buf) > 1 {
			length, size = int(buf[1]), 2
		}
		if length < info1Byte || size == 2 {
			if end := size + length; end <= len(buf) && ascii(buf[size:end]) {
				r.pos += end
				return buf[size:end], true, nil
			}
		}
	}
	return r.keySlow()
}

func (r *Reader) keySlow() (key []byte, isText bool, err error) {
	start := r.pos
	major, argument, err := r.head()
	if err != nil {
		return nil, false, err
	}
	switch major {
	case MajorString:
		if argument > uint64(r.Len()) {
			r.pos = start
			return nil, false, ErrTruncated
		}
		end := r.pos + int(argument)
		key = r.buf[r.pos:end]
		if !utf8.Valid(key) {
			r.pos = start
			return nil, false, ErrSyntax
		}
		r.pos = end
		return key, true, nil
	case MajorUint, MajorNegInt:
		if argument > math.MaxInt64 {
			r.pos = start
			return nil, false, ErrType
		}
		return nil, false, nil
	default:
		r.pos = start
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
