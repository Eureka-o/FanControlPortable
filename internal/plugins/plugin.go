package plugins

import (
	"context"

	"github.com/Eureka-o/FanControlPortable/internal/types"
)

const (
	KindBuiltin  = "builtin"
	KindExternal = "external"

	RuntimeBuiltin      = "builtin"
	RuntimeSidecar      = "sidecar"
	RuntimeInprocDLL    = "inproc-dll"
	RuntimeFrontendOnly = "frontend-only"

	UISlotNone          = "none"
	UISlotSidebarPage   = "sidebar-page"
	UISlotSettingsPanel = "settings-panel"

	StateReady          = "ready"
	StateDisabled       = "disabled"
	StateDiscovered     = "discovered"
	StateStarting       = "starting"
	StateSuspending     = "suspending"
	StateSuspended      = "suspended"
	StateRestarting     = "restarting"
	StatePendingRestart = "pending_restart"
	StateUnsupported    = "unsupported"
	StateIncompatible   = "incompatible"
	StateFailed         = "failed"
)

// Plugin is the lifecycle contract for optional background integrations.
type Plugin interface {
	ID() string
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Status() Status
}

// Descriptor describes static metadata for a registered built-in or external plugin.
type Descriptor struct {
	Kind         string   `json:"kind"`
	Runtime      string   `json:"runtime,omitempty"`
	UISlot       string   `json:"uiSlot,omitempty"`
	Category     string   `json:"category,omitempty"`
	Version      string   `json:"version,omitempty"`
	Description  string   `json:"description,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// DescriptorProvider lets a plugin expose richer metadata for the plugin center.
type DescriptorProvider interface {
	Descriptor() Descriptor
}

// AvailabilityChecker reports whether a registered plugin supports the current host.
type AvailabilityChecker interface {
	Available() (supported bool, reason string)
}

// FanDataObserver receives cooler fan telemetry updates without coupling plugins to coreapp.
type FanDataObserver interface {
	OnFanData(fanData *types.FanData)
	OnDeviceDisconnected()
}

// SettingOption describes one selectable choice for a "select" plugin setting field.
type SettingOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// SettingField declares a Gopeed-style plugin configuration item rendered natively in UI.
type SettingField struct {
	Key         string          `json:"key"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type"` // boolean | string | number | select
	Default     any             `json:"default,omitempty"`
	Options     []SettingOption `json:"options,omitempty"`
	Min         *float64        `json:"min,omitempty"`
	Max         *float64        `json:"max,omitempty"`
	Step        *float64        `json:"step,omitempty"`
}

// Status describes the runtime state of a plugin.
type Status struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Running   bool   `json:"running"`
	LastError string `json:"lastError,omitempty"`
}

// PluginInfo is the unified view exposed to Core IPC, diagnostics, and the GUI plugin center.
type PluginInfo struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Kind            string         `json:"kind"`
	Runtime         string         `json:"runtime,omitempty"`
	UISlot          string         `json:"uiSlot,omitempty"`
	Category        string         `json:"category,omitempty"`
	Version         string         `json:"version,omitempty"`
	Description     string         `json:"description,omitempty"`
	Capabilities    []string       `json:"capabilities,omitempty"`
	Frontend        string         `json:"frontend,omitempty"`
	Style           string         `json:"style,omitempty"`
	Icon            string         `json:"icon,omitempty"`
	State           string         `json:"state"`
	Enabled         bool           `json:"enabled"`
	Running         bool           `json:"running"`
	Supported       bool           `json:"supported"`
	RestartRequired bool           `json:"restartRequired,omitempty"`
	PendingRestart  bool           `json:"pendingRestart,omitempty"`
	DevMode         bool           `json:"devMode,omitempty"`
	Reason          string         `json:"reason,omitempty"`
	LastError       string         `json:"lastError,omitempty"`
	Path            string         `json:"path,omitempty"`
	Settings        []SettingField `json:"settings,omitempty"`
}
