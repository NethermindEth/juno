package cursor

import (
	"errors"
	"unsafe"

	"github.com/NethermindEth/juno/internal/feltcbor"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// Limbs is the number of words in a felt.
const Limbs = fp.Limbs

// Limbs reads a legacy array of Montgomery limbs or a compact felt. dst is written only on success.
func (c *Cursor) Limbs(dst *[Limbs]uint64) error {
	buf := c.buf[c.pos:]
	// Read the common full-width array directly, as in the bulk reader below.
	if c.depth < maxNestedLevels && feltcbor.IsWide(buf) {
		feltcbor.ReadWide(buf, (*fp.Element)(dst))
		c.pos += feltcbor.WideLen
		return nil
	}
	// Legacy arrays consume one nesting level. Compact integers and byte strings consume none.
	if c.depth < maxNestedLevels || (len(buf) > 0 && buf[0]&majorMask != majorArray) {
		n, err := feltcbor.Decode(buf, (*fp.Element)(dst))
		if err == nil {
			c.pos += n
			return nil
		}
		if err != feltcbor.ErrLegacy {
			kind := ErrUnsupported
			if errors.Is(err, feltcbor.ErrTruncated) {
				kind = ErrTruncated
			}
			return c.fail(kind, "%v", err)
		}
	}
	return c.legacyLimbs(dst)
}

// legacyLimbs handles other CBOR arrays under the cursor's nesting limit.
func (c *Cursor) legacyLimbs(dst *[Limbs]uint64) error {
	if c.valid() {
		if head := c.buf[c.pos]; head&majorMask != majorArray {
			return c.mismatch("a felt", head)
		}
	}
	// A felt is an array, so the items past the fourth, which Skip passes, sit one level deeper. They
	// are read with a copy, which c follows only if the read succeeds.
	inner := *c
	count, err := inner.expect(majorArray)
	if err != nil {
		return err
	}
	if err := inner.enter(c.pos); err != nil {
		return err
	}
	// As with a Go array, a shorter felt leaves the rest zero and a longer one is cut off.
	var limbs [Limbs]uint64
	for i := range count {
		if i >= Limbs {
			err = inner.Skip()
		} else {
			limbs[i], err = inner.Uint64()
		}
		if err != nil {
			return err
		}
	}
	c.pos = inner.pos
	*dst = limbs
	return nil
}

// LimbSlice reads an array of felts, or null as a nil slice. F is any type with a felt's layout.
func LimbSlice[F ~[Limbs]uint64](c *Cursor) ([]F, error) {
	if c.Null() {
		return nil, nil
	}
	start := c.pos
	// The slice is read with a copy, which c follows only if the read succeeds.
	inner := *c
	count, err := inner.expect(majorArray)
	if err != nil {
		return nil, err
	}
	if left := inner.Len(); count > uint64(left) {
		// A compact felt takes at least one byte, so the input must back the allocation.
		return nil, c.fail(
			ErrUnsupported, "count %d does not fit the %d bytes left at one byte per felt", count, left,
		)
	}
	// The slice is a nesting level. Legacy felt arrays add another.
	if err := inner.enter(start); err != nil {
		return nil, err
	}
	out := make([]F, count)
	// F has the layout of [Limbs]uint64, so the slice can be read as one.
	felts := unsafe.Slice((*[Limbs]uint64)(unsafe.Pointer(unsafe.SliceData(out))), len(out))
	if err := inner.readLimbs(felts); err != nil {
		return nil, err
	}
	c.pos = inner.pos
	return out, nil
}

// readLimbs fills felts from consecutive felts in the input. It may write felts before failing.
func (c *Cursor) readLimbs(felts [][Limbs]uint64) error {
	buf := c.buf[c.pos:]
	for i := range felts {
		if c.depth < maxNestedLevels && feltcbor.IsWide(buf) {
			feltcbor.ReadWide(buf, (*fp.Element)(&felts[i]))
			buf = buf[feltcbor.WideLen:]
			continue
		}
		c.pos = len(c.buf) - len(buf)
		if err := c.Limbs(&felts[i]); err != nil {
			return err
		}
		buf = c.buf[c.pos:]
	}
	c.pos = len(c.buf) - len(buf)
	return nil
}

// Felt reads a felt into any type with a felt's layout.
func Felt[F ~[Limbs]uint64](c *Cursor, dst *F) error {
	return c.Limbs((*[Limbs]uint64)(unsafe.Pointer(dst)))
}
