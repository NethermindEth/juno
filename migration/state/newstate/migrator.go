package newstate

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/headstate"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/history"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

var _ migration.Migration = (*Migrator)(nil)

type phase struct {
	name string
	m    migration.Migration
}

// Migrator applies new state migrations as one unit.
//
// Intermediate state is [phase index][that phase's own state].
type Migrator struct {
	phases   []phase
	phase    uint8
	subState []byte
}

// New returns a migrator over the phases in the only order they may run
func New() *Migrator {
	return &Migrator{phases: []phase{
		{name: "headstate", m: &headstate.Migrator{}},
		{name: "history", m: &history.Migrator{}},
	}}
}

// Before restores the phase to resume from and that phase's own state.
func (m *Migrator) Before(state []byte) error {
	if len(state) == 0 {
		m.phase, m.subState = 0, nil
		return nil
	}
	if int(state[0]) >= len(m.phases) {
		return fmt.Errorf(
			"newstate: intermediate state names phase %d, but only %d phases are registered",
			state[0], len(m.phases),
		)
	}
	m.phase, m.subState = state[0], state[1:]
	return nil
}

// Migrate runs the phases in order, starting from the one Before restored.
func (m *Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	for i := int(m.phase); i < len(m.phases); i++ {
		p := m.phases[i]

		var sub []byte
		if i == int(m.phase) {
			sub = m.subState
		}
		if err := p.m.Before(sub); err != nil {
			return checkpoint(i, sub), fmt.Errorf("newstate: %s: restoring state: %w", p.name, err)
		}

		logger.Info("Applying new state migration phase",
			zap.String("phase", p.name),
			zap.String("progress", fmt.Sprintf("%d/%d", i+1, len(m.phases))),
		)

		next, err := p.m.Migrate(ctx, database, network, logger)
		if err != nil {
			return checkpoint(i, next), fmt.Errorf("newstate: %s: %w", p.name, err)
		}
		if next != nil {
			return checkpoint(i, next), nil
		}
	}

	return nil, nil
}

func checkpoint(i int, sub []byte) []byte {
	return append([]byte{uint8(i)}, sub...)
}
