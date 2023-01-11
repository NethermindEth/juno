package felt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshalJson(t *testing.T) {
	var with Felt
	assert.NoError(t, with.UnmarshalJSON([]byte("0x4437ab")))

	var without Felt
	assert.NoError(t, without.UnmarshalJSON([]byte("4437ab")))
	assert.Equal(t, true, without.Equal(&with))
}
