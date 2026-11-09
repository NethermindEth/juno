package fastcbor

import (
	"math"
	"slices"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
)

// readFelt reads a felt. Null leaves the pointer nil.
func readFelt(r *cursor.Cursor, dst **felt.Felt) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value := new(felt.Felt)
	if err := r.Limbs((*[cursor.Limbs]uint64)(value)); err != nil {
		return err
	}
	*dst = value
	return nil
}

// readFeltSlice reads an array of felts, or null as nil.
func readFeltSlice(r *cursor.Cursor, dst *felt.Slice[felt.Felt]) error {
	value, err := cursor.LimbSlice[felt.Felt](r)
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

func readUint64(r *cursor.Cursor, dst *uint64) error {
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
func readUint[T ~uint](r *cursor.Cursor, dst *T) error {
	if r.Null() {
		return nil // null leaves a plain field as it was
	}
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	if uint64(T(value)) != value {
		return cursor.ErrType // wider than uint
	}
	*dst = T(value)
	return nil
}

func readUint32[T ~uint32](r *cursor.Cursor, dst *T) error {
	if r.Null() {
		return nil // null leaves a plain field as it was
	}
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	if value > 1<<32-1 {
		return cursor.ErrType
	}
	*dst = T(value)
	return nil
}

func readUint16(r *cursor.Cursor, dst *uint16) error {
	if r.Null() {
		return nil // null leaves a plain field as it was
	}
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	if value > math.MaxUint16 {
		return cursor.ErrType
	}
	*dst = uint16(value)
	return nil
}

func readByte(r *cursor.Cursor, dst *byte) error {
	if r.Null() {
		return nil // null leaves a plain field as it was
	}
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	if value > math.MaxUint8 {
		return cursor.ErrType
	}
	*dst = byte(value)
	return nil
}

func readBool(r *cursor.Cursor, dst *bool) error {
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

func readString(r *cursor.Cursor, dst *string) error {
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
func readRaw(r *cursor.Cursor, dst *[]byte) error {
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

// minPtrSliceElement is fewer bytes than any struct readPtrSliceOf reads takes.
const minPtrSliceElement = 32

// readFeltPtrSlice reads an array of felt pointers, or null as nil. An element may be null.
func readFeltPtrSlice(r *cursor.Cursor, dst *[]*felt.Felt) error {
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
		if err := r.Limbs((*[cursor.Limbs]uint64)(&values[i])); err != nil {
			return err
		}
		out[i] = &values[i]
	}
	r.Leave()
	*dst = out
	return nil
}

// readFeltMap reads a map from felt to felt, or null as nil, with nothing allocated but the map and
// one array for its values.
//
//nolint:gocritic // dst is given the map made here when there is none
func readFeltMap(r *cursor.Cursor, dst *map[felt.Felt]*felt.Felt) error {
	var spare []felt.Felt
	return readFeltMapFrom(r, dst, &spare, 0)
}

// readFeltMapFrom is readFeltMap taking the values from spare, which it refills when it runs out.
//
//nolint:gocritic // dst is given the map made here when there is none
func readFeltMapFrom(
	r *cursor.Cursor, dst *map[felt.Felt]*felt.Felt, spare *[]felt.Felt, extra int,
) error {
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
		if err := r.Limbs((*[cursor.Limbs]uint64)(&key)); err != nil {
			return err
		}
		if r.Null() {
			out[key] = nil
			continue
		}
		value := nextFelt(spare, left+extra)
		if err := r.Limbs((*[cursor.Limbs]uint64)(value)); err != nil {
			return err
		}
		out[key] = value
	}
	r.Leave()
	*dst = out
	return nil
}

// nextFelt returns a zero felt from the front of spare, which it refills when it is empty.
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
func readStorageDiffs(r *cursor.Cursor, dst *map[felt.Felt]map[felt.Felt]*felt.Felt) error {
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
		if err := r.Limbs((*[cursor.Limbs]uint64)(&key)); err != nil {
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
