package utils_test

import (
	"strings"
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var levelStrings = map[utils.LogLevel]string{
	utils.DEBUG: "debug",
	utils.INFO:  "info",
	utils.WARN:  "warn",
	utils.ERROR: "error",
}

func TestLogLevelString(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level "+str, func(t *testing.T) {
			assert.Equal(t, str, level.String())
		})
	}
}

// Tests are similar for LogLevel and Network since they
// both implement the pflag.Value and encoding.TextUnmarshaller interfaces.
// We can open a PR on github.com/thediveo/enumflag to add TextUnmarshaller
// support.
//
//nolint:dupl
func TestLogLevelSet(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level "+str, func(t *testing.T) {
			l := new(utils.LogLevel)
			require.NoError(t, l.Set(str))
			assert.Equal(t, level, *l)
		})
		uppercase := strings.ToUpper(str)
		t.Run("level "+uppercase, func(t *testing.T) {
			l := new(utils.LogLevel)
			require.NoError(t, l.Set(uppercase))
			assert.Equal(t, level, *l)
		})
	}

	t.Run("unknown log level", func(t *testing.T) {
		l := new(utils.LogLevel)
		require.ErrorIs(t, l.Set("blah"), utils.ErrUnknownLogLevel)
	})
}

func TestLogLevelUnmarshalText(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level "+str, func(t *testing.T) {
			l := new(utils.LogLevel)
			require.NoError(t, l.UnmarshalText([]byte(str)))
			assert.Equal(t, level, *l)
		})
		uppercase := strings.ToUpper(str)
		t.Run("level "+uppercase, func(t *testing.T) {
			l := new(utils.LogLevel)
			require.NoError(t, l.UnmarshalText([]byte(uppercase)))
			assert.Equal(t, level, *l)
		})
	}

	t.Run("unknown log level", func(t *testing.T) {
		l := new(utils.LogLevel)
		require.ErrorIs(t, l.UnmarshalText([]byte("blah")), utils.ErrUnknownLogLevel)
	})
}

func TestLogLevelType(t *testing.T) {
	assert.Equal(t, "LogLevel", new(utils.LogLevel).Type())
}

func TestZapWithColour(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level: "+str, func(t *testing.T) {
			_, err := utils.NewZapLogger(level, true)
			assert.NoError(t, err)
		})
	}
}

func TestZapWithoutColour(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level: "+str, func(t *testing.T) {
			_, err := utils.NewZapLogger(level, false)
			assert.NoError(t, err)
		})
	}
}
