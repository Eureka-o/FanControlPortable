// Package theme discovers, seeds, migrates, and reads FanControl UI themes.
package theme

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	manifestName = "theme.json"
	styleName    = "theme.css"

	SourceUser    = "user"
	SourceInstall = "install"
	SourceBuiltin = "builtin"
)

// Meta is parsed from theme.json and returned to the frontend theme picker.
type Meta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Base        string `json:"base"` // light | dark
	Author      string `json:"author,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source"` // user | install | builtin
}

// Manager keeps install-root themes authoritative.
//
// legacyDirs are old user-profile theme folders from earlier versions. They are
// read only as a compatibility source and migration input; new files are never
// written there.
type Manager struct {
	installDir string
	legacyDirs []string
	builtin    fs.FS
}

// NewManager creates a theme manager.
func NewManager(installThemesDir string, legacyThemeDirs []string, builtin fs.FS) *Manager {
	return &Manager{
		installDir: cleanOptionalPath(installThemesDir),
		legacyDirs: normalizeLegacyDirs(installThemesDir, legacyThemeDirs),
		builtin:    builtin,
	}
}

func cleanOptionalPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func normalizeLegacyDirs(installThemesDir string, dirs []string) []string {
	installKey := pathKey(installThemesDir)
	seen := map[string]bool{}
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = cleanOptionalPath(dir)
		if dir == "" {
			continue
		}
		key := pathKey(dir)
		if key == "" || key == installKey || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, dir)
	}
	return out
}

func pathKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(filepath.Clean(path)); err == nil {
		path = abs
	} else {
		path = filepath.Clean(path)
	}
	return strings.ToLower(path)
}

func validID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func normalizeBase(base string) string {
	if base == "dark" {
		return "dark"
	}
	return "light"
}

// EnsureSeeded copies built-in themes to the install-root themes folder and
// then tries to migrate old user-profile themes into that same folder.
//
// All work here is best effort: missing legacy folders, read errors, write
// errors, and cleanup failures never block app startup or installation.
func (m *Manager) EnsureSeeded() {
	if m.builtin != nil {
		m.seedBuiltinThemes()
	}
	m.migrateLegacyThemes()
}

func (m *Manager) seedBuiltinThemes() {
	entries, err := fs.ReadDir(m.builtin, ".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		if !validID(id) || m.themeExistsOnDisk(m.installDir, id) {
			continue
		}
		_ = m.copyBuiltin(id, m.installDir)
	}
}

func (m *Manager) themeExistsOnDisk(baseDir, id string) bool {
	if baseDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(baseDir, id, manifestName))
	return err == nil
}

func (m *Manager) copyBuiltin(id, baseDir string) error {
	if baseDir == "" {
		return fmt.Errorf("empty theme destination")
	}
	srcRoot := id
	entries, err := fs.ReadDir(m.builtin, srcRoot)
	if err != nil {
		return err
	}
	dstDir := filepath.Join(baseDir, id)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := fs.ReadFile(m.builtin, srcRoot+"/"+entry.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstDir, entry.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

type legacyCandidate struct {
	id      string
	dir     string
	meta    Meta
	modTime time.Time
}

func (m *Manager) migrateLegacyThemes() {
	if m.installDir == "" || len(m.legacyDirs) == 0 {
		return
	}

	grouped := map[string][]legacyCandidate{}
	for _, legacyDir := range m.legacyDirs {
		for _, candidate := range scanLegacyCandidates(legacyDir) {
			grouped[candidate.id] = append(grouped[candidate.id], candidate)
		}
	}

	for id, candidates := range grouped {
		if len(candidates) == 0 {
			continue
		}

		if m.themeExistsOnDisk(m.installDir, id) {
			removeLegacyCandidates(candidates, m.legacyDirs)
			continue
		}

		chosen := chooseNewestCandidate(candidates)
		if err := copyLegacyTheme(chosen, m.installDir); err != nil {
			continue
		}
		removeLegacyCandidates(candidates, m.legacyDirs)
	}

	for _, legacyDir := range m.legacyDirs {
		_ = os.Remove(legacyDir)
		_ = os.Remove(filepath.Dir(legacyDir))
	}
}

func scanLegacyCandidates(legacyDir string) []legacyCandidate {
	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		return nil
	}

	var out []legacyCandidate
	for _, entry := range entries {
		if !entry.IsDir() || !validID(entry.Name()) {
			continue
		}
		manifestPath := filepath.Join(legacyDir, entry.Name(), manifestName)
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		meta, ok := parseMeta(data, entry.Name())
		if !ok {
			continue
		}
		info, err := os.Stat(manifestPath)
		if err != nil {
			continue
		}
		out = append(out, legacyCandidate{
			id:      meta.ID,
			dir:     filepath.Join(legacyDir, entry.Name()),
			meta:    meta,
			modTime: info.ModTime(),
		})
	}
	return out
}

func chooseNewestCandidate(candidates []legacyCandidate) legacyCandidate {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if compareVersion(candidate.meta.Version, best.meta.Version) > 0 {
			best = candidate
			continue
		}
		if compareVersion(candidate.meta.Version, best.meta.Version) == 0 && candidate.modTime.After(best.modTime) {
			best = candidate
		}
	}
	return best
}

func compareVersion(a, b string) int {
	aa := versionParts(a)
	bb := versionParts(b)
	maxLen := len(aa)
	if len(bb) > maxLen {
		maxLen = len(bb)
	}
	for i := 0; i < maxLen; i++ {
		av, bv := 0, 0
		if i < len(aa) {
			av = aa[i]
		}
		if i < len(bb) {
			bv = bb[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func versionParts(version string) []int {
	fields := strings.FieldsFunc(version, func(r rune) bool {
		return r < '0' || r > '9'
	})
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		value, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		parts = append(parts, value)
	}
	return parts
}

func copyLegacyTheme(candidate legacyCandidate, installDir string) error {
	if installDir == "" || candidate.id == "" {
		return fmt.Errorf("empty theme migration target")
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return err
	}

	tmpDir := filepath.Join(installDir, "."+candidate.id+".migration")
	dstDir := filepath.Join(installDir, candidate.id)
	_ = os.RemoveAll(tmpDir)
	if err := copyDir(candidate.dir, tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	_ = os.RemoveAll(dstDir)
	if err := os.Rename(tmpDir, dstDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func removeLegacyCandidates(candidates []legacyCandidate, legacyDirs []string) {
	for _, candidate := range candidates {
		if isUnderAnyDir(candidate.dir, legacyDirs) {
			_ = os.RemoveAll(candidate.dir)
		}
	}
}

func isUnderAnyDir(path string, roots []string) bool {
	pathKeyValue := pathKey(path)
	for _, root := range roots {
		rootKey := pathKey(root)
		if rootKey == "" {
			continue
		}
		rel, err := filepath.Rel(rootKey, pathKeyValue)
		if err != nil {
			continue
		}
		if rel == "." || (rel != "" && !strings.HasPrefix(rel, "..")) {
			return true
		}
	}
	return false
}

// List returns available themes. Install-root themes win over legacy user
// residue and embedded built-ins.
func (m *Manager) List() []Meta {
	merged := map[string]Meta{}

	if m.builtin != nil {
		if entries, err := fs.ReadDir(m.builtin, "."); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				if meta, ok := m.readBuiltinMeta(entry.Name()); ok {
					merged[meta.ID] = meta
				}
			}
		}
	}

	for _, legacyDir := range m.legacyDirs {
		for _, meta := range m.scanDir(legacyDir, SourceUser) {
			merged[meta.ID] = meta
		}
	}

	for _, meta := range m.scanDir(m.installDir, SourceInstall) {
		merged[meta.ID] = meta
	}

	out := make([]Meta, 0, len(merged))
	for _, meta := range merged {
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (m *Manager) scanDir(baseDir, source string) []Meta {
	if baseDir == "" {
		return nil
	}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}
	var out []Meta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(baseDir, entry.Name(), manifestName)
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		meta, ok := parseMeta(data, entry.Name())
		if !ok {
			continue
		}
		meta.Source = source
		out = append(out, meta)
	}
	return out
}

func (m *Manager) readBuiltinMeta(id string) (Meta, bool) {
	data, err := fs.ReadFile(m.builtin, id+"/"+manifestName)
	if err != nil {
		return Meta{}, false
	}
	meta, ok := parseMeta(data, id)
	if !ok {
		return Meta{}, false
	}
	meta.Source = SourceBuiltin
	return meta, true
}

func parseMeta(data []byte, folderName string) (Meta, bool) {
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return Meta{}, false
	}
	if meta.ID == "" {
		meta.ID = folderName
	}
	if !validID(meta.ID) {
		return Meta{}, false
	}
	if meta.Name == "" {
		meta.Name = meta.ID
	}
	meta.Base = normalizeBase(meta.Base)
	return meta, true
}

// ReadCSS reads theme.css. The install package wins over old user residue.
func (m *Manager) ReadCSS(id string) (string, error) {
	if !validID(id) {
		return "", fmt.Errorf("invalid theme id: %q", id)
	}

	if m.installDir != "" {
		if data, err := os.ReadFile(filepath.Join(m.installDir, id, styleName)); err == nil {
			return string(data), nil
		}
	}

	for _, legacyDir := range m.legacyDirs {
		if data, err := os.ReadFile(filepath.Join(legacyDir, id, styleName)); err == nil {
			return string(data), nil
		}
	}

	if m.builtin != nil {
		if data, err := fs.ReadFile(m.builtin, id+"/"+styleName); err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("theme %q style file not found", id)
}

// ResolveDir returns the install-root themes folder exposed by "Open themes".
func (m *Manager) ResolveDir() string {
	if m.installDir != "" {
		_ = os.MkdirAll(m.installDir, 0o755)
	}
	return m.installDir
}
