package statebackend

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

// storeCasmHashMetadata stores CASM hash metadata for declared and migrated classes.
// See [core.ClassCasmHashMetadata].
func storeCasmHashMetadata(
	reader db.KeyValueReader,
	writer db.KeyValueWriter,
	blockNumber uint64,
	protocolVersion string,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	ver, err := core.ParseBlockVersion(protocolVersion)
	if err != nil {
		return err
	}

	isV2Protocol := ver.GreaterThanEqual(core.Ver0_14_1)

	if isV2Protocol {
		return storeCasmHashMetadataV2(reader, writer, blockNumber, stateUpdate)
	}

	return storeCasmHashMetadataV1(writer, blockNumber, stateUpdate, newClasses)
}

// storeCasmHashMetadataV2 stores metadata for classes declared with casm hash v2 or
// migrated from v1. casm hash v2 is after protocol version >= 0.14.1.
func storeCasmHashMetadataV2(
	reader db.KeyValueReader,
	writer db.KeyValueWriter,
	blockNumber uint64,
	stateUpdate *core.StateUpdate,
) error {
	for sierraClassHash, casmHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		metadata := core.NewCasmHashMetadataDeclaredV2(
			blockNumber,
			(*felt.CasmClassHash)(casmHash),
		)
		err := core.WriteClassCasmHashMetadata(
			writer,
			(*felt.SierraClassHash)(&sierraClassHash),
			&metadata,
		)
		if err != nil {
			return err
		}
	}

	for sierraClassHash := range stateUpdate.StateDiff.MigratedClasses {
		metadata, err := core.GetClassCasmHashMetadata(reader, &sierraClassHash)
		if err != nil {
			return fmt.Errorf("cannot migrate class %s: metadata not found",
				sierraClassHash.String(),
			)
		}

		if err := metadata.Migrate(blockNumber); err != nil {
			return fmt.Errorf("failed to migrate class %s at block %d: %w",
				sierraClassHash.String(),
				blockNumber,
				err,
			)
		}

		err = core.WriteClassCasmHashMetadata(writer, &sierraClassHash, &metadata)
		if err != nil {
			return err
		}
	}
	return nil
}

// storeCasmHashMetadataV1 stores metadata for classes declared with V1 hash (protocol < 0.14.1).
// It computes the V2 hash from the class definition.
func storeCasmHashMetadataV1(
	writer db.KeyValueWriter,
	blockNumber uint64,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	for sierraClassHash, casmHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		casmHashV1 := (*felt.CasmClassHash)(casmHash)

		classDef, ok := newClasses[sierraClassHash]
		if !ok {
			return fmt.Errorf("class %s not available in newClasses at block %d",
				sierraClassHash.String(),
				blockNumber,
			)
		}

		sierraClass, ok := classDef.(*core.SierraClass)
		if !ok {
			return fmt.Errorf("class %s must be a SierraClass at block %d",
				sierraClassHash.String(),
				blockNumber,
			)
		}

		v2Hash := sierraClass.Compiled.Hash(core.HashVersionV2)
		casmHashV2 := felt.CasmClassHash(v2Hash)

		metadata := core.NewCasmHashMetadataDeclaredV1(blockNumber, casmHashV1, &casmHashV2)
		err := core.WriteClassCasmHashMetadata(
			writer,
			(*felt.SierraClassHash)(&sierraClassHash),
			&metadata,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// revertCasmHashMetadata reverts CASM hash metadata for declared and migrated classes.
func revertCasmHashMetadata(
	r db.KeyValueReader,
	w db.Batch,
	stateUpdate *core.StateUpdate,
) error {
	for sierraClassHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		err := core.DeleteClassCasmHashMetadata(w, (*felt.SierraClassHash)(&sierraClassHash))
		if err != nil {
			return err
		}
	}
	for sierraClassHash := range stateUpdate.StateDiff.MigratedClasses {
		metadata, err := core.GetClassCasmHashMetadata(r, &sierraClassHash)
		if err != nil {
			return fmt.Errorf("revert migrated class %s: %w", sierraClassHash.String(), err)
		}
		if err := metadata.Unmigrate(); err != nil {
			return fmt.Errorf("failed to unmigrate class %s: %w", sierraClassHash.String(), err)
		}
		if err := core.WriteClassCasmHashMetadata(w, &sierraClassHash, &metadata); err != nil {
			return err
		}
	}
	return nil
}
