package fast

// Skip consumes the next item, whatever it is, and checks its syntax on the way: heads must
// be well-formed, lengths must fit the input and nesting must stay within maxNestedLevels.
// Indefinite-length items return ErrUnsupported. It consumes nothing on error.
//
// Tags 0-3 (times and big integers) return ErrUnsupported: their content has a type to check,
// which only a reader that returns the value does, so what to make of one is left to the caller.
// Skip checks no UTF-8 either, for the same reason.
func (r *Reader) Skip() error {
	start := r.pos
	if err := r.skip(r.depth); err != nil {
		r.pos = start
		return err
	}
	return nil
}

// skip takes the number of arrays, maps and chained tags already open around the item.
//
//nolint:gocyclo // one switch over the CBOR major types
func (r *Reader) skip(depth int) error {
	// A felt is the bulk of what a record holds, and its shape needs no head decoding. It is an
	// array, so it opens a nesting level of its own.
	if buf := r.buf[r.pos:]; depth < maxNestedLevels && len(buf) >= feltLen && buf[0] == feltHead &&
		buf[1] == 0x1b && buf[10] == 0x1b && buf[19] == 0x1b && buf[28] == 0x1b {
		r.pos += feltLen
		return nil
	}

	head := byte(0)
	if r.pos < len(r.buf) {
		head = r.buf[r.pos]
	}
	major, argument, err := r.head()
	if err != nil {
		return err
	}

	switch major {
	case MajorUint, MajorNegInt:
		return nil

	case MajorSimple:
		// One-byte simple values below 32 must use the short form.
		if head&infoMask == info1Byte && argument < 32 {
			return ErrSyntax
		}
		return nil

	case MajorBytes, MajorString:
		if argument > uint64(r.Len()) {
			return ErrTruncated
		}
		r.pos += int(argument)
		return nil

	case MajorTag:
		if argument <= 3 {
			return ErrUnsupported
		}
		// Only a tag whose content is another tag counts as a level, so a single tag is free.
		if next, err := r.Peek(); err == nil && next&majorMask == MajorTag {
			depth++
			if depth > maxNestedLevels {
				return ErrDepth
			}
		}
		return r.skip(depth)

	case MajorArray:
		// Every element takes at least one byte.
		return r.skipElements(argument, uint64(r.Len()), 1, depth)

	default: // MajorMap, the only major left. A pair takes at least two bytes.
		return r.skipElements(argument, uint64(r.Len())/2, 2, depth)
	}
}

// skipElements skips count*perCount items. count is rejected, before any work, when it exceeds
// what the remaining input could hold (limit).
func (r *Reader) skipElements(count, limit uint64, perCount, depth int) error {
	depth++
	if depth > maxNestedLevels {
		return ErrDepth
	}
	if count > limit {
		return ErrTruncated
	}
	for n := count * uint64(perCount); n > 0; n-- {
		if buf := r.buf[r.pos:]; depth < maxNestedLevels && len(buf) >= feltLen && buf[0] == feltHead &&
			buf[1] == 0x1b && buf[10] == 0x1b && buf[19] == 0x1b && buf[28] == 0x1b {
			r.pos += feltLen
			continue
		}
		if err := r.skip(depth); err != nil {
			return err
		}
	}
	return nil
}
