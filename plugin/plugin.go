package junoplugin

import (
	"fmt"
	"plugin"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type IPlugin interface {
	Init() error
	Shutdown() error
	NewBlock(block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class) error
	RevertBlock(block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class) error
}

func New() Plugin {
	return Plugin{}
}

type Plugin struct {
	Plugin IPlugin
}

func (l *Plugin) Load(pluginPath string) error {
	plug, err := plugin.Open(pluginPath)
	if err != nil {
		return fmt.Errorf("error loading plugin: %w", err)
	}

	symPlugin, err := plug.Lookup("PluginInstance")
	if err != nil {
		return fmt.Errorf("error looking up PluginInstance: %w", err)
	}

	pluginInstance, ok := symPlugin.(IPlugin)
	if !ok {
		return fmt.Errorf("unexpected type from module symbol")
	}

	l.Plugin = pluginInstance

	return l.Plugin.Init()
}
