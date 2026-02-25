package triedb

import (
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

func New(disk db.KeyValueStore, config *Config) database.TrieDB {
	// Default to raw config if not provided
	if config == nil {
		return rawdb.New(disk)
	} else if config.PathConfig != nil {
		pathDB, err := pathdb.New(disk, config.PathConfig)
		if err != nil {
			panic(err)
		}
		return pathDB
	} else if config.HashConfig != nil {
		return hashdb.New(disk, config.HashConfig)
	} else if config.RawConfig != nil {
		return rawdb.New(disk)
	}
	return nil
}
