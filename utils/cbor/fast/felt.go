package fast

import (
	"encoding/binary"
	"unsafe"
)

// Limbs is the number of 64-bit words of a felt. Felts are stored as four unsigned
// integers in Montgomery form, written big-endian and as short as each value allows, which
// in practice is the 37 byte form: an array head and four 0x1b heads with 8 byte payloads.
const Limbs = 4

const (
	// feltHead is the head of an array of four items.
	feltHead = MajorArray | Limbs

	// feltLen is the length of the all-uint64 form.
	feltLen = 1 + Limbs*(1+8)
)

// The test for the all-uint64 form, an array head and four 0x1b heads, is written out where it
// sits in a loop (here, in Limbs, and in Skip). A function for it, even one the compiler inlines,
// measured 1 to 2% slower on BenchmarkReadFeltArray.

// Limbs reads a felt: an array of four unsigned integers. The 37 byte all-uint64 form
// takes a single bounds check and four loads; every other width goes through the slow path.
// dst is written only on success.
func (r *Reader) Limbs(dst *[Limbs]uint64) error {
	buf := r.buf[r.pos:]
	if len(buf) >= feltLen && buf[0] == feltHead &&
		buf[1] == 0x1b && buf[10] == 0x1b && buf[19] == 0x1b && buf[28] == 0x1b {
		dst[0] = binary.BigEndian.Uint64(buf[2:])
		dst[1] = binary.BigEndian.Uint64(buf[11:])
		dst[2] = binary.BigEndian.Uint64(buf[20:])
		dst[3] = binary.BigEndian.Uint64(buf[29:])
		r.pos += feltLen
		return nil
	}
	if n := shortLimbs(buf, dst); n > 0 {
		r.pos += n
		return nil
	}
	return r.limbsSlow(dst)
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
func (r *Reader) limbsSlow(dst *[Limbs]uint64) error {
	start := r.pos
	count, err := r.expect(MajorArray)
	if err != nil {
		return err
	}
	// The felt is an array, so what it holds, which a longer one skips, sits one level deeper.
	if err := r.enter(); err != nil {
		r.pos = start
		return err
	}
	// Like a Go array in fxamacker, a shorter array leaves the rest zero and a longer one is
	// cut off. Juno never writes either.
	var limbs [Limbs]uint64
	for i := range count {
		if i >= Limbs {
			err = r.Skip()
		} else {
			limbs[i], err = r.Uint64()
		}
		if err != nil {
			r.depth--
			r.pos = start
			return err
		}
	}
	r.depth--
	*dst = limbs
	return nil
}

// LimbSlice reads an array of felts, or null as a nil slice. F is any type with the layout
// of a felt. The slice is sized exactly from the element count, which the input caps: every
// felt takes at least five bytes.
func LimbSlice[F ~[Limbs]uint64](r *Reader) ([]F, error) {
	if r.Null() {
		return nil, nil
	}
	start := r.pos
	count, err := r.expect(MajorArray)
	if err != nil {
		return nil, err
	}
	if count > uint64(r.Len())/(1+Limbs) {
		// Every felt Juno writes takes at least five bytes, so this count is a damaged one or a
		// felt written in a form fxamacker also takes (an empty array is a zero felt). It is
		// never worth an allocation the input cannot back; fxamacker decides.
		r.pos = start
		return nil, ErrUnsupported
	}
	out := make([]F, count)
	// F has the layout of [Limbs]uint64, so the slice can be read as one.
	felts := unsafe.Slice((*[Limbs]uint64)(unsafe.Pointer(unsafe.SliceData(out))), len(out))
	if err := r.readLimbs(felts); err != nil {
		r.pos = start
		return nil, err
	}
	return out, nil
}

// readLimbs fills felts from consecutive felts in the input. The 37 byte form is read in the
// loop itself; any other goes through Limbs. It may write felts before failing.
func (r *Reader) readLimbs(felts [][Limbs]uint64) error {
	buf := r.buf[r.pos:]
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
		r.pos = len(r.buf) - len(buf)
		if err := r.Limbs(&felts[i]); err != nil {
			return err
		}
		buf = r.buf[r.pos:]
	}
	r.pos = len(r.buf) - len(buf)
	return nil
}

// Felt reads a felt into any type with the layout of one.
func Felt[F ~[Limbs]uint64](r *Reader, dst *F) error {
	return r.Limbs((*[Limbs]uint64)(unsafe.Pointer(dst)))
}
