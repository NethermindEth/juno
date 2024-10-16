package junoplugin

import (
	"context"
	"fmt"
	"plugin"
	"sync"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type PluginService struct {
	jPlugin JunoPlugin
	wg      sync.WaitGroup
	log     utils.SimpleLogger
}

func New(log utils.SimpleLogger) *PluginService {
	return &PluginService{wg: sync.WaitGroup{}, log: log}
}

func (p *PluginService) WithPlugin(jPlugin JunoPlugin) {
	p.jPlugin = jPlugin
}

func (p *PluginService) Run(ctx context.Context) error {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		<-ctx.Done()
		if err := p.jPlugin.Shutdown(); err != nil {
			p.log.Errorw("Error while calling plugin Shutdown() function", "err", err)
		}
	}()
	p.wg.Wait()
	return nil
}

//go:generate mockgen -destination=../mocks/mock_plugin.go -package=mocks github.com/NethermindEth/juno/plugin JunoPlugin
type JunoPlugin interface {
	Init() error
	Shutdown() error
	NewBlock(block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class) error
	// The state is reverted by applying a write operation with the reverseStateDiff's StorageDiffs, Nonces, and ReplacedClasses,
	// and a delete option with its DeclaredV0Classes, DeclaredV1Classes, and ReplacedClasses.
	RevertBlock(from, to *BlockAndStateUpdate, reverseStateDiff *core.StateDiff) error
}

type BlockAndStateUpdate struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
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

	return pluginInstance, pluginInstance.Init()
}
