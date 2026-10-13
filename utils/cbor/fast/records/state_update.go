package records

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

func readMigratedClasses(r *fast.Reader, dst *map[felt.SierraClassHash]felt.CasmClassHash) error {
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
		if err := fast.Felt(r, &key); err != nil {
			return err
		}
		var value felt.CasmClassHash
		if err := fast.Felt(r, &value); err != nil {
			return err
		}
		out[key] = value
	}
	r.Leave()
	*dst = out
	return nil
}
