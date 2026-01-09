package db_test

import (
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	t.Run("bucket with no key", func(t *testing.T) {
		key := db.StateTrie.Key()
		assert.Equal(t, []byte{byte(db.StateTrie)}, key)
	})
	t.Run("bucket with nil key", func(t *testing.T) {
		key := db.StateTrie.Key(nil)
		assert.Equal(t, []byte{byte(db.StateTrie)}, key)
	})
	t.Run("bucket with multiple keys", func(t *testing.T) {
		keys := [][]byte{{}, {0}, {0, 1, 2, 3, 4}, {0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}

		for _, k := range keys {
			t.Run(string(rune(len(k))), func(t *testing.T) {
				expectedKey := make([]byte, 0, 1+len(k))
				expectedKey = append(expectedKey, byte(db.StateTrie))
				expectedKey = append(expectedKey, k...)
				assert.Equal(t, expectedKey, db.StateTrie.Key(k))
			})
		}
	})
}
