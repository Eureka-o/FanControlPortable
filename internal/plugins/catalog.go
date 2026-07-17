package plugins

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/version"
)

const (
	ManifestFileName  = "plugin.json"
	manifestSizeLimit = 64 * 1024
	pluginAssetLimit  = 8 * 1024 * 1024
	ProtocolVersionV1 = 1
	HostAPIVersionV1  = 1
)

type CatalogState string

const (
	CatalogStateDiscovered   CatalogState = "discovered"
	CatalogStateDisabled     CatalogState = "disabled"
	CatalogStateStarting     CatalogState = "starting"
	CatalogStateReady        CatalogState = "ready"
	CatalogStateSuspending   CatalogState = "suspending"
	CatalogStateSuspended    CatalogState = "suspended"
	CatalogStateRestarting   CatalogState = "restarting"
	CatalogStateUnsupported  CatalogState = "unsupported"
	CatalogStateFailed       CatalogState = "failed"
	CatalogStateInvalid      CatalogState = "invalid"
	CatalogStateIncompatible CatalogState = "incompatible"
)

type PageManifest struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Icon      string `json:"icon"`
	IconAsset string `json:"iconAsset,omitempty"`
	Order     int    `json:"order"`
}

type Manifest struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Description     string       `json:"description,omitempty"`
	Version         string       `json:"version"`
	Platform        string       `json:"platform"`
	MinCoreVersion  string       `json:"minCoreVersion,omitempty"`
	ProtocolVersion int          `json:"protocolVersion"`
	Backend         string       `json:"backend"`
	Frontend        string       `json:"frontend"`
	Style           string       `json:"style,omitempty"`
	HostAPIVersion  int          `json:"hostApiVersion"`
	Capabilities    []string     `json:"capabilities,omitempty"`
	TelemetryInputs []string     `json:"telemetryInputs,omitempty"`
	Page            PageManifest `json:"page"`
}

type CatalogEntry struct {
	ID                  string       `json:"id"`
	Name                string       `json:"name"`
	Description         string       `json:"description,omitempty"`
	Version             string       `json:"version,omitempty"`
	Platform            string       `json:"platform,omitempty"`
	Enabled             bool         `json:"enabled"`
	State               CatalogState `json:"state"`
	LastError           string       `json:"lastError,omitempty"`
	Capabilities        []string     `json:"capabilities,omitempty"`
	RuntimeCapabilities []string     `json:"runtimeCapabilities,omitempty"`
	TelemetryInputs     []string     `json:"telemetryInputs,omitempty"`
	Frontend            string       `json:"frontend,omitempty"`
	Style               string       `json:"style,omitempty"`
	Page                PageManifest `json:"page"`

	pluginDir   string
	backendPath string
}

type CatalogSnapshot struct {
	Revision uint64         `json:"revision"`
	Plugins  []CatalogEntry `json:"plugins"`
	Error    string         `json:"error,omitempty"`
}

type PluginAsset struct {
	Name        string
	Version     string
	ContentType string
	ModTime     time.Time
	Data        []byte
}

// RuntimeSpec contains only paths and metadata that passed manifest validation.
type RuntimeSpec struct {
	ID              string
	Name            string
	Version         string
	PluginDir       string
	BackendPath     string
	Capabilities    []string
	TelemetryInputs []string
}

type Catalog struct {
	rootDir     string
	coreVersion string

	mutex       sync.RWMutex
	initialized bool
	revision    uint64
	plugins     []CatalogEntry
	lastError   string
	enabled     map[string]bool
}

func NewCatalog(rootDir, coreVersion string) *Catalog {
	return &Catalog{
		rootDir:     filepath.Clean(rootDir),
		coreVersion: strings.TrimSpace(coreVersion),
		plugins:     []CatalogEntry{},
		enabled:     map[string]bool{},
	}
}

func (c *Catalog) Refresh() CatalogSnapshot {
	plugins, scanErr := scanCatalog(c.rootDir, c.coreVersion)
	errorText := ""
	if scanErr != nil {
		errorText = scanErr.Error()
	}

	c.mutex.Lock()
	applyEnabledState(plugins, c.enabled)
	mergeRuntimeState(plugins, c.plugins)
	if !c.initialized || !reflect.DeepEqual(c.plugins, plugins) || c.lastError != errorText {
		c.revision++
		c.initialized = true
		c.plugins = plugins
		c.lastError = errorText
	}
	snapshot := c.snapshotLocked()
	c.mutex.Unlock()
	return snapshot
}

func (c *Catalog) SetEnabledState(enabled map[string]bool) CatalogSnapshot {
	c.mutex.Lock()
	c.enabled = cloneEnabledMap(enabled)
	plugins := append([]CatalogEntry(nil), c.plugins...)
	applyEnabledState(plugins, c.enabled)
	if !reflect.DeepEqual(c.plugins, plugins) {
		c.revision++
		c.plugins = plugins
	}
	snapshot := c.snapshotLocked()
	c.mutex.Unlock()
	return snapshot
}

func (c *Catalog) SetEnabled(id string, enabled bool) (CatalogSnapshot, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	index := c.findLocked(id)
	if index < 0 {
		return c.snapshotLocked(), fmt.Errorf("plugin not found: %s", id)
	}
	if c.plugins[index].State == CatalogStateInvalid || c.plugins[index].State == CatalogStateIncompatible {
		return c.snapshotLocked(), fmt.Errorf("plugin %s cannot be enabled in state %s", id, c.plugins[index].State)
	}
	if c.plugins[index].Enabled == enabled {
		return c.snapshotLocked(), nil
	}
	if enabled {
		c.enabled[id] = true
		c.plugins[index].State = CatalogStateDiscovered
	} else {
		delete(c.enabled, id)
		c.plugins[index].State = CatalogStateDisabled
	}
	c.plugins[index].Enabled = enabled
	if enabled {
		c.plugins[index].LastError = ""
	}
	c.plugins[index].RuntimeCapabilities = nil
	c.revision++
	return c.snapshotLocked(), nil
}

func (c *Catalog) SetRuntimeState(id string, state CatalogState, lastError string, capabilities []string) (CatalogSnapshot, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	index := c.findLocked(id)
	if index < 0 || c.plugins[index].State == CatalogStateInvalid || c.plugins[index].State == CatalogStateIncompatible {
		return c.snapshotLocked(), false
	}
	if !c.plugins[index].Enabled && state != CatalogStateDisabled {
		return c.snapshotLocked(), false
	}
	nextCapabilities := append([]string(nil), capabilities...)
	if c.plugins[index].State == state && c.plugins[index].LastError == lastError && reflect.DeepEqual(c.plugins[index].RuntimeCapabilities, nextCapabilities) {
		return c.snapshotLocked(), false
	}
	c.plugins[index].State = state
	c.plugins[index].LastError = lastError
	c.plugins[index].RuntimeCapabilities = nextCapabilities
	c.revision++
	return c.snapshotLocked(), true
}

func (c *Catalog) Entry(id string) (CatalogEntry, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	index := c.findLocked(id)
	if index < 0 {
		return CatalogEntry{}, false
	}
	return c.plugins[index], true
}

func (c *Catalog) ReadAsset(id, relativePath string) (PluginAsset, error) {
	if !validPluginID(id) {
		return PluginAsset{}, fmt.Errorf("invalid plugin id")
	}
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" || filepath.IsAbs(relativePath) || filepath.VolumeName(relativePath) != "" {
		return PluginAsset{}, fmt.Errorf("asset must be a relative file path")
	}
	cleaned := filepath.Clean(filepath.FromSlash(relativePath))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return PluginAsset{}, fmt.Errorf("asset escapes the plugin directory")
	}

	entry, ok := c.Entry(id)
	if !ok || entry.pluginDir == "" {
		// Plugins installed after GUI startup should be available without restarting the app.
		c.Refresh()
		entry, ok = c.Entry(id)
	}
	if !ok || entry.pluginDir == "" || entry.State == CatalogStateInvalid || entry.State == CatalogStateIncompatible {
		return PluginAsset{}, fmt.Errorf("plugin assets are not available")
	}

	contentType, ok := pluginAssetContentType(filepath.Ext(cleaned))
	if !ok {
		return PluginAsset{}, fmt.Errorf("plugin asset type is not allowed")
	}
	fullPath := filepath.Join(entry.pluginDir, cleaned)
	if !pathInside(fullPath, entry.pluginDir) {
		return PluginAsset{}, fmt.Errorf("asset escapes the plugin directory")
	}
	info, err := os.Lstat(fullPath)
	if err != nil {
		return PluginAsset{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return PluginAsset{}, fmt.Errorf("plugin asset must be a regular non-symlink file")
	}
	if info.Size() > pluginAssetLimit {
		return PluginAsset{}, fmt.Errorf("plugin asset exceeds %d bytes", pluginAssetLimit)
	}
	resolvedPluginDir, err := filepath.EvalSymlinks(entry.pluginDir)
	if err != nil {
		return PluginAsset{}, err
	}
	resolvedPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return PluginAsset{}, err
	}
	if !pathInside(resolvedPath, resolvedPluginDir) {
		return PluginAsset{}, fmt.Errorf("plugin asset resolves outside the plugin directory")
	}
	data, err := readLimitedAsset(fullPath, pluginAssetLimit)
	if err != nil {
		return PluginAsset{}, err
	}
	return PluginAsset{
		Name:        filepath.Base(fullPath),
		Version:     entry.Version,
		ContentType: contentType,
		ModTime:     info.ModTime(),
		Data:        data,
	}, nil
}

func (c *Catalog) Delete(id string) (CatalogSnapshot, error) {
	entry, ok := c.Entry(id)
	if !ok {
		return c.Snapshot(), fmt.Errorf("plugin not found: %s", id)
	}
	if entry.Enabled {
		return c.Snapshot(), fmt.Errorf("disable plugin %s before deleting it", id)
	}
	if !validPluginID(id) {
		return c.Snapshot(), fmt.Errorf("invalid plugin id")
	}
	pluginDir := filepath.Join(c.rootDir, id)
	if err := removeManagedDirectory(c.rootDir, pluginDir); err != nil {
		return c.Snapshot(), err
	}
	c.mutex.Lock()
	delete(c.enabled, id)
	c.mutex.Unlock()
	return c.Refresh(), nil
}

func (c *Catalog) findLocked(id string) int {
	for index := range c.plugins {
		if c.plugins[index].ID == id {
			return index
		}
	}
	return -1
}

func (c *Catalog) Snapshot() CatalogSnapshot {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.snapshotLocked()
}

func (c *Catalog) RuntimeSpecs() []RuntimeSpec {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	specs := make([]RuntimeSpec, 0, len(c.plugins))
	for _, plugin := range c.plugins {
		if plugin.backendPath == "" || plugin.State == CatalogStateInvalid || plugin.State == CatalogStateIncompatible {
			continue
		}
		specs = append(specs, RuntimeSpec{
			ID:              plugin.ID,
			Name:            plugin.Name,
			Version:         plugin.Version,
			PluginDir:       plugin.pluginDir,
			BackendPath:     plugin.backendPath,
			Capabilities:    append([]string(nil), plugin.Capabilities...),
			TelemetryInputs: append([]string(nil), plugin.TelemetryInputs...),
		})
	}
	return specs
}

func (c *Catalog) snapshotLocked() CatalogSnapshot {
	plugins := make([]CatalogEntry, len(c.plugins))
	for i, plugin := range c.plugins {
		plugin.Capabilities = append([]string(nil), plugin.Capabilities...)
		plugin.RuntimeCapabilities = append([]string(nil), plugin.RuntimeCapabilities...)
		plugin.TelemetryInputs = append([]string(nil), plugin.TelemetryInputs...)
		plugins[i] = plugin
	}
	return CatalogSnapshot{Revision: c.revision, Plugins: plugins, Error: c.lastError}
}

func cloneEnabledMap(input map[string]bool) map[string]bool {
	output := make(map[string]bool, len(input))
	for id, enabled := range input {
		if enabled {
			output[id] = true
		}
	}
	return output
}

func applyEnabledState(plugins []CatalogEntry, enabled map[string]bool) {
	for index := range plugins {
		if plugins[index].State == CatalogStateInvalid || plugins[index].State == CatalogStateIncompatible {
			plugins[index].Enabled = false
			continue
		}
		plugins[index].Enabled = enabled[plugins[index].ID]
		if !plugins[index].Enabled {
			plugins[index].State = CatalogStateDisabled
			plugins[index].RuntimeCapabilities = nil
		} else if plugins[index].State == CatalogStateDisabled {
			plugins[index].State = CatalogStateDiscovered
		}
	}
}

func mergeRuntimeState(plugins, previous []CatalogEntry) {
	previousByID := make(map[string]CatalogEntry, len(previous))
	for _, plugin := range previous {
		previousByID[plugin.ID] = plugin
	}
	for index := range plugins {
		current := &plugins[index]
		old, ok := previousByID[current.ID]
		if !ok || !current.Enabled || !old.Enabled || current.Version != old.Version || !isRuntimeCatalogState(old.State) {
			continue
		}
		current.State = old.State
		current.LastError = old.LastError
		current.RuntimeCapabilities = append([]string(nil), old.RuntimeCapabilities...)
	}
}

func isRuntimeCatalogState(state CatalogState) bool {
	switch state {
	case CatalogStateStarting, CatalogStateReady, CatalogStateSuspending, CatalogStateSuspended,
		CatalogStateRestarting, CatalogStateUnsupported, CatalogStateFailed:
		return true
	default:
		return false
	}
}

func ResetPluginData(dataRoot, id string) error {
	if !validPluginID(id) {
		return fmt.Errorf("invalid plugin id")
	}
	dataDir := filepath.Join(dataRoot, id)
	if _, err := os.Lstat(dataDir); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return removeManagedDirectory(dataRoot, dataDir)
}

func removeManagedDirectory(root, target string) error {
	if !pathInside(target, root) || filepath.Clean(target) == filepath.Clean(root) {
		return fmt.Errorf("managed directory escapes its root")
	}
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("managed path must be a real directory")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return err
	}
	if !pathInside(resolvedTarget, resolvedRoot) {
		return fmt.Errorf("managed directory resolves outside its root")
	}
	return os.RemoveAll(target)
}

func scanCatalog(rootDir, coreVersion string) ([]CatalogEntry, error) {
	entries, err := os.ReadDir(rootDir)
	if os.IsNotExist(err) {
		return []CatalogEntry{}, nil
	}
	if err != nil {
		return []CatalogEntry{}, fmt.Errorf("scan plugins directory: %w", err)
	}

	plugins := make([]CatalogEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		pluginDir := filepath.Join(rootDir, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			plugins = append(plugins, invalidCatalogEntry(entry.Name(), "plugin entry must be a real directory"))
			continue
		}
		plugins = append(plugins, readCatalogEntry(pluginDir, entry.Name(), coreVersion))
	}

	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Page.Order != plugins[j].Page.Order {
			return plugins[i].Page.Order < plugins[j].Page.Order
		}
		if plugins[i].Name != plugins[j].Name {
			return plugins[i].Name < plugins[j].Name
		}
		return plugins[i].ID < plugins[j].ID
	})
	return plugins, nil
}

func readCatalogEntry(pluginDir, folderName, coreVersion string) CatalogEntry {
	manifestPath := filepath.Join(pluginDir, ManifestFileName)
	manifestData, err := readLimitedFile(manifestPath, manifestSizeLimit)
	if err != nil {
		return invalidCatalogEntry(folderName, fmt.Sprintf("read plugin manifest: %v", err))
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return invalidCatalogEntry(folderName, fmt.Sprintf("parse plugin manifest: %v", err))
	}
	if err := validateManifest(pluginDir, folderName, &manifest); err != nil {
		entry := catalogEntryFromManifest(manifest)
		// The directory is the only trusted identity for an invalid manifest.
		entry.ID = folderName
		entry.State = CatalogStateInvalid
		entry.LastError = err.Error()
		return entry
	}

	entry := catalogEntryFromManifest(manifest)
	entry.State = CatalogStateDiscovered
	if manifest.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		entry.State = CatalogStateIncompatible
		entry.LastError = fmt.Sprintf("plugin platform %s does not match %s-%s", manifest.Platform, runtime.GOOS, runtime.GOARCH)
	} else if manifest.ProtocolVersion != ProtocolVersionV1 {
		entry.State = CatalogStateIncompatible
		entry.LastError = fmt.Sprintf("plugin protocol %d is not supported", manifest.ProtocolVersion)
	} else if manifest.HostAPIVersion != HostAPIVersionV1 {
		entry.State = CatalogStateIncompatible
		entry.LastError = fmt.Sprintf("plugin host API %d is not supported", manifest.HostAPIVersion)
	} else if coreVersion != "" && coreVersion != "dev" && manifest.MinCoreVersion != "" && version.IsNewer(coreVersion, manifest.MinCoreVersion) {
		entry.State = CatalogStateIncompatible
		entry.LastError = fmt.Sprintf("plugin requires FanControl %s or newer", manifest.MinCoreVersion)
	}
	if entry.State == CatalogStateDiscovered {
		entry.pluginDir = pluginDir
		entry.backendPath = filepath.Join(pluginDir, filepath.FromSlash(manifest.Backend))
	}
	return entry
}

func catalogEntryFromManifest(manifest Manifest) CatalogEntry {
	return CatalogEntry{
		ID:              strings.TrimSpace(manifest.ID),
		Name:            strings.TrimSpace(manifest.Name),
		Description:     strings.TrimSpace(manifest.Description),
		Version:         strings.TrimSpace(manifest.Version),
		Platform:        strings.TrimSpace(manifest.Platform),
		Capabilities:    append([]string(nil), manifest.Capabilities...),
		TelemetryInputs: append([]string(nil), manifest.TelemetryInputs...),
		Frontend:        filepath.ToSlash(filepath.Clean(filepath.FromSlash(manifest.Frontend))),
		Style:           normalizedOptionalPluginPath(manifest.Style),
		Page:            manifest.Page,
	}
}

func normalizedOptionalPluginPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
}

func invalidCatalogEntry(id, reason string) CatalogEntry {
	id = strings.TrimSpace(id)
	if len(id) > 64 {
		id = id[:64]
	}
	return CatalogEntry{ID: id, Name: id, State: CatalogStateInvalid, LastError: reason}
}

func validateManifest(pluginDir, folderName string, manifest *Manifest) error {
	manifest.ID = strings.TrimSpace(manifest.ID)
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Platform = strings.TrimSpace(manifest.Platform)
	manifest.Backend = strings.TrimSpace(manifest.Backend)
	manifest.Frontend = strings.TrimSpace(manifest.Frontend)
	manifest.Style = strings.TrimSpace(manifest.Style)
	manifest.Page.ID = strings.TrimSpace(manifest.Page.ID)
	manifest.Page.Title = strings.TrimSpace(manifest.Page.Title)
	manifest.Page.Icon = strings.TrimSpace(manifest.Page.Icon)
	manifest.Page.IconAsset = normalizedOptionalPluginPath(manifest.Page.IconAsset)

	if !validPluginID(manifest.ID) {
		return fmt.Errorf("invalid plugin id")
	}
	if manifest.ID != folderName {
		return fmt.Errorf("plugin id %q does not match directory %q", manifest.ID, folderName)
	}
	if manifest.Name == "" || len(manifest.Name) > 80 {
		return fmt.Errorf("plugin name is required and must not exceed 80 characters")
	}
	if manifest.Version == "" || len(manifest.Version) > 40 {
		return fmt.Errorf("plugin version is required")
	}
	if manifest.Platform == "" || len(manifest.Platform) > 40 {
		return fmt.Errorf("plugin platform is required")
	}
	if manifest.Page.ID == "" || !validPluginID(manifest.Page.ID) {
		return fmt.Errorf("plugin page id is invalid")
	}
	if manifest.Page.Title == "" || len(manifest.Page.Title) > 48 {
		return fmt.Errorf("plugin page title is required and must not exceed 48 characters")
	}
	if !validPageIcon(manifest.Page.Icon) {
		return fmt.Errorf("plugin page icon is not allowed")
	}
	if manifest.Page.IconAsset != "" {
		if err := validatePluginFile(pluginDir, manifest.Page.IconAsset); err != nil {
			return fmt.Errorf("plugin page icon asset: %w", err)
		}
		contentType, ok := pluginAssetContentType(filepath.Ext(manifest.Page.IconAsset))
		if !ok || !strings.HasPrefix(contentType, "image/") {
			return fmt.Errorf("plugin page icon asset must be a supported image")
		}
	}
	if manifest.Page.Order < 0 || manifest.Page.Order > 1000 {
		return fmt.Errorf("plugin page order must be between 0 and 1000")
	}

	for _, file := range []struct {
		label string
		path  string
	}{
		{label: "backend", path: manifest.Backend},
		{label: "frontend", path: manifest.Frontend},
	} {
		if err := validatePluginFile(pluginDir, file.path); err != nil {
			return fmt.Errorf("plugin %s: %w", file.label, err)
		}
	}
	if !strings.EqualFold(filepath.Ext(manifest.Frontend), ".js") {
		return fmt.Errorf("plugin frontend must be a JavaScript file")
	}
	if manifest.Style != "" {
		if err := validatePluginFile(pluginDir, manifest.Style); err != nil {
			return fmt.Errorf("plugin style: %w", err)
		}
		if !strings.EqualFold(filepath.Ext(manifest.Style), ".css") {
			return fmt.Errorf("plugin style must be a CSS file")
		}
	}
	return nil
}

func pluginAssetContentType(extension string) (string, bool) {
	switch strings.ToLower(extension) {
	case ".js":
		return "text/javascript; charset=utf-8", true
	case ".css":
		return "text/css; charset=utf-8", true
	case ".json":
		return "application/json; charset=utf-8", true
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".webp":
		return "image/webp", true
	case ".gif":
		return "image/gif", true
	case ".woff":
		return "font/woff", true
	case ".woff2":
		return "font/woff2", true
	default:
		return "", false
	}
}

func validatePluginFile(pluginDir, relativePath string) error {
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" || filepath.IsAbs(relativePath) || filepath.VolumeName(relativePath) != "" {
		return fmt.Errorf("entry must be a relative file path")
	}
	cleaned := filepath.Clean(filepath.FromSlash(relativePath))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("entry escapes the plugin directory")
	}
	fullPath := filepath.Join(pluginDir, cleaned)
	if !pathInside(fullPath, pluginDir) {
		return fmt.Errorf("entry escapes the plugin directory")
	}
	info, err := os.Lstat(fullPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("entry must be a regular non-symlink file")
	}
	resolved, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return err
	}
	if !pathInside(resolved, pluginDir) {
		return fmt.Errorf("entry resolves outside the plugin directory")
	}
	return nil
}

func pathInside(path, root string) bool {
	path, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func validPluginID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= '0' && char <= '9':
		case char == '-':
		default:
			return false
		}
	}
	return true
}

func validPageIcon(value string) bool {
	switch value {
	case "fan", "gauge", "laptop", "plug", "settings", "thermometer", "zap":
		return true
	default:
		return false
	}
}

func readLimitedFile(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("manifest exceeds %d bytes", maxBytes)
	}
	return data, nil
}

func readLimitedAsset(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("plugin asset exceeds %d bytes", maxBytes)
	}
	return data, nil
}
