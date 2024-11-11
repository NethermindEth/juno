package plugin

import "context"

// Service provides an abstraction for signalling the plugin to shut down.
type Service struct {
	plugin JunoPlugin
}

func NewService(plugin JunoPlugin) *Service {
	return &Service{
		plugin: plugin,
	}
}

func (p *Service) Run(ctx context.Context) error {
	<-ctx.Done()
	return p.plugin.Shutdown()
}
