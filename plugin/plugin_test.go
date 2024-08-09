package junoplugin_test

import (
	"os"
	"testing"

	junoplugin "github.com/NethermindEth/juno/plugin"
	"github.com/stretchr/testify/require"
)

func TestPluginLoad(t *testing.T) {
	pluginSOPath := "../build/plugin.so"
	_, err := os.Stat(pluginSOPath)
	require.False(t, os.IsNotExist(err))
	junoPluginInstance, err := junoplugin.Load(pluginSOPath)
	require.Nil(t, err)
	require.NotNil(t, junoPluginInstance, "Plugin was not loaded correctly")
	require.Nil(t, junoPluginInstance.NewBlock(nil, nil, nil))
	require.Nil(t, junoPluginInstance.RevertBlock(nil, nil, nil))
}
