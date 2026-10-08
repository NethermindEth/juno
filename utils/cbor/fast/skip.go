package fast

// Skip consumes the next item, whatever it is, and checks its syntax on the way: heads must
// be well-formed, lengths must fit the input and nesting must stay within MaxNestedLevels.
// Indefinite-length items return ErrUnsupported. It consumes nothing on error.
//
// Like fxamacker's decoder, it checks neither UTF-8 nor the content type of tags 0-3: only
// a reader that returns the value does. A skipped field that breaks those is accepted by both.
func (r *Reader) Skip() error {
	start := r.pos
	if err := r.skip(0); err != nil {
		r.pos = start
		return err
	}
	return nil
}

// skip takes the number of arrays, maps and chained tags already open around the item.
func (r *Reader) skip(depth int) error {
	head := byte(0)
	if r.pos < len(r.buf) {
		head = r.buf[r.pos]
	}
	major, argument, err := r.Head()
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
		// Like fxamacker, only a tag whose content is another tag counts as a level,
		// so a single tag is free.
		if next, err := r.Peek(); err == nil && next&majorMask == MajorTag {
			depth++
			if depth > MaxNestedLevels {
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
func (r *Reader) skipElements(count, limit uint64, perCount int, depth int) error {
	depth++
	if depth > MaxNestedLevels {
		return ErrDepth
	}
	if count > limit {
		return ErrTruncated
	}
	for n := count * uint64(perCount); n > 0; n-- {
		if err := r.skip(depth); err != nil {
			return err
		}
	}
	return nil
}
