package records

import (
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"strings"

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

// readUint reads an unsigned integer into any type whose underlying type is uint.
func readUint[T ~uint](r *fast.Reader, dst *T) error {
	if r.Null() {
		return nil // null leaves a plain field as it was
	}
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	if uint64(T(value)) != value {
		return fast.ErrType // wider than uint
	}
	*dst = T(value)
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
	if value > math.MaxUint8 {
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

// minPtrSliceElement is fewer bytes than any struct readPtrSlice reads takes when stored with
// its fields.
const minPtrSliceElement = 32

// readPtrSlice reads an array of struct pointers, or null as nil. An element may be null. The
// elements share one array, as long as the bytes left could hold them: an array of nulls or
// empty maps gets no more memory than fxamacker would give it, give or take a small factor.
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
	values := make([]T, min(count, r.Len()/minPtrSliceElement))
	for i := range out {
		if r.Null() {
			continue
		}
		var value *T
		if len(values) > 0 {
			value, values = &values[0], values[1:]
		} else {
			value = new(T)
		}
		if err := read(r, value); err != nil {
			return err
		}
		out[i] = value
	}
	r.Leave()
	*dst = out
	return nil
}

// fieldsOf returns the Fields of T, whose field number i has the wire name names[i]. It panics
// unless the names are the wire names fxamacker gives the fields of T, so a reader cannot leave
// a field of T unread.
func fieldsOf[T any](names []string) *fast.Fields {
	want := map[string]bool{}
	addFields(want, reflect.TypeFor[T]())
	for _, name := range names {
		if !want[name] {
			panic("records: " + reflect.TypeFor[T]().String() + " has no field " + name)
		}
		delete(want, name)
	}
	if len(want) > 0 {
		panic(fmt.Sprintf("records: no field numbers for %v fields %v", reflect.TypeFor[T](), slices.Sorted(maps.Keys(want))))
	}
	return fast.NewFields(names...)
}

// addFields lists the wire names of the fields of typ as fxamacker does: the field name, or the
// name in its cbor tag, with embedded structs promoted and "-" fields left out.
func addFields(out map[string]bool, typ reflect.Type) {
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
		out[name] = true
	}
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
	// The felts share one array the count sizes, rather than one allocation each.
	values := make([]felt.Felt, count)
	for i := range out {
		if r.Null() {
			continue
		}
		if err := r.Limbs((*[fast.Limbs]uint64)(&values[i])); err != nil {
			return err
		}
		out[i] = &values[i]
	}
	r.Leave()
	*dst = out
	return nil
}

// readFeltMap reads a map from felt to felt, or null as nil. Keys and values are read in
// place, with no generic function value in the way, so nothing but the map and one array
// holding its values is allocated.
//
//nolint:gocritic // dst is given the map made here when there is none
func readFeltMap(r *fast.Reader, dst *map[felt.Felt]*felt.Felt) error {
	var spare []felt.Felt
	return readFeltMapFrom(r, dst, &spare, 0)
}

// readFeltMapFrom is readFeltMap taking the values from spare. When spare runs out it is
// refilled with room for the values left in the map and extra more.
//
//nolint:gocritic // dst is given the map made here when there is none
func readFeltMapFrom(r *fast.Reader, dst *map[felt.Felt]*felt.Felt, spare *[]felt.Felt, extra int) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.MapLen()
	if err != nil {
		return err
	}
	out := make(map[felt.Felt]*felt.Felt, count)
	for left := count; left > 0; left-- {
		var key felt.Felt
		if err := r.Limbs((*[fast.Limbs]uint64)(&key)); err != nil {
			return err
		}
		if r.Null() {
			out[key] = nil
			continue
		}
		value := nextFelt(spare, left+extra)
		if err := r.Limbs((*[fast.Limbs]uint64)(value)); err != nil {
			return err
		}
		out[key] = value
	}
	r.Leave()
	*dst = out
	return nil
}

// nextFelt returns a zero felt from the front of spare. An empty spare is refilled with an
// array of at least want felts, rounded up to what its allocation holds anyway, so the
// rounding serves later felts rather than going to waste.
func nextFelt(spare *[]felt.Felt, want int) *felt.Felt {
	if len(*spare) == 0 {
		grown := slices.Grow([]felt.Felt(nil), want)
		*spare = grown[:cap(grown)]
	}
	value := &(*spare)[0]
	*spare = (*spare)[1:]
	return value
}

// readStorageDiffs reads a map from contract address to its map of storage updates.
//
//nolint:gocritic // dst is given the map made here when there is none
func readStorageDiffs(r *fast.Reader, dst *map[felt.Felt]map[felt.Felt]*felt.Felt) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.MapLen()
	if err != nil {
		return err
	}
	out := make(map[felt.Felt]map[felt.Felt]*felt.Felt, count)
	// The inner maps take their values from shared arrays, each sized for at least one
	// value per map still to come.
	var spare []felt.Felt
	for ; count > 0; count-- {
		var key felt.Felt
		if err := r.Limbs((*[fast.Limbs]uint64)(&key)); err != nil {
			return err
		}
		var value map[felt.Felt]*felt.Felt
		if err := readFeltMapFrom(r, &value, &spare, count-1); err != nil {
			return err
		}
		out[key] = value
	}
	r.Leave()
	*dst = out
	return nil
}
