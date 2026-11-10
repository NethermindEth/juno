package core

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
)

func cborReadFeltMap(c *cursor.Cursor, dst *map[felt.Felt]*felt.Felt) error {
	var spare []felt.Felt
	return cborReadFeltMapFrom(c, dst, &spare, 0)
}

// Values share arrays across this map and, for storage diffs, across subsequent
// inner maps. Duplicate keys replace the prior pointer, including with nil.
func cborReadFeltMapFrom(c *cursor.Cursor, dst *map[felt.Felt]*felt.Felt, spare *[]felt.Felt, extra int) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	count, err := c.MapLen()
	if err != nil {
		return err
	}
	out := make(map[felt.Felt]*felt.Felt, count)
	for left := count; left > 0; left-- {
		var key felt.Felt
		if err := c.Limbs((*[4]uint64)(&key)); err != nil {
			return err
		}
		if c.Null() {
			out[key] = nil
			continue
		}
		value := cborNextFelt(spare, left+extra)
		if err := c.Limbs((*[4]uint64)(value)); err != nil {
			return err
		}
		out[key] = value
	}
	c.Leave()
	*dst = out
	return nil
}

// Specialize slices.Grow(nil, want) to its append/make path, retaining capacity
// rounding without a dependency on an untranslated library helper. Callers
// supply a positive count; the whole newly allocated capacity starts zeroed.
func cborNextFelt(spare *[]felt.Felt, want int) *felt.Felt {
	if len(*spare) == 0 {
		grown := append([]felt.Felt(nil), make([]felt.Felt, want)...)
		*spare = grown[:cap(grown)]
	}
	value := &(*spare)[0]
	*spare = (*spare)[1:]
	return value
}

func cborReadStorageDiffs(c *cursor.Cursor, dst *map[felt.Felt]map[felt.Felt]*felt.Felt) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	count, err := c.MapLen()
	if err != nil {
		return err
	}
	out := make(map[felt.Felt]map[felt.Felt]*felt.Felt, count)
	var spare []felt.Felt
	for ; count > 0; count-- {
		var key felt.Felt
		if err := c.Limbs((*[4]uint64)(&key)); err != nil {
			return err
		}
		var value map[felt.Felt]*felt.Felt
		if err := cborReadFeltMapFrom(c, &value, &spare, count-1); err != nil {
			return err
		}
		out[key] = value
	}
	c.Leave()
	*dst = out
	return nil
}

func cborReadMigratedClasses(c *cursor.Cursor, dst *map[felt.SierraClassHash]felt.CasmClassHash) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	count, err := c.MapLen()
	if err != nil {
		return err
	}
	out := make(map[felt.SierraClassHash]felt.CasmClassHash, count)
	for ; count > 0; count-- {
		var key felt.SierraClassHash
		if err := c.Limbs((*[4]uint64)(&key)); err != nil {
			return err
		}
		var value felt.CasmClassHash
		if err := c.Limbs((*[4]uint64)(&value)); err != nil {
			return err
		}
		out[key] = value
	}
	c.Leave()
	*dst = out
	return nil
}
