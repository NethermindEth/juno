package junoplugin_test

import (
	"os"
	"path/filepath"
	"testing"

	junoplugin "github.com/NethermindEth/juno/plugin"
	"github.com/stretchr/testify/require"
)

func TestPluginLoad(t *testing.T) {

	// Todo: autogenerate for tests?
	pluginPath := filepath.Join("./", "example.so")

	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Fatalf("Plugin file %s does not exist. Make sure it is built before running tests.", pluginPath)
	}

	junoPlugin := junoplugin.New()
	if err := junoPlugin.Load("./example/example.so"); err != nil {
		t.Fatalf("Failed to load plugin: %v", err)
	}
	require.NotNil(t, junoPlugin.Plugin, "Plugin was not loaded correctly")

	// Todo: call NewBlock in sync etc

}
