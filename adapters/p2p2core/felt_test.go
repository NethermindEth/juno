package p2p2core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdaptNilReturnsNil(t *testing.T) {
	assert.Nil(t, AdaptHash(nil))
	assert.Nil(t, AdaptAddress(nil))
	assert.Nil(t, AdaptFelt(nil))
}
