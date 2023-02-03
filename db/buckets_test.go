package db_test

import (
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	key := db.State.Key([]byte{1})
	assert.Equal(t, []byte{byte(db.State), 1}, key)
	key = db.State.Key([]byte{1}, []byte{2})
	assert.Equal(t, []byte{byte(db.State), 1, 2}, key)
	key = db.StateTrie.Key([]byte{1}, []byte{2})
	assert.Equal(t, []byte{byte(db.StateTrie), 1, 2}, key)
}
