package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

// ReadStateUpdate reads a stored state update.
func ReadStateUpdate(r *fast.Reader, dst *core.StateUpdate) error {
	var seen uint64
	return readFields(r, foldStateUpdate, func(key string) (bool, error) {
		var err error
		switch key {
		case "BlockHash":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.BlockHash)
		case "NewRoot":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.NewRoot)
		case "OldRoot":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.OldRoot)
		case "StateDiff":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readPtr(r, &dst.StateDiff, readStateDiff)
		default:
			return false, nil
		}
		return true, err
	})
}

func readStateDiff(r *fast.Reader, dst *core.StateDiff) error {
	var seen uint64
	return readFields(r, foldStateDiff, func(key string) (bool, error) {
		var err error
		switch key {
		case "StorageDiffs":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readStorageDiffs(r, &dst.StorageDiffs)
		case "Nonces":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFeltMap(r, &dst.Nonces)
		case "DeployedContracts":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readFeltMap(r, &dst.DeployedContracts)
		case "DeclaredV0Classes":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readFeltPtrSlice(r, &dst.DeclaredV0Classes)
		case "DeclaredV1Classes":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readFeltMap(r, &dst.DeclaredV1Classes)
		case "ReplacedClasses":
			if once(&seen, 6) {
				return false, fast.ErrUnsupported
			}
			err = readFeltMap(r, &dst.ReplacedClasses)
		case "MigratedClasses":
			if once(&seen, 7) {
				return false, fast.ErrUnsupported
			}
			err = readMigratedClasses(r, &dst.MigratedClasses)
		default:
			return false, nil
		}
		return true, err
	})
}

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
