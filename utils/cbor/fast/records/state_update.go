package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

// ReadStateUpdate reads a stored state update.
func ReadStateUpdate(r *fast.Reader, dst *core.StateUpdate) error {
	return readFields(r, foldStateUpdate, func(key string) (bool, error) {
		var err error
		switch key {
		case "BlockHash":
			err = readFelt(r, &dst.BlockHash)
		case "NewRoot":
			err = readFelt(r, &dst.NewRoot)
		case "OldRoot":
			err = readFelt(r, &dst.OldRoot)
		case "StateDiff":
			err = readPtr(r, &dst.StateDiff, readStateDiff)
		default:
			return false, nil
		}
		return true, err
	})
}

func readStateDiff(r *fast.Reader, dst *core.StateDiff) error {
	return readFields(r, foldStateDiff, func(key string) (bool, error) {
		var err error
		switch key {
		case "StorageDiffs":
			err = readMap(r, &dst.StorageDiffs, readStorageDiff)
		case "Nonces":
			err = readMap(r, &dst.Nonces, readFelt)
		case "DeployedContracts":
			err = readMap(r, &dst.DeployedContracts, readFelt)
		case "DeclaredV0Classes":
			err = readFeltPtrSlice(r, &dst.DeclaredV0Classes)
		case "DeclaredV1Classes":
			err = readMap(r, &dst.DeclaredV1Classes, readFelt)
		case "ReplacedClasses":
			err = readMap(r, &dst.ReplacedClasses, readFelt)
		case "MigratedClasses":
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
