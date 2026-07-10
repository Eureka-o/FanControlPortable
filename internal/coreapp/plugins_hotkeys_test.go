package coreapp

import (
	"context"
	"sync"
	"testing"

	"github.com/TIANLI0/THRM/internal/plugins"
	"github.com/TIANLI0/THRM/internal/plugins/fnqpowermode"
	"github.com/TIANLI0/THRM/internal/types"
)

type pluginConfigLifecyclePlugin struct {
	mu sync.Mutex

	id      string
	starts  int
	stops   int
	running bool
}

func (p *pluginConfigLifecyclePlugin) ID() string   { return p.id }
func (p *pluginConfigLifecyclePlugin) Name() string { return p.id }

func (p *pluginConfigLifecyclePlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.starts++
	p.running = true
	return nil
}

func (p *pluginConfigLifecyclePlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stops++
	p.running = false
	return nil
}

func (p *pluginConfigLifecyclePlugin) Status() plugins.Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	return plugins.Status{ID: p.id, Name: p.id, Running: p.running}
}

func (p *pluginConfigLifecyclePlugin) snapshot() (starts, stops int, running bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.starts, p.stops, p.running
}

func TestApplyPluginConfigLeavesUnrelatedRuntimePluginsUntouched(t *testing.T) {
	other := &pluginConfigLifecyclePlugin{id: "sample-device"}
	manager := plugins.NewManager(nil)
	manager.Register(other)
	app := &CoreApp{pluginManager: manager}

	app.applyPluginConfig(types.AppConfig{})

	starts, stops, running := other.snapshot()
	if starts != 0 || stops != 0 || running {
		t.Fatalf("unrelated plugin lifecycle = starts %d stops %d running %v, want untouched",
			starts, stops, running)
	}
}

func TestApplyPluginConfigLegionFnQBehaviorUnchanged(t *testing.T) {
	fnq := &pluginConfigLifecyclePlugin{id: fnqpowermode.PluginID}
	manager := plugins.NewManager(nil)
	manager.Register(fnq)
	app := &CoreApp{pluginManager: manager}
	cfg := types.AppConfig{LegionFnQ: types.LegionFnQConfig{Enabled: true}}

	app.applyPluginConfig(cfg)
	starts, stops, running := fnq.snapshot()
	if starts != 0 || stops != 0 {
		t.Fatalf("unsupported Legion Fn+Q lifecycle = starts %d stops %d, want untouched", starts, stops)
	}

	app.legionFnQSupported.Store(true)
	app.applyPluginConfig(cfg)
	starts, stops, running = fnq.snapshot()
	if starts != 1 || stops != 0 || !running {
		t.Fatalf("enabled Legion Fn+Q lifecycle = starts %d stops %d running %v, want one start",
			starts, stops, running)
	}

	cfg.LegionFnQ.Enabled = false
	app.applyPluginConfig(cfg)
	starts, stops, running = fnq.snapshot()
	if starts != 1 || stops != 1 || running {
		t.Fatalf("disabled Legion Fn+Q lifecycle = starts %d stops %d running %v, want one stop",
			starts, stops, running)
	}
}
