package pebblev2_test

import (
	"crypto/rand"
	"encoding/binary"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/encoder"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/cockroachdb/pebble/v2/sstable/block"
	"github.com/stretchr/testify/require"
)

const (
	fieldCount = 20
	dataCount  = 1000
)

type entry struct {
	key   []byte
	value []byte
}

var fields = func() []string {
	keys := make([]string, fieldCount)
	for i := range keys {
		keys[i] = rand.Text()
	}
	return keys
}()

func buildTestKey(key int) []byte {
	keyBytes := make([]byte, 64)
	binary.BigEndian.PutUint64(keyBytes, uint64(key))
	return keyBytes
}

func buildTestValue(t *testing.T) []byte {
	t.Helper()

	data := make(map[string]string)
	for _, key := range fields {
		data[key] = rand.Text()
	}

	dataBytes, err := encoder.Marshal(data)
	require.NoError(t, err)
	return dataBytes
}

type entries struct {
	entries []entry
}

func (e *entries) writeEntries(t *testing.T, database db.KeyValueStore, count int) {
	t.Helper()
	start := len(e.entries)
	e.entries = append(e.entries, make([]entry, count)...)
	for i := start; i < len(e.entries); i++ {
		e.entries[i] = entry{
			key:   buildTestKey(i),
			value: buildTestValue(t),
		}
		require.NoError(t, database.Put(e.entries[i].key, e.entries[i].value))
	}
}

func (e *entries) validateEntries(t *testing.T, database db.KeyValueStore) {
	t.Helper()
	for _, entry := range e.entries {
		err := database.Get(entry.key, func(value []byte) error {
			require.Equal(t, entry.value, value)
			return nil
		})
		require.NoError(t, err)
	}
}

type scenario struct {
	v1 bool
	v2 bool
}

func runScenario(t *testing.T, scenario scenario) {
	name := ""
	if scenario.v1 {
		name += "v1 -> "
	}
	if scenario.v2 {
		name += "v2 -> "
	}
	name += "v2zstd"

	t.Run(name, func(t *testing.T) {
		t.Helper()

		entries := entries{}
		path := t.TempDir()

		if scenario.v1 {
			t.Run("open in v1", func(t *testing.T) {
				database, err := pebble.New(path)
				require.NoError(t, err)
				defer database.Close()

				entries.writeEntries(t, database, dataCount)
				entries.validateEntries(t, database)
			})
		}

		if scenario.v2 {
			for range 2 {
				t.Run("open in v2", func(t *testing.T) {
					database, err := pebblev2.New(path)
					require.NoError(t, err)
					defer database.Close()

					entries.validateEntries(t, database)
					entries.writeEntries(t, database, dataCount)
					entries.validateEntries(t, database)
				})
			}
		}

		for range 2 {
			t.Run("open in v2 with compression", func(t *testing.T) {
				database, err := pebblev2.New(path, pebblev2.WithCompression(block.ZstdCompression))
				require.NoError(t, err)
				defer database.Close()

				entries.validateEntries(t, database)
				entries.writeEntries(t, database, dataCount)
				entries.validateEntries(t, database)
			})
		}
	})
}

func TestUpgradeFormatIfNeeded(t *testing.T) {
	for _, v1 := range []bool{true, false} {
		for _, v2 := range []bool{true, false} {
			runScenario(t, scenario{v1: v1, v2: v2})
		}
	}
}
