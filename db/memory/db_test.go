package memory

import (
	"testing"

	"github.com/NethermindEth/juno/db"
)

func TestMemoryDB(t *testing.T) {
	t.Run("test suite", func(t *testing.T) {
		db.TestKeyValueStoreSuite(t, func() db.KeyValueStore {
			return New()
		})
	})
}
