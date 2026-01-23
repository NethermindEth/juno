package migration

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/value"
)

// SchemaMetadata tracks the migration state of the database.
//
// CurrentVersion: migrations that have been successfully applied to the database.
// LastTargetVersion: the target version from the previous migration run.
// This is used to prevent opt-out: users cannot disable migrations that were
// previously enabled. When a migration run starts, the current targetVersion
// is saved here to become the "last target" for the next run.
type SchemaMetadata struct {
	CurrentVersion    SchemaVersion // Applied migrations
	LastTargetVersion SchemaVersion // Previous target (for opt-out prevention)
}

// Schema metadata represented as [SchemaMetadata]
var SchemaMetadataBucket = typed.NewBucket(
	db.SchemaMetadata,
	key.Empty,
	value.Cbor[SchemaMetadata](),
)

// Schema intermediate state represented as []byte
var SchemaIntermediateState = typed.NewBucket(
	db.SchemaIntermediateState,
	key.Uint8,
	value.Bytes,
)

// GetSchemaMetadata gets the schema metadata from the database.
func GetSchemaMetadata(r db.KeyValueReader) (SchemaMetadata, error) {
	return SchemaMetadataBucket.Get(r, struct{}{})
}

// WriteSchemaMetadata writes the schema metadata to the database.
func WriteSchemaMetadata(txn db.KeyValueWriter, sm SchemaMetadata) error {
	return SchemaMetadataBucket.Put(txn, struct{}{}, &sm)
}

// GetIntermediateState gets the intermediate state for a specific migration.
func GetIntermediateState(r db.KeyValueReader, migrationIndex uint8) ([]byte, error) {
	return SchemaIntermediateState.Get(r, migrationIndex)
}

// WriteIntermediateState writes the intermediate state for a specific migration.
func WriteIntermediateState(w db.KeyValueWriter, migrationIndex uint8, state []byte) error {
	return SchemaIntermediateState.Put(w, migrationIndex, &state)
}

// DeleteIntermediateState deletes the intermediate state for a specific migration.
func DeleteIntermediateState(w db.KeyValueWriter, migrationIndex uint8) error {
	return SchemaIntermediateState.Delete(w, migrationIndex)
}
