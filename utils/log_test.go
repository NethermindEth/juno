package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZap(t *testing.T) {
	levels := []LogLevel{
		DEBUG, INFO, WARN, ERROR,
	}

	for _, level := range levels {
		t.Run("test level: "+level.String(), func(t *testing.T) {
			_, err := NewZapLogger(level)
			assert.NoError(t, err)
		})
	}
}
