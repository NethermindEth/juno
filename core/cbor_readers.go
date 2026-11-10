package core

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
)

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
