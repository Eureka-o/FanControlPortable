package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Eureka-o/FanControlPortable/internal/ghmirror"
	"github.com/Eureka-o/FanControlPortable/internal/plugins"
)

const maxCatalogBytes = 4 << 20

var mirrorCandidateBuilder = ghmirror.BuildCandidates

// HTTPClientFactory returns an HTTP client honoring system proxy unless bypassProxy is true.
type HTTPClientFactory func(bypassProxy bool) *http.Client

// FetchCatalog loads and evaluates catalog.json from a remote URL or local file path.
func FetchCatalog(
	ctx context.Context,
	catalogURL string,
	platform string,
	coreVersion string,
	installedPlugins InstalledPluginMap,
	installedThemes map[string]InstalledThemeMeta,
	clientFactory HTTPClientFactory,
) (*CatalogView, error) {
	resolvedURL := strings.TrimSpace(catalogURL)
	if resolvedURL == "" {
		resolvedURL = DefaultCatalogURL
	}
	if strings.TrimSpace(platform) == "" {
		platform = plugins.CurrentPlatform()
	}

	raw, resolvedSource, latencyMs, err := readCatalogBytes(ctx, resolvedURL, clientFactory)
	if err != nil {
		return nil, err
	}

	var catalog Catalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return nil, fmt.Errorf("解析市场目录 JSON 失败: %w", err)
	}
	if catalog.SchemaVersion != 1 {
		return nil, fmt.Errorf("不支持的市场目录版本: %d", catalog.SchemaVersion)
	}

	localBaseDir := ""
	if !isRemoteURL(resolvedURL) {
		localBaseDir = filepath.Dir(resolveLocalPath(resolvedURL))
	}

	view := &CatalogView{
		CatalogURL:     resolvedURL,
		ResolvedSource: resolvedSource,
		LatencyMs:      latencyMs,
		UpdatedAt:      catalog.UpdatedAt,
		Plugins:        make([]CatalogPluginView, 0, len(catalog.Plugins)),
		Themes:         make([]CatalogThemeView, 0, len(catalog.Themes)),
	}

	for _, entry := range catalog.Plugins {
		entry.DownloadURL = resolveCatalogPackageURL(localBaseDir, entry.DownloadURL)
		item := CatalogPluginView{PluginEntry: entry}
		compat, reason := evaluatePluginCompat(entry, platform, coreVersion)
		item.Compatible = compat
		item.Reason = reason

		if existing, ok := installedPlugins[entry.ID]; ok {
			item.InstalledVersion = existing.Version
			item.PendingRestart = existing.PendingRestart
		}

		switch {
		case !compat:
			item.InstallStatus = InstallStatusIncompatible
		case item.InstalledVersion != "":
			if compareSemver(entry.Version, item.InstalledVersion) > 0 {
				item.InstallStatus = InstallStatusUpdateAvailable
			} else {
				item.InstallStatus = InstallStatusInstalled
			}
		default:
			item.InstallStatus = InstallStatusNotInstalled
		}
		view.Plugins = append(view.Plugins, item)
	}

	for _, entry := range catalog.Themes {
		entry.DownloadURL = resolveCatalogPackageURL(localBaseDir, entry.DownloadURL)
		item := CatalogThemeView{ThemeEntry: entry}
		compat, reason := evaluateThemeCompat(entry, coreVersion)
		item.Compatible = compat
		item.Reason = reason

		existing, isInstalled := installedThemes[entry.ID]
		if isInstalled {
			item.InstalledVersion = existing.Version
			item.InstalledSource = existing.Source
			item.InstalledBuiltin = existing.Builtin
		}

		switch {
		case !compat:
			item.InstallStatus = InstallStatusIncompatible
		case isInstalled:
			if entry.Version != "" && (strings.TrimSpace(existing.Version) == "" || compareSemver(entry.Version, existing.Version) > 0) {
				item.InstallStatus = InstallStatusUpdateAvailable
			} else {
				item.InstallStatus = InstallStatusInstalled
			}
		default:
			item.InstallStatus = InstallStatusNotInstalled
		}
		view.Themes = append(view.Themes, item)
	}

	return view, nil
}

func readCatalogBytes(ctx context.Context, rawLocation string, clientFactory HTTPClientFactory) ([]byte, string, int64, error) {
	if isRemoteURL(rawLocation) {
		candidates := mirrorCandidateBuilder(rawLocation)
		headers := map[string]string{
			"Accept":     "application/json",
			"User-Agent": "FanControl-Marketplace",
		}
		validate := func(data []byte) error {
			var probe Catalog
			if err := json.Unmarshal(data, &probe); err != nil {
				return fmt.Errorf("invalid catalog JSON: %w", err)
			}
			if probe.SchemaVersion != 1 {
				return fmt.Errorf("unsupported catalog schemaVersion: %d", probe.SchemaVersion)
			}
			return nil
		}

		var lastErr error
		for _, bypassProxy := range []bool{false, true} {
			client := resolveClient(clientFactory, bypassProxy)
			data, winner, latencyMs, err := ghmirror.RaceGet(ctx, candidates, client, headers, maxCatalogBytes, validate)
			if err == nil {
				return data, winner.Label, latencyMs, nil
			}
			lastErr = err
			if ctx.Err() != nil {
				break
			}
		}
		return nil, "", 0, fmt.Errorf("获取市场目录失败: %w", lastErr)
	}

	localPath := resolveLocalPath(rawLocation)
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, "", 0, fmt.Errorf("读取本地目录失败: %w", err)
	}
	if len(data) > maxCatalogBytes {
		return nil, "", 0, fmt.Errorf("市场目录超过大小限制")
	}
	return data, "本地索引", 0, nil
}

func resolveClient(factory HTTPClientFactory, bypassProxy bool) *http.Client {
	if factory != nil {
		return factory(bypassProxy)
	}
	return http.DefaultClient
}

func isRemoteURL(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func resolveLocalPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		if u, err := url.Parse(trimmed); err == nil {
			p := u.Path
			if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
				p = p[1:]
			}
			return filepath.FromSlash(p)
		}
	}
	return filepath.Clean(trimmed)
}

func resolveCatalogPackageURL(localBaseDir, rawDownloadURL string) string {
	trimmed := strings.TrimSpace(rawDownloadURL)
	if trimmed == "" || localBaseDir == "" || isRemoteURL(trimmed) || strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		return trimmed
	}
	resolved := resolveLocalPath(trimmed)
	if filepath.IsAbs(resolved) {
		return resolved
	}
	return filepath.Clean(filepath.Join(localBaseDir, resolved))
}

func evaluatePluginCompat(entry PluginEntry, platform, coreVersion string) (bool, string) {
	if entry.Platform != "" && entry.Platform != platform {
		return false, fmt.Sprintf("平台不匹配（需要 %s）", entry.Platform)
	}
	if entry.ProtocolVersion != plugins.SupportedProtocolVersion {
		return false, fmt.Sprintf("协议版本不兼容（需要 v%d）", entry.ProtocolVersion)
	}
	if entry.HostAPIVersion != 0 && entry.HostAPIVersion != plugins.SupportedHostAPIVersion {
		return false, fmt.Sprintf("宿主接口版本不兼容（v%d）", entry.HostAPIVersion)
	}
	if entry.MinCoreVersion != "" && compareSemver(normalizedCoreVersion(coreVersion), entry.MinCoreVersion) < 0 {
		return false, fmt.Sprintf("需要核心版本 >= %s", entry.MinCoreVersion)
	}
	return true, ""
}

func evaluateThemeCompat(entry ThemeEntry, coreVersion string) (bool, string) {
	if entry.MinCoreVersion != "" && compareSemver(normalizedCoreVersion(coreVersion), entry.MinCoreVersion) < 0 {
		return false, fmt.Sprintf("需要核心版本 >= %s", entry.MinCoreVersion)
	}
	return true, ""
}

func normalizedCoreVersion(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" || strings.EqualFold(trimmed, "dev") {
		return "999.0.0"
	}
	return trimmed
}

func compareSemver(a, b string) int {
	pa, okA := parseSemver(a)
	pb, okB := parseSemver(b)
	if !okA || !okB {
		return strings.Compare(strings.TrimSpace(a), strings.TrimSpace(b))
	}
	for i := 0; i < 3; i++ {
		if pa[i] > pb[i] {
			return 1
		}
		if pa[i] < pb[i] {
			return -1
		}
	}
	return 0
}

func parseSemver(raw string) ([3]int, bool) {
	var out [3]int
	cleaned := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	if idx := strings.IndexAny(cleaned, "-+"); idx >= 0 {
		cleaned = cleaned[:idx]
	}
	segs := strings.Split(cleaned, ".")
	if len(segs) < 2 || len(segs) > 3 {
		return out, false
	}
	for i, s := range segs {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
