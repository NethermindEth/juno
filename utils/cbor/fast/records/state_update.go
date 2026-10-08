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
			err = readMap(r, &dst.StorageDiffs, readStorageDiff)
		case "Nonces":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readMap(r, &dst.Nonces, readFelt)
		case "DeployedContracts":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readMap(r, &dst.DeployedContracts, readFelt)
		case "DeclaredV0Classes":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readFeltPtrSlice(r, &dst.DeclaredV0Classes)
		case "DeclaredV1Classes":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readMap(r, &dst.DeclaredV1Classes, readFelt)
		case "ReplacedClasses":
			if once(&seen, 6) {
				return false, fast.ErrUnsupported
			}
			err = readMap(r, &dst.ReplacedClasses, readFelt)
		case "MigratedClasses":
			if once(&seen, 7) {
				return false, fast.ErrUnsupported
			}
			err = readMap(r, &dst.MigratedClasses, func(r *fast.Reader, v *felt.CasmClassHash) error {
				return fast.Felt(r, v)
			})
		default:
			return false, nil
		}
		return true, err
	})
}

func readStorageDiff(r *fast.Reader, dst *map[felt.Felt]*felt.Felt) error {
	return readMap(r, dst, readFelt)
}
