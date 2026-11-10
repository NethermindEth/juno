package core

import (
	"math"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
)

// Integer-keyed records recognize only unsigned keys. Other supported keys are
// consumed and treated as unknown; unsigned values outside int64 are rejected.
func cborReadIntegerKey(c *cursor.Cursor) (uint64, bool, error) {
	if head, err := c.Peek(); err != nil || head&0xe0 != cursor.MajorUint {
		_, _, err := c.Key()
		return 0, false, err
	}
	key, err := c.Uint64()
	if err != nil {
		return 0, false, err
	}
	if key > math.MaxInt64 {
		return 0, false, cursor.ErrType
	}
	return key, true, nil
}

// Index arrays preserve nil versus empty. Null elements remain zero. The
// unsigned first attempt matches the planner's common path and diagnostics.
func cborReadInts(c *cursor.Cursor, dst *[]int) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	count, err := c.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]int, count)
	for i := range out {
		value, err := c.Uint64()
		if err != nil {
			if c.Null() {
				continue
			}
			signed, err := c.Int64()
			if err != nil {
				return err
			}
			out[i] = int(signed)
			continue
		}
		if value > math.MaxInt64 {
			return cursor.ErrType
		}
		out[i] = int(value)
	}
	c.Leave()
	*dst = out
	return nil
}

func cborReadUint64(c *cursor.Cursor, dst *uint64) error {
	if c.Null() {
		return nil
	}
	v, err := c.Uint64()
	if err == nil {
		*dst = v
	}
	return err
}

func cborReadUint64Pointer(c *cursor.Cursor, dst **uint64) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	v, err := c.Uint64()
	if err == nil {
		*dst = &v
	}
	return err
}

// Projections accept one enclosing tag, except the time and bignum tags. This
// matches the planner's projection contract, including rejection of tag chains.
func cborReadProjectionTag(c *cursor.Cursor) error {
	if head, err := c.Peek(); err == nil && head&0xe0 == cursor.MajorTag {
		tag, err := c.Tag()
		if err != nil {
			return err
		}
		if next, err := c.Peek(); tag <= 3 || (err == nil && next&0xe0 == cursor.MajorTag) {
			return cursor.ErrUnsupported
		}
	}
	return nil
}

func cborReadMode(c *cursor.Cursor, dst *L1DAMode) error {
	if c.Null() {
		return nil
	}
	v, err := c.Uint64()
	if err == nil {
		if uint64(L1DAMode(v)) != v {
			return cursor.ErrType
		}
		*dst = L1DAMode(v)
	}
	return err
}

func cborReadString(c *cursor.Cursor, dst *string) error {
	if c.Null() {
		return nil
	}
	v, err := c.String()
	if err == nil {
		*dst = v
	}
	return err
}

func cborReadFelt(c *cursor.Cursor, dst **felt.Felt) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	v := new(felt.Felt)
	if err := c.Limbs((*[4]uint64)(v)); err != nil {
		return err
	}
	*dst = v
	return nil
}

func cborReadFeltPointers(c *cursor.Cursor, dst *[]*felt.Felt) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	count, err := c.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]*felt.Felt, count)
	values := make([]felt.Felt, count)
	for i := range out {
		if c.Null() {
			continue
		}
		if err := c.Limbs((*[4]uint64)(&values[i])); err != nil {
			return err
		}
		out[i] = &values[i]
	}
	c.Leave()
	*dst = out
	return nil
}

func cborReadSignatures(c *cursor.Cursor, dst *[][]*felt.Felt) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	count, err := c.ArrayLen()
	if err != nil {
		return err
	}
	out := make([][]*felt.Felt, count)
	for i := range out {
		if err := cborReadFeltPointers(c, &out[i]); err != nil {
			return err
		}
	}
	c.Leave()
	*dst = out
	return nil
}

func cborReadBool(c *cursor.Cursor, dst *bool) error {
	if c.Null() {
		return nil
	}
	value, err := c.Bool()
	if err == nil {
		*dst = value
	}
	return err
}

func cborReadFeeUnit(c *cursor.Cursor, dst *FeeUnit) error {
	if c.Null() {
		return nil
	}
	value, err := c.Uint64()
	if err != nil {
		return err
	}
	if value > math.MaxUint8 {
		return cursor.ErrType
	}
	*dst = FeeUnit(value)
	return nil
}

func cborReadFeltValues(c *cursor.Cursor, dst *felt.Slice[felt.Felt]) error {
	value, err := cursor.LimbSlice[felt.Felt](c)
	if err == nil {
		*dst = value
	}
	return err
}

func cborReadAddress(c *cursor.Cursor, dst *eth.Address) error {
	if c.Null() {
		return nil
	}
	return c.BytesInto(dst[:])
}

func cborReadFeltValue(c *cursor.Cursor, dst *felt.Felt) error {
	if c.Null() {
		return nil
	}
	return c.Limbs((*[4]uint64)(dst))
}

func cborReadPlainFeltValues(c *cursor.Cursor, dst *[]felt.Felt) error {
	value, err := cursor.LimbSlice[felt.Felt](c)
	if err == nil {
		*dst = value
	}
	return err
}

func cborReadTransactionVersion(c *cursor.Cursor, dst **TransactionVersion) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	value := new(TransactionVersion)
	if err := c.Limbs((*[4]uint64)(value)); err != nil {
		return err
	}
	*dst = value
	return nil
}

func cborReadDataAvailabilityMode(c *cursor.Cursor, dst *DataAvailabilityMode) error {
	if c.Null() {
		return nil
	}
	value, err := c.Uint64()
	if err != nil {
		return err
	}
	if value > math.MaxUint32 {
		return cursor.ErrType
	}
	*dst = DataAvailabilityMode(value)
	return nil
}

// Map entries replace earlier values in full, including when a later value is
// null or a partial record. A fresh zero temporary preserves the planner's slot
// clearing without runtime map-assignment hooks or reflection.
func cborReadResourceBoundsMap(c *cursor.Cursor, dst *map[Resource]ResourceBounds) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	count, err := c.MapLen()
	if err != nil {
		return err
	}
	out := make(map[Resource]ResourceBounds, count)
	for ; count > 0; count-- {
		if c.Null() {
			return cursor.ErrUnsupported
		}
		key, err := c.Uint64()
		if err != nil {
			return err
		}
		if key > math.MaxUint32 {
			return cursor.ErrType
		}
		var value ResourceBounds
		if err := cborReadResourceBounds(c, &value); err != nil {
			return err
		}
		out[Resource(key)] = value
	}
	c.Leave()
	*dst = out
	return nil
}
