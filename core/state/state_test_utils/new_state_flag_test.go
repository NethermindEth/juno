package statetestutils_test

import (
	"os"
	"testing"

	statetestutils "github.com/NethermindEth/juno/core/state/state_test_utils"
	"github.com/stretchr/testify/assert"
)

func TestUseNewState(t *testing.T) {
	t.Run("default false", func(t *testing.T) {
		os.Unsetenv("USE_NEW_STATE")
		assert.False(t, statetestutils.UseNewState())
	})

	t.Run("env true", func(t *testing.T) {
		os.Setenv("USE_NEW_STATE", "true")
		assert.True(t, statetestutils.UseNewState())
	})

	t.Run("env false", func(t *testing.T) {
		os.Setenv("USE_NEW_STATE", "false")
		assert.False(t, statetestutils.UseNewState())
	})
}
