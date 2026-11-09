package cursor

// Skip consumes the next item and checks its syntax, but not UTF-8. Indefinite-length items and
// tags 0-3, whose content only a reader that returns the value can check, are ErrUnsupported.
// It consumes nothing on error.
func (c *Cursor) Skip() error {
	start := c.pos
	if err := c.skip(c.depth); err != nil {
		c.pos = start
		return err
	}
	return nil
}

// skip takes the number of arrays, maps and chained tags already open around the item.
//
//nolint:gocyclo // one switch over the CBOR major types
func (c *Cursor) skip(depth int) error {
	// A felt in the uint64 form is the bulk of a record and needs no head decoding. It opens a level
	// of its own.
	if buf := c.buf[c.pos:]; depth < maxNestedLevels && len(buf) >= feltLen && buf[0] == feltHead &&
		buf[1] == 0x1b && buf[10] == 0x1b && buf[19] == 0x1b && buf[28] == 0x1b {
		c.pos += feltLen
		return nil
	}

	head := byte(0)
	if c.pos < len(c.buf) {
		head = c.buf[c.pos]
	}
	major, argument, err := c.head()
	if err != nil {
		return err
	}

	switch major {
	case MajorUint, majorNegInt:
		return nil

	case majorSimple:
		// One-byte simple values below 32 must use the short form.
		if head&infoMask == info1Byte && argument < 32 {
			return c.fail(ErrSyntax, "simple value below 32 in two bytes")
		}
		return nil

	case majorBytes, majorString:
		if argument > uint64(c.Len()) {
			return c.fail(ErrTruncated, "byte or text string longer than the input")
		}
		c.pos += int(argument)
		return nil

	case MajorTag:
		if argument <= 3 {
			return c.fail(ErrUnsupported, "tag 0 to 3, whose content is not checked")
		}
		// Only a tag whose content is another tag counts as a level, so a single tag is free.
		if next, err := c.Peek(); err == nil && next&majorMask == MajorTag {
			depth++
			if depth > maxNestedLevels {
				return errDepth(c.pos)
			}
		}
		return c.skip(depth)

	case majorArray:
		// Every element takes at least one byte.
		return c.skipElements(argument, uint64(c.Len()), 1, depth)

	default: // majorMap, the only major left. A pair takes at least two bytes.
		return c.skipElements(argument, uint64(c.Len())/2, 2, depth)
	}
}

// skipElements skips count*perCount items, rejecting a count the remaining input cannot hold.
func (c *Cursor) skipElements(count, limit uint64, perCount, depth int) error {
	depth++
	if depth > maxNestedLevels {
		return errDepth(c.pos)
	}
	if count > limit {
		return c.fail(ErrTruncated, "count exceeds the input")
	}
	for n := count * uint64(perCount); n > 0; n-- {
		if buf := c.buf[c.pos:]; depth < maxNestedLevels && len(buf) >= feltLen && buf[0] == feltHead &&
			buf[1] == 0x1b && buf[10] == 0x1b && buf[19] == 0x1b && buf[28] == 0x1b {
			c.pos += feltLen
			continue
		}
		if err := c.skip(depth); err != nil {
			return err
		}
	}
	return nil
}
