package node

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/blocktransactions"
	"github.com/NethermindEth/juno/migration/deprecated" //nolint:staticcheck,nolintlint,lll // ignore statick check package will be removed in future, nolinlint because main config does not check
	"github.com/NethermindEth/juno/utils/log"
)

// registerMigrations creates and configures the migration registry with all migrations.
// This is where all migrations should be registered. Optional migrations can use
// config variables from cfg to determine if they should be enabled.
func registerMigrations(cfg *Config) *migration.Registry {
	// cfg parameter is reserved for optional migrations based on config
	_ = cfg

	registry := migration.NewRegistry().
		With(&blocktransactions.Migrator{})

	return registry
}

// migrateIfNeeded runs all migrations (deprecated and new) if needed.
// If HTTP is enabled in config, it will start a status server to expose
// migration progress via health check endpoints.
func migrateIfNeeded(
	ctx context.Context,
	db db.KeyValueStore,
	config *Config,
	logger log.Logger,
) error {
	migrateFn := func() error {
		// Run deprecated migrations first
		if err := deprecated.MigrateIfNeeded(
			ctx,
			db,
			&config.Network,
			logger,
		); err != nil {
			return fmt.Errorf("deprecated migration failed: %w", err)
		}

		// Run new migrations
		registry := registerMigrations(config)
		runner, err := migration.NewRunner(
			registry,
			db,
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
