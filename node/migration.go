package node

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/deprecatedmigration" //nolint:staticcheck,nolintlint,lll // ignore statick check package will be removed in future, nolinlint because main config does not check
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/utils"
)

// registerMigrations creates and configures the migration registry with all migrations.
// This is where all migrations should be registered. Optional migrations can use
// config variables from cfg to determine if they should be enabled.
func registerMigrations(cfg *Config) *migration.Registry {
	// cfg parameter is reserved for future optional migrations based on config
	_ = cfg

	registry := migration.NewRegistry()

	return registry
}

// migrateIfNeeded runs all migrations (deprecated and new) if needed.
// If HTTP is enabled in config, it will start a status server to expose
// migration progress via health check endpoints.
func migrateIfNeeded(
	ctx context.Context,
	db db.KeyValueStore,
	config *Config,
	log utils.Logger,
) error {
	migrateFn := func() error {
		// Run deprecated migrations first
		if err := deprecatedmigration.MigrateIfNeeded(
			ctx,
			db,
			&config.Network,
			log,
		); err != nil {
			return fmt.Errorf("deprecated migration failed: %w", err)
		}

		// Run new migrations
		registry := registerMigrations(config)
		runner, err := migration.NewRunner(
			registry.Entries(),
			registry.TargetVersion(),
			db,
			&config.Network,
			log,
		)
		if err != nil {
			return fmt.Errorf("create migration runner: %w", err)
		}

		return runner.Run(ctx)
	}

	if config.HTTP {
		return migration.RunWithServer(
			log,
			config.HTTPHost,
			config.HTTPPort,
			migrateFn,
		)
	}

	return migrateFn()
}
