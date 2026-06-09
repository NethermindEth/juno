package dummy

import (
	"context"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/utils/log"
)

var _ migration.Migration = (*Migrator)(nil)

type Migrator struct{}

func (m *Migrator) Before(state []byte) error {
	return nil
}

func (m *Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	return nil, nil
}
