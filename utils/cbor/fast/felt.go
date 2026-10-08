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
	return r.limbsSlow(dst)
}

// limbsSlow reads a felt whose limbs are not all eight bytes wide.
func (r *Reader) limbsSlow(dst *[Limbs]uint64) error {
	start := r.pos
	count, err := r.expect(MajorArray)
	if err != nil {
		return err
	}
	if count != Limbs {
		r.pos = start
		return ErrType
	}
	var limbs [Limbs]uint64
	for i := range limbs {
		limbs[i], err = r.Uint64()
		if err != nil {
			r.pos = start
			return err
		}
	}
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
		r.pos = start
		return nil, ErrTruncated
	}
	out := make([]F, count)
	// F has the layout of [Limbs]uint64, so the slice can be read as one.
	felts := unsafe.Slice((*[Limbs]uint64)(unsafe.Pointer(unsafe.SliceData(out))), len(out))
	for i := range felts {
		if err := r.Limbs(&felts[i]); err != nil {
			r.pos = start
			return nil, err
		}
	}
	return out, nil
}

// Felt reads a felt into any type with the layout of one.
func Felt[F ~[Limbs]uint64](r *Reader, dst *F) error {
	return r.Limbs((*[Limbs]uint64)(unsafe.Pointer(dst)))
}
