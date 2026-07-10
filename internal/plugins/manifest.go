package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const ManifestFileName = "plugin.json"

type PluginType string

const (
	PluginTypeBackground PluginType = "background"
	PluginTypeDevice     PluginType = "device"
)

type Manifest struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Version     string     `json:"version"`
	Type        PluginType `json:"type"`
	Executable  string     `json:"executable"`
	Frontend    string     `json:"frontend,omitempty"`
	Icon        string     `json:"icon,omitempty"`
	MinCoreVer  string     `json:"minCoreVersion,omitempty"`
	Description string     `json:"description,omitempty"`
}

var validPluginIDRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

func (m *Manifest) Validate() error {
	if m == nil {
		return errors.New("manifest is nil")
	}

	m.ID = strings.TrimSpace(m.ID)
	m.Name = strings.TrimSpace(m.Name)
	m.Version = strings.TrimSpace(m.Version)
	m.Executable = strings.TrimSpace(m.Executable)
	m.Frontend = strings.TrimSpace(m.Frontend)
	m.Icon = strings.TrimSpace(m.Icon)
	m.MinCoreVer = strings.TrimSpace(m.MinCoreVer)
	m.Description = strings.TrimSpace(m.Description)

	if m.ID == "" {
		return errors.New("manifest id is required")
	}
	if len(m.ID) > 64 || !validPluginIDRe.MatchString(m.ID) {
		return errors.New("manifest id must be lowercase letters, digits, and hyphens")
	}
	if m.Name == "" {
		return errors.New("manifest name is required")
	}
	if m.Version == "" {
		return errors.New("manifest version is required")
	}
	if m.Type != PluginTypeBackground && m.Type != PluginTypeDevice {
		return errors.New("manifest type must be background or device")
	}
	if err := validateRelativeExecutable(m.Executable); err != nil {
		return err
	}
	if m.Frontend != "" {
		if err := validateRelativePath(m.Frontend, "manifest frontend"); err != nil {
			return err
		}
	}
	if m.Icon != "" {
		// Icon accepts either an inline <svg>…</svg> string (preferred, keeps plugin visuals
		// self-contained) or a legacy id (lowercase letters/digits/hyphens, ≤64 chars) as a
		// fallback marker that the host may render with a default glyph.
		trimmed := strings.TrimSpace(m.Icon)
		looksLikeSVG := strings.HasPrefix(trimmed, "<svg") && strings.HasSuffix(trimmed, "</svg>")
		if looksLikeSVG {
			if len(trimmed) > 8192 {
				return errors.New("manifest icon svg must be <= 8192 bytes")
			}
		} else if len(trimmed) > 64 || !validPluginIDRe.MatchString(trimmed) {
			return errors.New("manifest icon must be an inline <svg>…</svg> or a legacy id (lowercase letters, digits, hyphens)")
		}
	}

	return nil
}

func LoadManifest(pluginDir string) (*Manifest, error) {
	manifestPath := filepath.Join(pluginDir, ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func (m *Manifest) ExecutablePath(pluginDir string) (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}

	root, err := filepath.Abs(pluginDir)
	if err != nil {
		return "", err
	}
	executable := filepath.FromSlash(strings.ReplaceAll(m.Executable, "\\", "/"))
	resolved, err := filepath.Abs(filepath.Join(root, executable))
	if err != nil {
		return "", err
	}
	if !pathInside(root, resolved) {
		return "", fmt.Errorf("manifest executable escapes plugin directory")
	}

	return resolved, nil
}

func (m *Manifest) FrontendPath(pluginDir string) (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}
	if m.Frontend == "" {
		return "", errors.New("manifest frontend is not configured")
	}

	root, err := filepath.Abs(pluginDir)
	if err != nil {
		return "", err
	}
	frontend := filepath.FromSlash(strings.ReplaceAll(m.Frontend, "\\", "/"))
	resolved, err := filepath.Abs(filepath.Join(root, frontend))
	if err != nil {
		return "", err
	}
	if !pathInside(root, resolved) {
		return "", fmt.Errorf("manifest frontend escapes plugin directory")
	}

	return resolved, nil
}

func validateRelativeExecutable(executable string) error {
	if executable == "" {
		return errors.New("manifest executable is required")
	}
	return validateRelativePath(executable, "manifest executable")
}

func validateRelativePath(pathValue string, fieldName string) error {
	normalized := strings.ReplaceAll(pathValue, "\\", "/")
	if strings.HasPrefix(normalized, "/") || filepath.IsAbs(pathValue) || filepath.VolumeName(pathValue) != "" {
		return fmt.Errorf("%s must be relative", fieldName)
	}
	if strings.Contains(normalized, ":") {
		return fmt.Errorf("%s must not contain volume or stream separators", fieldName)
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%s must not contain empty or traversal path components", fieldName)
		}
	}
	return nil
}

func pathInside(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
