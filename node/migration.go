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
	"github.com/NethermindEth/juno/utils/log"
)

// registerMigrations creates and configures the migration registry with all migrations.
// This is where all migrations should be registered. Optional migrations can use
// config variables from cfg to determine if they should be enabled.
func registerMigrations(cfg *Config) *migration.Registry {
	registry := migration.NewRegistry().
		With(&blocktransactions.Migrator{}).
		WithOptional(
			historyprunner.New(cfg.RetainedBlocks),
			cfg.Prune,
			PruneModeFlag,
		)

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

		// The history pruning migration reads the L1 head to compute the
		// reorg-safe cutoff. On a fresh sync the L1 client (which writes the
		// head) only starts as a regular service AFTER this point, so a
		// pruning-enabled node restarted mid-sync would fail with a cryptic
		// "key not found". Bootstrap the head via a one-shot catch-up here,
		// reusing the operator's existing --eth-node so no extra config is
		// required. Skipped if the head is already on disk from a previous run.
		if config.Prune {
			if err := bootstrapL1HeadIfMissing(ctx, database, config, chain, logger); err != nil {
				return fmt.Errorf("bootstrap L1 head for pruning: %w", err)
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

// bootstrapL1HeadIfMissing ensures an L1 head is persisted before the
// history pruning migration reads it. No-op when the head is already on
// disk. Otherwise it builds a one-shot L1 client against the operator's
// existing --eth-node and runs the same catch-up scan the regular L1
// service runs on startup. Returns an explicit error if the catch-up
// completes without finding any finalised LogStateUpdate, so the operator
// sees a clear cause instead of a downstream "key not found".
func bootstrapL1HeadIfMissing(
	ctx context.Context,
	database db.KeyValueStore,
	config *Config,
	chain *blockchain.Blockchain,
	logger log.StructuredLogger,
) error {
	if _, err := core.GetL1Head(database); err == nil {
		return nil
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	logger.Info("Bootstrapping L1 head for pruning migration via --eth-node catch-up")
	client, err := newL1Client(config.EthNode, config.Metrics, chain, logger)
	if err != nil {
		return err
	}
	if err := client.CatchUpL1Head(ctx); err != nil {
		return err
	}

	if _, err := core.GetL1Head(database); err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return errors.New("catch-up completed without a finalised LogStateUpdate; " +
				"retry once L1 has a finalised Starknet state update")
		}
		return err
	}
	return nil
}
