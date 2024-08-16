package junoplugin

import (
	"fmt"
	"plugin"

	junopluginsync "github.com/NethermindEth/juno/plugin/sync"
	"github.com/NethermindEth/juno/rpc"
)

//go:generate mockgen -destination=../mocks/mock_plugin.go -package=mocks github.com/NethermindEth/juno/plugin JunoPlugin
type JunoPlugin interface {
	Init(rpcHandler *rpc.Handler) error
	ShutDown() // Todo: Currently this function will never be called.
	junopluginsync.JunoPlugin
}

func Load(pluginPath string) (JunoPlugin, error) {
	plug, err := plugin.Open(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("error loading plugin .so file: %w", err)
	}

	symPlugin, err := plug.Lookup("JunoPluginInstance")
	if err != nil {
		return nil, fmt.Errorf("error looking up PluginInstance: %w", err)
	}

	pluginInstance, ok := symPlugin.(JunoPlugin)
	if !ok {
		return nil, fmt.Errorf("the plugin does not staisfy the required interface")
	}

	return pluginInstance, nil
}
