package triedb

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/triedb/hashdb"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/triedb/rawdb"
	"github.com/NethermindEth/juno/db"
)

type Config struct {
	PathConfig *pathdb.Config
	HashConfig *hashdb.Config
	RawConfig  *rawdb.Config
}

func New(disk db.KeyValueStore, config *Config) (database.TrieDB, error) {
	// Default to raw config if not provided
	if config == nil {
		return rawdb.New(disk), nil
	} else if config.PathConfig != nil {
		return pathdb.New(disk, config.PathConfig)
	} else if config.HashConfig != nil {
		return hashdb.New(disk, config.HashConfig), nil
	} else if config.RawConfig != nil {
		return rawdb.New(disk), nil
	}
	return nil, fmt.Errorf("invalid config")
}

func (d *Database) Commit(stateComm *felt.Felt) error {
	return d.triedb.Commit(stateComm)
}
