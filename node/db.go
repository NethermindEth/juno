package node

import (
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/utils/log"
)

func initializeLocalDB(cfg *Config) (db.KeyValueStore, error) {
	// note(rdr): A dedicated logger with level Error to avoid noise.
	dbLog, err := log.NewZapLogger(
		log.NewLevel(log.ERROR),
		log.WithColour(cfg.Colour),
		log.WithJSON(cfg.LogJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("create DB logger: %w", err)
	}

	return pebblev2.New(
		cfg.DatabasePath,
		pebblev2.WithCacheSize(cfg.DBCacheSize),
		pebblev2.WithMaxOpenFiles(cfg.DBMaxHandles),
		pebblev2.WithLogger(dbLog),
		pebblev2.WithCompactionConcurrency(cfg.DBCompactionConcurrency),
		pebblev2.WithMemtableSize(cfg.DBMemtableSize),
		pebblev2.WithMemtableCount(cfg.DBMemtableCount),
		pebblev2.WithCompression(cfg.DBCompression),
	)
}
