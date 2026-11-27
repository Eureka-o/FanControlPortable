package marketplace

import "github.com/Eureka-o/FanControlPortable/internal/plugins"

const (
	DefaultCatalogURL = "https://raw.githubusercontent.com/Eureka-o/FanControl-Plugins/main/catalog.json"

	InstallStatusNotInstalled    = "not_installed"
	InstallStatusInstalled       = "installed"
	InstallStatusUpdateAvailable = "update_available"
	InstallStatusIncompatible    = "incompatible"
)

// Catalog is the root JSON index published by Eureka-o/FanControl-Plugins.
type Catalog struct {
	SchemaVersion int           `json:"schemaVersion"`
	UpdatedAt     string        `json:"updatedAt"`
	Plugins       []PluginEntry `json:"plugins"`
	Themes        []ThemeEntry  `json:"themes"`
}

// PluginEntry describes one official plugin package in catalog.json.
type PluginEntry struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Description     string   `json:"description"`
	Author          string   `json:"author"`
	Category        string   `json:"category"`
	Runtime         string   `json:"runtime"` // inproc-dll | sidecar | frontend-only
	UISlot          string   `json:"uiSlot"`  // sidebar-page | settings-panel | none
	Platform        string   `json:"platform"`
	MinCoreVersion  string   `json:"minCoreVersion,omitempty"`
	ProtocolVersion int      `json:"protocolVersion"`
	HostAPIVersion  int      `json:"hostApiVersion,omitempty"`
	Capabilities    []string `json:"capabilities,omitempty"`
	RestartRequired bool     `json:"restartRequired"`
	DownloadURL     string   `json:"downloadUrl"`
	SHA256          string   `json:"sha256"`
	Size            int64    `json:"size"`
	Homepage        string   `json:"homepage,omitempty"`
	IconURL         string   `json:"iconUrl,omitempty"`
}

// ThemeEntry describes one downloadable UI theme package in catalog.json.
type ThemeEntry struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Version        string `json:"version"`
	Base           string `json:"base"`  // light | dark
	Layer          string `json:"layer"` // basic | advanced
	Author         string `json:"author"`
	Description    string `json:"description"`
	MinCoreVersion string `json:"minCoreVersion,omitempty"`
	PreviewURL     string `json:"previewUrl,omitempty"`
	DownloadURL    string `json:"downloadUrl"`
	SHA256         string `json:"sha256"`
	Size           int64  `json:"size"`
}

// CatalogPluginView merges a marketplace entry with local installation and compatibility state.
type CatalogPluginView struct {
	PluginEntry
	InstalledVersion string `json:"installedVersion,omitempty"`
	InstallStatus    string `json:"installStatus"`
	Compatible       bool   `json:"compatible"`
	Reason           string `json:"reason,omitempty"`
	PendingRestart   bool   `json:"pendingRestart,omitempty"`
}

// CatalogThemeView merges a theme entry with local theme installation state.
type CatalogThemeView struct {
	ThemeEntry
	InstalledVersion string `json:"installedVersion,omitempty"`
	InstalledSource  string `json:"installedSource,omitempty"`
	InstalledBuiltin bool   `json:"installedBuiltin,omitempty"`
	InstallStatus    string `json:"installStatus"`
	Compatible       bool   `json:"compatible"`
	Reason           string `json:"reason,omitempty"`
}

// CatalogView is returned to the GUI marketplace tabs.
type CatalogView struct {
	CatalogURL     string              `json:"catalogUrl"`
	ResolvedSource string              `json:"resolvedSource,omitempty"`
	LatencyMs      int64               `json:"latencyMs,omitempty"`
	UpdatedAt      string              `json:"updatedAt"`
	Plugins        []CatalogPluginView `json:"plugins"`
	Themes         []CatalogThemeView  `json:"themes"`
}

// InstallResult describes the outcome of a plugin/theme install, uninstall, or local zip import.
type InstallResult struct {
	Kind            string `json:"kind"` // plugin | theme
	ID              string `json:"id"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Runtime         string `json:"runtime,omitempty"`
	RestartRequired bool   `json:"restartRequired"`
	Message         string `json:"message,omitempty"`
}

// ProgressEvent is emitted over Wails events during marketplace downloads.
type ProgressEvent struct {
	Kind      string `json:"kind"` // plugin | theme
	ID        string `json:"id"`
	Stage     string `json:"stage"` // downloading | verifying | extracting | done | error
	Percent   int    `json:"percent"`
	Received  int64  `json:"received"`
	Total     int64  `json:"total"`
	Source    string `json:"source,omitempty"`
	LatencyMs int64  `json:"latencyMs,omitempty"`
	Message   string `json:"message,omitempty"`
}

// InstalledThemeMap maps theme ID -> (version, source, builtin) for compatibility display.
type InstalledThemeMeta struct {
	Version string
	Source  string
	Builtin bool
}

type InstalledPluginMap = map[string]plugins.PluginInfo
