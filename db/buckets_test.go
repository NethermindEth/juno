package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	key := Key(State, []byte{1})
	assert.Equal(t, []byte{State, 1}, key)
	key = Key(State, []byte{1}, []byte{2})
	assert.Equal(t, []byte{State, 1, 2}, key)
	key = Key(StateTrie, []byte{1}, []byte{2})
	assert.Equal(t, []byte{StateTrie, 1, 2}, key)
}
