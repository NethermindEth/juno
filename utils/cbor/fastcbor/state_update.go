package fastcbor

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
)

//nolint:gocritic // dst is given the map made here when there is none
func readMigratedClasses(r *cursor.Cursor, dst *map[felt.SierraClassHash]felt.CasmClassHash) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.MapLen()
	if err != nil {
		return err
	}
	out := make(map[felt.SierraClassHash]felt.CasmClassHash, count)
	for ; count > 0; count-- {
		var key felt.SierraClassHash
		if err := cursor.Felt(r, &key); err != nil {
			return err
		}
		var value felt.CasmClassHash
		if err := cursor.Felt(r, &value); err != nil {
			return err
		}
		out[key] = value
	}
	r.Leave()
	*dst = out
	return nil
}
