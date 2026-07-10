package plugins

import (
	"context"
	"testing"
)

type fakeManagerPlugin struct {
	id     string
	starts int
	stops  int
}

func (p *fakeManagerPlugin) ID() string {
	return p.id
}

func (p *fakeManagerPlugin) Name() string {
	return p.id
}

func (p *fakeManagerPlugin) Start(context.Context) error {
	p.starts++
	return nil
}

func (p *fakeManagerPlugin) Stop() error {
	p.stops++
	return nil
}

func (p *fakeManagerPlugin) Status() Status {
	return Status{ID: p.id, Name: p.id}
}

func TestManagerPluginReturnsRegisteredPlugin(t *testing.T) {
	manager := NewManager(nil)
	plugin := &fakeManagerPlugin{id: "registered"}

	manager.Register(plugin)

	got := manager.Plugin("registered")
	if got != plugin {
		t.Fatalf("Plugin() = %#v, want registered plugin", got)
	}
	if plugin.starts != 0 || plugin.stops != 0 {
		t.Fatalf("Plugin() invoked lifecycle methods: starts=%d stops=%d", plugin.starts, plugin.stops)
	}
}

func TestManagerPluginMissingReturnsNil(t *testing.T) {
	manager := NewManager(nil)

	if got := manager.Plugin("missing"); got != nil {
		t.Fatalf("Plugin() = %#v, want nil", got)
	}
}
