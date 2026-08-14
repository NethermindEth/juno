package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaxFDLimit(t *testing.T) {
	limit, err := MaxFDLimit()
	require.NoError(t, err)
	assert.Positive(t, limit)
}
