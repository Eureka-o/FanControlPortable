//go:build !windows

package fnqpowermode

import (
	"context"

	"github.com/Eureka-o/FanControlPortable/internal/plugins"
)

type Plugin struct{}

func New(options Options) *Plugin {
	return &Plugin{}
}

func (p *Plugin) ID() string {
	return PluginID
}

func (p *Plugin) Name() string {
	return PluginName
}

func (p *Plugin) Descriptor() plugins.Descriptor {
	return plugins.Descriptor{
		Kind:         plugins.KindBuiltin,
		Category:     "hardware",
		Version:      "1.0.0",
		Description:  "Listen for Lenovo Legion Fn+Q power mode changes",
		Capabilities: []string{"power-mode", "fan-takeover"},
	}
}

func (p *Plugin) Start(ctx context.Context) error {
	return nil
}

func (p *Plugin) Stop() error {
	return nil
}

func (p *Plugin) Status() plugins.Status {
	return plugins.Status{ID: p.ID(), Name: p.Name(), Running: false, LastError: "unsupported platform"}
}
