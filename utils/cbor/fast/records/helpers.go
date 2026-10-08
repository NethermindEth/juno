package records

import (
	"reflect"
	"strings"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

// readFelt reads a felt. Null leaves the pointer nil.
func readFelt(r *fast.Reader, dst **felt.Felt) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value := new(felt.Felt)
	if err := r.Limbs((*[fast.Limbs]uint64)(value)); err != nil {
		return err
	}
	*dst = value
	return nil
}

// readFeltSlice reads an array of felts, or null as nil.
func readFeltSlice(r *fast.Reader, dst *felt.Slice[felt.Felt]) error {
	value, err := fast.LimbSlice[felt.Felt](r)
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

func readUint64(r *fast.Reader, dst *uint64) error {
	if r.Null() {
		return nil // null leaves a plain field as it was
	}
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

func readUint32[T ~uint32](r *fast.Reader, dst *T) error {
	if r.Null() {
		return nil // null leaves a plain field as it was
	}
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	if value > 1<<32-1 {
		return fast.ErrType
	}
	*dst = T(value)
	return nil
}

func readByte(r *fast.Reader, dst *byte) error {
	if r.Null() {
		return nil // null leaves a plain field as it was
	}
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	if value > 255 {
		return fast.ErrType
	}
	*dst = byte(value)
	return nil
}

func readBool(r *fast.Reader, dst *bool) error {
	if r.Null() {
		return nil // null leaves a plain field as it was
	}
	value, err := r.Bool()
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

func readString(r *fast.Reader, dst *string) error {
	if r.Null() {
		return nil // null leaves a plain field as it was
	}
	value, err := r.String()
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

// readRaw reads a byte string into a byte slice, or null as nil.
func readRaw(r *fast.Reader, dst *[]byte) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value, err := r.Bytes()
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

// readPtr reads a struct behind a pointer. Null leaves the pointer nil.
func readPtr[T any](r *fast.Reader, dst **T, read func(*fast.Reader, *T) error) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value := new(T)
	if err := read(r, value); err != nil {
		return err
	}
	*dst = value
	return nil
}

// readSlice reads an array of structs, or null as nil.
func readSlice[T any](r *fast.Reader, dst *[]T, read func(*fast.Reader, *T) error) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]T, count)
	for i := range out {
		if err := read(r, &out[i]); err != nil {
			return err
		}
	}
	r.Leave()
	*dst = out
	return nil
}

// readPtrSlice reads an array of struct pointers, or null as nil. An element may be null.
func readPtrSlice[T any](r *fast.Reader, dst *[]*T, read func(*fast.Reader, *T) error) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]*T, count)
	for i := range out {
		if err := readPtr(r, &out[i], read); err != nil {
			return err
		}
	}
	r.Leave()
	*dst = out
	return nil
}

func readStringSlice(r *fast.Reader, dst *[]string) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]string, count)
	for i := range out {
		if err := readString(r, &out[i]); err != nil {
			return err
		}
	}
	r.Leave()
	*dst = out
	return nil
}

// readFields walks a struct's map, giving field the name of each entry to read. field reports
// whether it knows the name; an unknown name is retried case-insensitively, which is how
// fxamacker matches keys, and skipped when it is still unknown. Null leaves the struct as it is.
func readFields(r *fast.Reader, names folder, field func(key string) (known bool, err error)) error {
	if r.Null() {
		return nil
	}
	count, err := r.MapLen()
	if err != nil {
		return err
	}
	for ; count > 0; count-- {
		raw, isText, err := r.Key()
		if err != nil {
			return err
		}
		if !isText {
			if err := r.Skip(); err != nil {
				return err
			}
			continue
		}
		// The name is only compared inside field, so it can borrow the input.
		key := unsafe.String(unsafe.SliceData(raw), len(raw))
		known, err := field(key)
		if err != nil {
			return err
		}
		if !known {
			if canonical, ok := names.canonical(raw); ok {
				known, err = field(canonical)
				if err != nil {
					return err
				}
			}
		}
		if !known {
			if err := r.Skip(); err != nil {
				return err
			}
		}
	}
	r.Leave()
	return nil
}

// folder maps the lower-cased wire name of each field of a struct to its wire name.
type folder map[string]string

// maxFieldName is the longest wire name a folder will match; a longer key cannot be one.
const maxFieldName = 64

// newFolder lists the wire names of the fields of T as fxamacker does: the field name, or the
// name in its cbor tag, with embedded structs promoted and "-" fields left out.
func newFolder[T any]() folder {
	out := folder{}
	addFields(out, reflect.TypeFor[T]())
	return out
}

func addFields(out folder, typ reflect.Type) {
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("cbor"), ",")
		switch {
		case name == "-":
			continue
		case field.Anonymous && name == "" && field.Type.Kind() == reflect.Struct:
			addFields(out, field.Type)
			continue
		case !field.IsExported():
			continue
		case name == "":
			name = field.Name
		}
		out[strings.ToLower(name)] = name
	}
}

// canonical returns the wire name that key matches ignoring case.
func (f folder) canonical(key []byte) (string, bool) {
	if len(key) > maxFieldName {
		return "", false
	}
	var buf [maxFieldName]byte
	for i, c := range key {
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf[i] = c
	}
	name, ok := f[string(buf[:len(key)])]
	return name, ok
}

// readLimbPtr reads a felt-shaped value behind a pointer. Null leaves the pointer nil.
func readLimbPtr[F ~[fast.Limbs]uint64](r *fast.Reader, dst **F) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value := new(F)
	if err := fast.Felt(r, value); err != nil {
		return err
	}
	*dst = value
	return nil
}

// readFeltPtrSlice reads an array of felt pointers, or null as nil. An element may be null.
func readFeltPtrSlice(r *fast.Reader, dst *[]*felt.Felt) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]*felt.Felt, count)
	for i := range out {
		if err := readFelt(r, &out[i]); err != nil {
			return err
		}
	}
	r.Leave()
	*dst = out
	return nil
}

// readMap reads a map whose keys are felt-shaped, or null as nil. Each value is read by readValue.
func readMap[K ~[fast.Limbs]uint64, V any](
	r *fast.Reader,
	dst *map[K]V,
	readValue func(*fast.Reader, *V) error,
) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.MapLen()
	if err != nil {
		return err
	}
	out := make(map[K]V, count)
	for ; count > 0; count-- {
		var key K
		if err := fast.Felt(r, &key); err != nil {
			return err
		}
		var value V
		if err := readValue(r, &value); err != nil {
			return err
		}
		out[key] = value
	}
	r.Leave()
	*dst = out
	return nil
}

// once marks field number bit as read, and reports whether it already was. A map with the same
// key twice is something fxamacker has its own rule for (the first value wins), so the reader
// leaves it to fxamacker.
func once(seen *uint64, bit uint) bool {
	mask := uint64(1) << bit
	repeated := *seen&mask != 0
	*seen |= mask
	return repeated
}
