// Package compactfelt records where a database switched to the compact felt forms.
package compactfelt

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

var _ migration.Migration = (*Migrator)(nil)

// Migrator records the switch block. Migrations run before sync, so the chain height when it runs
// is the last block written with legacy felts and every block after it is compact; a rerun after a
// crash sees the same height. The schema bit stops a build without compact decoding from opening
// the database.
type Migrator struct{}

func (*Migrator) Before([]byte) error {
	return nil
}

func (*Migrator) Migrate(
	_ context.Context,
	database db.KeyValueStore,
	_ *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	height, err := core.GetChainHeight(database)
	switch {
	case errors.Is(err, db.ErrKeyNotFound):
		height = 0
	case err != nil:
		return nil, fmt.Errorf("getting chain height: %w", err)
	default:
		height++
	}

	if err := core.CompactFeltHeightBucket.Put(database, struct{}{}, &height); err != nil {
		return nil, fmt.Errorf("writing compact felt height: %w", err)
	}
	logger.Info("Recorded the first block written with compact felts", zap.Uint64("block", height))
	return nil, nil
}
