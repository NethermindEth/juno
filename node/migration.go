package node

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/blocktransactions"
	"github.com/NethermindEth/juno/migration/deprecated" //nolint:staticcheck,nolintlint,lll // ignore statick check package will be removed in future, nolinlint because main config does not check
	"github.com/NethermindEth/juno/migration/historyprunner"
	"github.com/NethermindEth/juno/migration/state/headstate"
	"github.com/NethermindEth/juno/migration/statehistory"
	"github.com/NethermindEth/juno/utils/log"
)

// registerMigrations creates and configures the migration registry with all migrations.
// This is where all migrations should be registered. Optional migrations can use
// config variables from cfg to determine if they should be enabled.
func registerMigrations(cfg *Config) *migration.Registry {
	registry := migration.NewRegistry().
		With(&blocktransactions.Migrator{}).
		WithOptional(
			historyprunner.New(cfg.RetainedBlocks, cfg.PruneMinAge),
			cfg.Prune,
			PruneModeFlag,
		).
		WithOptional(&headstate.Migrator{}, cfg.NewState, "new-state").
		WithOptional(&statehistory.Migrator{}, cfg.NewState, "new-state")

	return registry
}

// migrateIfNeeded runs all migrations (deprecated and new) if needed.
// If HTTP is enabled in config, it will start a status server to expose
// migration progress via health check endpoints.
func migrateIfNeeded(
	ctx context.Context,
	database db.KeyValueStore,
	config *Config,
	chain *blockchain.Blockchain,
	logger log.Logger,
) error {
	migrateFn := func() error {
		// Run deprecated migrations first
		if err := deprecated.MigrateIfNeeded(
			ctx,
			database,
			&config.Network,
			logger,
		); err != nil {
			return fmt.Errorf("deprecated migration failed: %w", err)
		}

		// Make sure there is an available L1 head before starting pruning migration
		if config.Prune {
			if err := fetchL1HeadIfMissing(ctx, database, config, chain, logger); err != nil {
				return fmt.Errorf("fetch L1 head for pruning: %w", err)
			}
		}

		// Run new migrations
		registry := registerMigrations(config)
		runner, err := migration.NewRunner(
			registry,
			database,
			&config.Network,
			logger,
		)
		if err != nil {
			return fmt.Errorf("create migration runner: %w", err)
		}

		return runner.Run(ctx)
	}

	if config.HTTP {
		return migration.RunWithServer(
			logger,
			config.HTTPHost,
			config.HTTPPort,
			migrateFn,
		)
	}

	return migrateFn()
}

// fetchL1HeadIfMissing writes an L1 head to disk before the history pruning
// migration reads it. No-op when one is already stored.
func fetchL1HeadIfMissing(
	ctx context.Context,
	database db.KeyValueStore,
	config *Config,
	chain *blockchain.Blockchain,
	logger log.StructuredLogger,
) error {
	_, err := core.GetL1Head(database)
	if err == nil {
		return nil
	}
	if !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	logger.Info("Fetching the L1 head before running the prune migration")
	// Metrics are registered by the long-lived L1 client built in node.New; reusing
	// them here would panic via prometheus.MustRegister.
	client, err := newL1Client(config.EthNode, false, chain, logger)
	if err != nil {
		return fmt.Errorf("creating a new L1 client: %w", err)
	}

	if err := client.CatchUpL1Head(ctx); err != nil {
		return fmt.Errorf("catching up to the latest L1 head: %w", err)
	}

	if _, err := core.GetL1Head(database); err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return errors.New("couldn't find a finalized Starknet state update on L1")
		}
		return err
	}
	return nil
}
