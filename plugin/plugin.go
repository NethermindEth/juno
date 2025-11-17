package plugin

import (
	"fmt"
	stdplugin "plugin"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

//go:generate mockgen -destination=../mocks/mock_plugin.go -package=mocks github.com/NethermindEth/juno/plugin JunoPlugin
type JunoPlugin interface {
	Init() error
	Shutdown() error
	NewBlock(
		block *core.Block,
		stateUpdate *core.StateUpdate,
		newClasses map[felt.Felt]core.ClassDefinition,
	) error
	// The state is reverted by applying a write operation with the reverseStateDiff's StorageDiffs, Nonces, and ReplacedClasses,
	// and a delete option with its DeclaredV0Classes, DeclaredV1Classes, and ReplacedClasses.
	RevertBlock(from, to *BlockAndStateUpdate, reverseStateDiff *core.StateDiff) error
}

type BlockAndStateUpdate struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
}

func Load(pluginPath string) (JunoPlugin, error) {
	plug, err := stdplugin.Open(pluginPath)
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

	return pluginInstance, pluginInstance.Init()
}
