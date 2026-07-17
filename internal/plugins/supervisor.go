package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

type SupervisorOptions struct {
	Logger           types.Logger
	DataRoot         string
	CommandFactory   CommandFactory
	HandshakeTimeout time.Duration
	RequestTimeout   time.Duration
	StopTimeout      time.Duration
	OnStatus         func(RuntimeStatus)
	OnEvent          func(RuntimeEvent)
}

type Supervisor struct {
	options SupervisorOptions

	mutex    sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	runtimes map[string]*ProcessRuntime
}

func NewSupervisor(options SupervisorOptions) *Supervisor {
	options.DataRoot = filepath.Clean(options.DataRoot)
	return &Supervisor{
		options:  options,
		runtimes: make(map[string]*ProcessRuntime),
	}
}

func (supervisor *Supervisor) Start(parent context.Context) {
	supervisor.mutex.Lock()
	defer supervisor.mutex.Unlock()
	if supervisor.ctx != nil {
		return
	}
	if parent == nil {
		parent = context.Background()
	}
	supervisor.ctx, supervisor.cancel = context.WithCancel(parent)
}

func (supervisor *Supervisor) Sync(specs []RuntimeSpec) {
	desired := make(map[string]RuntimeSpec, len(specs))
	for _, spec := range specs {
		desired[spec.ID] = cloneRuntimeSpec(spec)
	}

	supervisor.mutex.Lock()
	toStop := make([]*ProcessRuntime, 0)
	for id, runtime := range supervisor.runtimes {
		spec, keep := desired[id]
		if keep && runtime.MatchesSpec(spec) {
			delete(desired, id)
			continue
		}
		delete(supervisor.runtimes, id)
		toStop = append(toStop, runtime)
	}
	for id, spec := range desired {
		supervisor.runtimes[id] = NewProcessRuntime(spec, ProcessRuntimeOptions{
			Logger:           supervisor.options.Logger,
			DataDir:          filepath.Join(supervisor.options.DataRoot, id),
			CommandFactory:   supervisor.options.CommandFactory,
			HandshakeTimeout: supervisor.options.HandshakeTimeout,
			RequestTimeout:   supervisor.options.RequestTimeout,
			StopTimeout:      supervisor.options.StopTimeout,
			OnStatus:         supervisor.options.OnStatus,
			OnEvent:          supervisor.options.OnEvent,
		})
	}
	supervisor.mutex.Unlock()

	for _, runtime := range toStop {
		_ = runtime.Stop("refresh")
	}
}

func (supervisor *Supervisor) Enable(id string) error {
	supervisor.mutex.RLock()
	runtime := supervisor.runtimes[id]
	ctx := supervisor.ctx
	supervisor.mutex.RUnlock()
	if runtime == nil {
		return fmt.Errorf("plugin runtime not found: %s", id)
	}
	if ctx == nil {
		supervisor.Start(context.Background())
		supervisor.mutex.RLock()
		ctx = supervisor.ctx
		supervisor.mutex.RUnlock()
	}
	return runtime.Start(ctx)
}

func (supervisor *Supervisor) Disable(id, reason string) error {
	supervisor.mutex.RLock()
	runtime := supervisor.runtimes[id]
	supervisor.mutex.RUnlock()
	if runtime == nil {
		return fmt.Errorf("plugin runtime not found: %s", id)
	}
	return runtime.Stop(reason)
}

func (supervisor *Supervisor) Remove(id, reason string) error {
	supervisor.mutex.Lock()
	runtime := supervisor.runtimes[id]
	delete(supervisor.runtimes, id)
	supervisor.mutex.Unlock()
	if runtime == nil {
		return nil
	}
	return runtime.Stop(reason)
}

func (supervisor *Supervisor) StartEnabled(enabled map[string]bool) error {
	ids := enabledPluginIDs(enabled)
	var firstErr error
	for _, id := range ids {
		if err := supervisor.Enable(id); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", id, err)
		}
	}
	return firstErr
}

func enabledPluginIDs(enabled map[string]bool) []string {
	ids := make([]string, 0, len(enabled))
	for id, value := range enabled {
		if value {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func (supervisor *Supervisor) SuspendAll() error {
	runtimes := supervisor.runtimeSnapshot()
	var firstErr error
	for _, runtime := range runtimes {
		state := runtime.Status().State
		if state == CatalogStateDisabled || state == CatalogStateSuspended || state == CatalogStateFailed {
			continue
		}
		if err := runtime.Stop("suspend"); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", runtime.Spec().ID, err)
		}
	}
	return firstErr
}

func (supervisor *Supervisor) ResumeEnabled(enabled map[string]bool) error {
	var firstErr error
	for _, id := range enabledPluginIDs(enabled) {
		status, ok := supervisor.Status(id)
		if !ok || status.State != CatalogStateSuspended {
			continue
		}
		if err := supervisor.Enable(id); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", id, err)
		}
	}
	return firstErr
}

func (supervisor *Supervisor) StopAll(reason string) error {
	runtimes := supervisor.runtimeSnapshot()
	var firstErr error
	for _, runtime := range runtimes {
		if err := runtime.Stop(reason); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", runtime.Spec().ID, err)
		}
	}
	supervisor.mutex.Lock()
	cancel := supervisor.cancel
	supervisor.ctx = nil
	supervisor.cancel = nil
	supervisor.mutex.Unlock()
	if cancel != nil {
		cancel()
	}
	return firstErr
}

func (supervisor *Supervisor) Invoke(ctx context.Context, id, method string, payload json.RawMessage) (json.RawMessage, error) {
	supervisor.mutex.RLock()
	runtime := supervisor.runtimes[id]
	supervisor.mutex.RUnlock()
	if runtime == nil {
		return nil, fmt.Errorf("plugin runtime not found: %s", id)
	}
	return runtime.Invoke(ctx, method, payload)
}

func (supervisor *Supervisor) SubmitTelemetry(snapshot TelemetrySnapshot) {
	for _, runtime := range supervisor.runtimeSnapshot() {
		runtime.SubmitTelemetry(snapshot)
	}
}

func (supervisor *Supervisor) HasTelemetryTargets() bool {
	supervisor.mutex.RLock()
	defer supervisor.mutex.RUnlock()
	for _, runtime := range supervisor.runtimes {
		if runtime.AcceptsTelemetry() {
			return true
		}
	}
	return false
}

func (supervisor *Supervisor) Status(id string) (RuntimeStatus, bool) {
	supervisor.mutex.RLock()
	runtime := supervisor.runtimes[id]
	supervisor.mutex.RUnlock()
	if runtime == nil {
		return RuntimeStatus{}, false
	}
	return runtime.Status(), true
}

func (supervisor *Supervisor) runtimeSnapshot() []*ProcessRuntime {
	supervisor.mutex.RLock()
	ids := make([]string, 0, len(supervisor.runtimes))
	for id := range supervisor.runtimes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	runtimes := make([]*ProcessRuntime, 0, len(ids))
	for _, id := range ids {
		runtimes = append(runtimes, supervisor.runtimes[id])
	}
	supervisor.mutex.RUnlock()
	return runtimes
}
