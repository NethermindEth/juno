package cursor

import (
	"encoding/binary"
	"unsafe"
)

// Limbs is the number of limbs in a felt, each a 64-bit value. feltHead is the head of its array,
// and feltLen the length of the uint64 form Juno writes, which is the array head and four limbs,
// each an eight-byte integer with its own head byte.
const (
	Limbs    = 4
	feltHead = majorArray | Limbs
	feltLen  = 1 + Limbs*(1+8)
)

// Limbs reads a felt, an array of four unsigned integers that are the limbs of its Montgomery
// form. dst is written only on success.
func (c *Cursor) Limbs(dst *[Limbs]uint64) error {
	// A felt is an array, a level of its own. The uint64 form and shortLimbs below do not count it.
	if c.depth >= maxNestedLevels {
		return c.limbsSlow(dst)
	}
	buf := c.buf[c.pos:]
	// Written out here, in readLimbs, skip and skipElements, and not as a helper, because a helper
	// measured slower.
	if len(buf) >= feltLen && buf[0] == feltHead &&
		buf[1] == 0x1b && buf[10] == 0x1b && buf[19] == 0x1b && buf[28] == 0x1b {
		dst[0] = binary.BigEndian.Uint64(buf[2:])
		dst[1] = binary.BigEndian.Uint64(buf[11:])
		dst[2] = binary.BigEndian.Uint64(buf[20:])
		dst[3] = binary.BigEndian.Uint64(buf[29:])
		c.pos += feltLen
		return nil
	}
	if n := shortLimbs(buf, dst); n > 0 {
		c.pos += n
		return nil
	}
	return c.limbsSlow(dst)
}

// shortLimbs reads a felt whose limbs have any width and returns the bytes it took, or 0 for
// another form.
func shortLimbs(buf []byte, dst *[Limbs]uint64) int {
	if len(buf) == 0 || buf[0] != feltHead {
		return 0
	}
	var limbs [Limbs]uint64
	n := 1
	for i := range limbs {
		if n >= len(buf) {
			return 0
		}
		head := buf[n]
		switch {
		case head < info1Byte:
			limbs[i] = uint64(head)
			n++
		case head == info1Byte && n+2 <= len(buf):
			limbs[i] = uint64(buf[n+1])
			n += 2
		case head == info2Byte && n+3 <= len(buf):
			limbs[i] = uint64(binary.BigEndian.Uint16(buf[n+1:]))
			n += 3
		case head == info4Byte && n+5 <= len(buf):
			limbs[i] = uint64(binary.BigEndian.Uint32(buf[n+1:]))
			n += 5
		case head == info8Byte && n+9 <= len(buf):
			limbs[i] = binary.BigEndian.Uint64(buf[n+1:])
			n += 9
		default:
			return 0
		}
	}
	*dst = limbs
	return n
}

// limbsSlow reads a felt whose limbs are not all eight bytes wide.
func (c *Cursor) limbsSlow(dst *[Limbs]uint64) error {
	start := c.pos
	count, err := c.expect(majorArray)
	if err != nil {
		return err
	}
	// A felt is an array, so the items past the fourth, which Skip passes, sit one level deeper.
	if err := c.enter(start); err != nil {
		c.pos = start
		return err
	}
	// As with a Go array, a shorter felt leaves the rest zero and a longer one is cut off.
	var limbs [Limbs]uint64
	for i := range count {
		if i >= Limbs {
			err = c.Skip()
		} else {
			limbs[i], err = c.Uint64()
		}
		if err != nil {
			c.depth--
			c.pos = start
			return err
		}
	}
	c.depth--
	*dst = limbs
	return nil
}

// LimbSlice reads an array of felts, or null as a nil slice. F is any type with a felt's layout.
func LimbSlice[F ~[Limbs]uint64](c *Cursor) ([]F, error) {
	if c.Null() {
		return nil, nil
	}
	start := c.pos
	count, err := c.expect(majorArray)
	if err != nil {
		return nil, err
	}
	if left := c.Len(); count > uint64(left)/(1+Limbs) {
		// A felt Juno writes takes at least five bytes, so a larger count is damaged or uses a shorter
		// form, and is not worth an allocation the input cannot back.
		c.pos = start
		return nil, c.fail(
			ErrUnsupported, "count %d does not fit the %d bytes left at five bytes per felt", count, left,
		)
	}
	// The slice is a nesting level, and each felt in it another.
	if err := c.enter(start); err != nil {
		c.pos = start
		return nil, err
	}
	if count > 0 && c.depth >= maxNestedLevels {
		// The felts in the slice are one level deeper, which readLimbs does not count.
		c.depth--
		c.pos = start
		return nil, errDepth(start)
	}
	out := make([]F, count)
	// F has the layout of [Limbs]uint64, so the slice can be read as one.
	var felts [][Limbs]uint64
	if len(out) != 0 {
		felts = unsafe.Slice((*[Limbs]uint64)(unsafe.Pointer(&out[0])), len(out))
	}
	err = c.readLimbs(felts)
	c.depth--
	if err != nil {
		c.pos = start
		return nil, err
	}
	return out, nil
}

// readLimbs fills felts from consecutive felts in the input. It may write felts before failing.
func (c *Cursor) readLimbs(felts [][Limbs]uint64) error {
	buf := c.buf[c.pos:]
	for i := range felts {
		if len(buf) >= feltLen && buf[0] == feltHead &&
			buf[1] == 0x1b && buf[10] == 0x1b && buf[19] == 0x1b && buf[28] == 0x1b {
			felt := &felts[i]
			felt[0] = binary.BigEndian.Uint64(buf[2:])
			felt[1] = binary.BigEndian.Uint64(buf[11:])
			felt[2] = binary.BigEndian.Uint64(buf[20:])
			felt[3] = binary.BigEndian.Uint64(buf[29:])
			buf = buf[feltLen:]
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
