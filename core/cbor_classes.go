package core

import (
	"encoding/json"
	"math/big"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
)

// RawMessage is stored as an opaque byte string in CBOR. Decoding does not parse
// or validate its JSON contents, matching the planner and persisted format.
func cborReadRawMessage(c *cursor.Cursor, dst *json.RawMessage) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	value, err := c.Bytes()
	if err == nil {
		*dst = value
	}
	return err
}

func cborReadBigIntPointer(c *cursor.Cursor, dst **big.Int) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	value, err := c.BigInt()
	if err == nil {
		*dst = value
	}
	return err
}

func cborReadStrings(c *cursor.Cursor, dst *[]string) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	count, err := c.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]string, count)
	for i := range out {
		if err := cborReadString(c, &out[i]); err != nil {
			return err
		}
	}
	c.Leave()
	*dst = out
	return nil
}
