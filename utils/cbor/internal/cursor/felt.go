package cursor

import (
	"encoding/binary"
	"unsafe"
)

// Limbs is the number of 64-bit words in a felt.
const Limbs = 4

const (
	// feltHead is the head of an array of four items.
	feltHead = majorArray | Limbs

	// feltLen is the length of the all-uint64 form: an array head and four 0x1b heads with 8 byte
	// payloads. It is what Juno writes in practice.
	feltLen = 1 + Limbs*(1+8)
)

// Limbs reads a felt: an array of four unsigned integers, the words of its Montgomery form, each as
// short as its value allows. The all-uint64 form takes a single bounds check and four loads; every
// other width goes through the slow path. dst is written only on success.
func (c *Cursor) Limbs(dst *[Limbs]uint64) error {
	buf := c.buf[c.pos:]
	// The test for the all-uint64 form is written out here, in readLimbs and in Skip on purpose:
	// a helper for it, even an inlined one, measured slower on BenchmarkReadFeltArray.
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

// shortLimbs reads a felt of four unsigned integers of any width from the start of buf and
// returns the bytes it took, or 0, leaving dst alone, for anything else.
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
	// The felt is an array, so what it holds, which a longer one skips, sits one level deeper.
	if err := c.enter(); err != nil {
		c.pos = start
		return err
	}
	// As with a Go array, a shorter array leaves the rest zero and a longer one is cut off. Juno
	// never writes either.
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

// LimbSlice reads an array of felts, or null as a nil slice. F is any type with the layout
// of a felt. The slice is sized from the element count, which the input caps.
func LimbSlice[F ~[Limbs]uint64](c *Cursor) ([]F, error) {
	if c.Null() {
		return nil, nil
	}
	start := c.pos
	count, err := c.expect(majorArray)
	if err != nil {
		return nil, err
	}
	if count > uint64(c.Len())/(1+Limbs) {
		// A felt takes at least five bytes, so a larger count is damaged or an unusual form, and
		// not worth an allocation the input cannot back.
		c.pos = start
		return nil, ErrUnsupported
	}
	// The slice is an array, one nesting level to fxamacker, and each felt in it is another.
	if err := c.enter(); err != nil {
		c.pos = start
		return nil, err
	}
	out := make([]F, count)
	// F has the layout of [Limbs]uint64, so the slice can be read as one.
	felts := unsafe.Slice((*[Limbs]uint64)(unsafe.Pointer(unsafe.SliceData(out))), len(out))
	err = c.readLimbs(felts)
	c.depth--
	if err != nil {
		c.pos = start
		return nil, err
	}
	return out, nil
}

// readLimbs fills felts from consecutive felts in the input. The all-uint64 form is read in the
// loop itself; any other goes through Limbs. It may write felts before failing.
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

// Felt reads a felt into any type with the layout of one.
func Felt[F ~[Limbs]uint64](c *Cursor, dst *F) error {
	return c.Limbs((*[Limbs]uint64)(unsafe.Pointer(dst)))
}
