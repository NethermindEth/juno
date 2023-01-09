package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	key := State.Key([]byte{1})
	assert.Equal(t, []byte{byte(State), 1}, key)
	key = State.Key([]byte{1}, []byte{2})
	assert.Equal(t, []byte{byte(State), 1, 2}, key)
	key = StateTrie.Key([]byte{1}, []byte{2})
	assert.Equal(t, []byte{byte(StateTrie), 1, 2}, key)
}
