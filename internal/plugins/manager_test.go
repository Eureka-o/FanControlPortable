package plugins

import (
	"context"
	"sync"
	"testing"
	"time"
)

type blockingPlugin struct {
	id            string
	startEntered  chan struct{}
	startRelease  chan struct{}
	statusEntered chan struct{}
	statusRelease chan struct{}
	onceStart     sync.Once
	onceStatus    sync.Once
}

func (p *blockingPlugin) ID() string   { return p.id }
func (p *blockingPlugin) Name() string { return p.id }

func (p *blockingPlugin) Start(context.Context) error {
	p.onceStart.Do(func() { close(p.startEntered) })
	<-p.startRelease
	return nil
}

func (p *blockingPlugin) Stop() error { return nil }

func (p *blockingPlugin) Status() Status {
	p.onceStatus.Do(func() { close(p.statusEntered) })
	<-p.statusRelease
	return Status{ID: p.id, Name: p.id, Running: true}
}

type idlePlugin struct{ id string }

func (p *idlePlugin) ID() string                  { return p.id }
func (p *idlePlugin) Name() string                { return p.id }
func (p *idlePlugin) Start(context.Context) error { return nil }
func (p *idlePlugin) Stop() error                 { return nil }
func (p *idlePlugin) Status() Status              { return Status{ID: p.id, Name: p.id} }

func TestStartDoesNotHoldManagerLock(t *testing.T) {
	manager := NewManager(nil)
	plugin := &blockingPlugin{
		id:           "blocking",
		startEntered: make(chan struct{}),
		startRelease: make(chan struct{}),
	}
	manager.Register(plugin)

	startDone := make(chan struct{})
	go func() {
		_ = manager.Start(plugin.id)
		close(startDone)
	}()
	<-plugin.startEntered

	registerDone := make(chan struct{})
	go func() {
		manager.Register(&idlePlugin{id: "second"})
		close(registerDone)
	}()
	select {
	case <-registerDone:
	case <-time.After(time.Second):
		t.Fatal("Register blocked behind plugin Start")
	}

	close(plugin.startRelease)
	<-startDone
}

func TestStatusesDoesNotHoldManagerLock(t *testing.T) {
	manager := NewManager(nil)
	plugin := &blockingPlugin{
		id:            "blocking",
		statusEntered: make(chan struct{}),
		statusRelease: make(chan struct{}),
	}
	manager.Register(plugin)

	statusesDone := make(chan struct{})
	go func() {
		_ = manager.Statuses()
		close(statusesDone)
	}()
	<-plugin.statusEntered

	registerDone := make(chan struct{})
	go func() {
		manager.Register(&idlePlugin{id: "second"})
		close(registerDone)
	}()
	select {
	case <-registerDone:
	case <-time.After(time.Second):
		t.Fatal("Register blocked behind plugin Status")
	}

	close(plugin.statusRelease)
	<-statusesDone
}

func TestRegisterIgnoresDuplicateID(t *testing.T) {
	manager := NewManager(nil)
	manager.Register(&idlePlugin{id: "same"})
	manager.Register(&idlePlugin{id: "same"})

	if got := len(manager.Statuses()); got != 1 {
		t.Fatalf("Statuses length = %d, want 1", got)
	}
}
