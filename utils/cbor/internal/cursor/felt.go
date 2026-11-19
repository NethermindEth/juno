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

// limbHead is the head of a limb in the eight-byte form.
const limbHead = MajorUint | info8Byte

// wideFelt reports whether buf starts with a felt whose four limbs all take the eight-byte form.
func wideFelt(buf []byte) bool {
	return len(buf) >= feltLen && buf[0] == feltHead &&
		buf[1] == limbHead && buf[10] == limbHead && buf[19] == limbHead && buf[28] == limbHead
}

// Limbs reads a legacy array of Montgomery limbs or a compact felt. dst is written only on success.
func (c *Cursor) Limbs(dst *[Limbs]uint64) error {
	// A felt is an array, a level of its own. The uint64 form and shortLimbs below do not count it.
	if c.depth >= maxNestedLevels {
		return c.limbsSlow(dst)
	}
	buf := c.buf[c.pos:]
	if wideFelt(buf) {
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
	if c.valid() {
		if major := c.buf[c.pos] & majorMask; major == MajorUint || major == majorBytes {
			return c.compactLimbs(dst)
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
		if c.depth < maxNestedLevels && wideFelt(buf) {
			felt := &felts[i]
			felt[0] = binary.BigEndian.Uint64(buf[2:])
			felt[1] = binary.BigEndian.Uint64(buf[11:])
			felt[2] = binary.BigEndian.Uint64(buf[20:])
			felt[3] = binary.BigEndian.Uint64(buf[29:])
			buf = buf[feltLen:]
			continue
		}
		c.pos = len(c.buf) - len(buf)
		var err error
		if len(buf) > 0 && (buf[0]&majorMask == MajorUint || buf[0]&majorMask == majorBytes) {
			// Limbs would hand a compact felt to the same reader.
			err = c.compactLimbs(&felts[i])
		} else {
			err = c.Limbs(&felts[i])
		}
		if err != nil {
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
