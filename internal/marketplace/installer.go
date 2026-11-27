package marketplace

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Eureka-o/FanControlPortable/internal/ghmirror"
	"github.com/Eureka-o/FanControlPortable/internal/plugins"
	"github.com/Eureka-o/FanControlPortable/internal/theme"
)

const (
	MaxPackageDownloadBytes = 64 << 20
	MaxZipEntryCount        = 2000
	MaxExtractedBytes       = 256 << 20
)

// DownloadAndVerify fetches a .zip package from remote URL or local file path and checks its SHA256.
func DownloadAndVerify(
	ctx context.Context,
	location string,
	expectedSHA256 string,
	clientFactory HTTPClientFactory,
	onProgress func(received, total int64),
) (zipPath string, actualSHA256 string, cleanup func(), err error) {
	tempDir, err := os.MkdirTemp("", "fancontrol-mkt-*")
	if err != nil {
		return "", "", func() {}, fmt.Errorf("创建临时目录失败: %w", err)
	}
	cleanup = func() {
		_ = os.RemoveAll(tempDir)
	}

	destZip := filepath.Join(tempDir, "package.zip")
	expected := strings.ToLower(strings.TrimSpace(expectedSHA256))
	if isRemoteURL(location) {
		sha, dlErr := downloadRemoteZip(ctx, location, expected, destZip, clientFactory, onProgress)
		if dlErr != nil {
			cleanup()
			return "", "", func() {}, dlErr
		}
		actualSHA256 = sha
	} else {
		localZip := resolveLocalPath(location)
		sha, copyErr := copyLocalZip(localZip, destZip, onProgress)
		if copyErr != nil {
			cleanup()
			return "", "", func() {}, copyErr
		}
		actualSHA256 = sha
		if expected != "" && !strings.EqualFold(actualSHA256, expected) {
			cleanup()
			return "", "", func() {}, fmt.Errorf("安装包 SHA256 校验失败: 期望 %s，实际 %s", expected, actualSHA256)
		}
	}

	return destZip, actualSHA256, cleanup, nil
}

func downloadRemoteZip(
	ctx context.Context,
	rawURL string,
	expectedSHA256 string,
	destPath string,
	clientFactory HTTPClientFactory,
	onProgress func(received, total int64),
) (string, error) {
	candidates := mirrorCandidateBuilder(rawURL)
	var lastErr error
	for _, bypassProxy := range []bool{false, true} {
		probeClient := resolveClient(clientFactory, bypassProxy)
		ranked := ghmirror.RankByLatency(ctx, candidates, probeClient, "FanControl-Marketplace")
		client := resolveClient(clientFactory, bypassProxy)

		for _, item := range ranked {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
			if err != nil {
				lastErr = fmt.Errorf("构建下载请求失败 (%s): %w", item.Name, err)
				continue
			}
			req.Header.Set("User-Agent", "FanControl-Marketplace")
			resp, err := client.Do(req)
			if err != nil {
				lastErr = fmt.Errorf("%s: %w", item.Name, err)
				continue
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				_ = resp.Body.Close()
				lastErr = fmt.Errorf("%s 下载失败 (HTTP %d)", item.Name, resp.StatusCode)
				continue
			}
			sha, writeErr := streamToFileWithHash(resp.Body, resp.ContentLength, destPath, onProgress)
			_ = resp.Body.Close()
			if writeErr != nil {
				_ = os.Remove(destPath)
				lastErr = fmt.Errorf("%s: %w", item.Name, writeErr)
				continue
			}
			if expectedSHA256 != "" && !strings.EqualFold(sha, expectedSHA256) {
				_ = os.Remove(destPath)
				lastErr = fmt.Errorf("%s 安装包 SHA256 校验失败: 期望 %s，实际 %s", item.Name, expectedSHA256, sha)
				continue
			}
			return sha, nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	return "", fmt.Errorf("下载包失败: %w", lastErr)
}

func copyLocalZip(srcPath, destPath string, onProgress func(received, total int64)) (string, error) {
	info, err := os.Stat(srcPath)
	if err != nil {
		return "", fmt.Errorf("打开本地安装包失败: %w", err)
	}
	if info.Size() > MaxPackageDownloadBytes {
		return "", fmt.Errorf("安装包体积超过限制 (%d 字节)", info.Size())
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("打开本地安装包失败: %w", err)
	}
	defer f.Close()
	return streamToFileWithHash(f, info.Size(), destPath, onProgress)
}

func streamToFileWithHash(src io.Reader, total int64, destPath string, onProgress func(received, total int64)) (string, error) {
	out, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("创建下载文件失败: %w", err)
	}
	defer out.Close()

	hasher := sha256.New()
	limited := io.LimitReader(src, MaxPackageDownloadBytes+1)
	buf := make([]byte, 32*1024)
	var received int64
	for {
		n, readErr := limited.Read(buf)
		if n > 0 {
			received += int64(n)
			if received > MaxPackageDownloadBytes {
				return "", fmt.Errorf("下载包超过大小限制 (%d MiB)", MaxPackageDownloadBytes>>20)
			}
			if _, wErr := out.Write(buf[:n]); wErr != nil {
				return "", fmt.Errorf("写入下载文件失败: %w", wErr)
			}
			_, _ = hasher.Write(buf[:n])
			if onProgress != nil {
				onProgress(received, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("读取下载流失败: %w", readErr)
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// SafeExtractZip validates zip archive entries (Zip Slip, symlinks, file count, total bytes)
// and strips a redundant single top-level wrapper folder when manifest sits one level down.
func SafeExtractZip(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("无效的 ZIP 安装包: %w", err)
	}
	defer zr.Close()

	if len(zr.File) == 0 {
		return fmt.Errorf("ZIP 安装包为空")
	}
	if len(zr.File) > MaxZipEntryCount {
		return fmt.Errorf("ZIP 文件数量超过限制 (%d)", MaxZipEntryCount)
	}

	stripPrefix := detectSingleRootStripPrefix(zr.File)

	var totalBytes uint64
	resolvedDest, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("ZIP 不允许包含符号链接: %s", f.Name)
		}
		cleanZipName := filepath.ToSlash(filepath.Clean(strings.ReplaceAll(f.Name, `\`, `/`)))
		if cleanZipName == "." || cleanZipName == "" {
			continue
		}
		if strings.HasPrefix(cleanZipName, "/") || strings.HasPrefix(cleanZipName, "..") || strings.Contains(cleanZipName, ":") {
			return fmt.Errorf("检测到越界 ZIP 路径: %s", f.Name)
		}

		relName := cleanZipName
		if stripPrefix != "" {
			if relName == strings.TrimSuffix(stripPrefix, "/") {
				continue
			}
			relName = strings.TrimPrefix(relName, stripPrefix)
		}
		if relName == "" || relName == "." {
			continue
		}

		targetPath := filepath.Join(resolvedDest, filepath.FromSlash(relName))
		relCheck, relErr := filepath.Rel(resolvedDest, targetPath)
		if relErr != nil || relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(filepath.Separator)) || filepath.IsAbs(relCheck) {
			return fmt.Errorf("检测到 Zip Slip 路径穿越: %s", f.Name)
		}

		if isZipDirEntry(f) {
			if mkErr := os.MkdirAll(targetPath, 0o755); mkErr != nil {
				return mkErr
			}
			continue
		}

		if mkErr := os.MkdirAll(filepath.Dir(targetPath), 0o755); mkErr != nil {
			return mkErr
		}

		rc, openErr := f.Open()
		if openErr != nil {
			return openErr
		}
		out, createErr := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if createErr != nil {
			_ = rc.Close()
			return createErr
		}
		n, copyErr := io.CopyN(out, rc, int64(MaxExtractedBytes-totalBytes)+1)
		_ = rc.Close()
		closeErr := out.Close()
		totalBytes += uint64(n)
		if copyErr != nil && copyErr != io.EOF {
			return fmt.Errorf("解压文件失败 (%s): %w", relName, copyErr)
		}
		if closeErr != nil {
			return closeErr
		}
		if totalBytes > MaxExtractedBytes {
			return fmt.Errorf("ZIP 解压总大小超过限制 (%d MiB)", MaxExtractedBytes>>20)
		}
	}
	return nil
}

func isZipDirEntry(f *zip.File) bool {
	return f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") || strings.HasSuffix(f.Name, `\`)
}

func detectSingleRootStripPrefix(files []*zip.File) string {
	hasRootManifest := false
	singleTopDir := ""
	allUnderSameDir := true

	for _, f := range files {
		norm := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(strings.ReplaceAll(f.Name, `\`, `/`))), "./")
		if norm == "" || norm == "." {
			continue
		}
		if norm == "plugin.json" || norm == "theme.json" {
			hasRootManifest = true
		}
		parts := strings.SplitN(norm, "/", 2)
		if len(parts) < 2 && !isZipDirEntry(f) {
			allUnderSameDir = false
		}
		top := parts[0]
		if singleTopDir == "" {
			singleTopDir = top
		} else if singleTopDir != top {
			allUnderSameDir = false
		}
	}

	if hasRootManifest || !allUnderSameDir || singleTopDir == "" {
		return ""
	}
	return singleTopDir + "/"
}

// DetectZipKind inspects a local package zip and reports whether it is a "plugin" or "theme".
func DetectZipKind(zipPath string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("打开 ZIP 失败: %w", err)
	}
	defer zr.Close()

	stripPrefix := detectSingleRootStripPrefix(zr.File)
	hasPlugin := false
	hasTheme := false
	for _, f := range zr.File {
		if isZipDirEntry(f) {
			continue
		}
		norm := filepath.ToSlash(filepath.Clean(strings.ReplaceAll(f.Name, `\`, `/`)))
		if stripPrefix != "" {
			norm = strings.TrimPrefix(norm, stripPrefix)
		}
		switch norm {
		case "plugin.json":
			hasPlugin = true
		case "theme.json":
			hasTheme = true
		}
	}
	if hasPlugin {
		return "plugin", nil
	}
	if hasTheme {
		return "theme", nil
	}
	return "", fmt.Errorf("压缩包根目录下缺少 plugin.json 或 theme.json")
}

// InstallPluginFromZip extracts, validates, and installs (or stages for restart) an external plugin.
func InstallPluginFromZip(
	zipPath string,
	pluginsDir string,
	expectedID string,
	platform string,
	coreVersion string,
	currentlyLoadedInproc bool,
) (*InstallResult, error) {
	if strings.TrimSpace(pluginsDir) == "" {
		return nil, fmt.Errorf("插件目录未配置")
	}
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建插件目录失败: %w", err)
	}

	unpackTemp, err := os.MkdirTemp(pluginsDir, ".unpack-*")
	if err != nil {
		return nil, fmt.Errorf("创建解压临时目录失败: %w", err)
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(unpackTemp)
		}
	}()

	if err := SafeExtractZip(zipPath, unpackTemp); err != nil {
		return nil, err
	}

	info, manifest, err := plugins.InspectPluginDirLooseID(unpackTemp, platform, coreVersion)
	if err != nil {
		return nil, fmt.Errorf("插件校验失败: %w", err)
	}
	if expectedID != "" && manifest.ID != expectedID {
		return nil, fmt.Errorf("插件包 ID (%s) 与目标 (%s) 不一致", manifest.ID, expectedID)
	}

	targetDir := filepath.Join(pluginsDir, manifest.ID)
	isInproc := info.Runtime == plugins.RuntimeInprocDLL
	_, targetStatErr := os.Stat(targetDir)
	targetExists := targetStatErr == nil

	if isInproc && (currentlyLoadedInproc || targetExists) {
		if err := plugins.StagePluginDir(pluginsDir, manifest.ID, unpackTemp); err != nil {
			return nil, err
		}
		cleanupTemp = false
		return &InstallResult{
			Kind:            "plugin",
			ID:              manifest.ID,
			Name:            manifest.Name,
			Version:         manifest.Version,
			Runtime:         info.Runtime,
			RestartRequired: true,
			Message:         "进程内插件更新已暂存，重启核心服务后生效",
		}, nil
	}

	backupDir := filepath.Join(pluginsDir, ".bak-"+manifest.ID)
	_ = os.RemoveAll(backupDir)
	if targetExists {
		if err := os.Rename(targetDir, backupDir); err != nil {
			return nil, fmt.Errorf("备份现有插件目录失败: %w", err)
		}
	}
	if err := os.Rename(unpackTemp, targetDir); err != nil {
		if targetExists {
			_ = os.Rename(backupDir, targetDir)
		}
		return nil, fmt.Errorf("安装插件目录失败: %w", err)
	}
	cleanupTemp = false
	_ = os.RemoveAll(backupDir)

	restartRequired := isInproc
	msg := "插件安装完成"
	if restartRequired {
		msg = "进程内插件已安装，重启核心服务后生效"
	}
	return &InstallResult{
		Kind:            "plugin",
		ID:              manifest.ID,
		Name:            manifest.Name,
		Version:         manifest.Version,
		Runtime:         info.Runtime,
		RestartRequired: restartRequired,
		Message:         msg,
	}, nil
}

// UninstallPlugin deletes a sidecar/frontend plugin immediately or stages inproc-dll deletion for restart.
func UninstallPlugin(pluginsDir, configDir, pluginID string, isLoadedInproc, removeData bool) (*InstallResult, error) {
	if strings.TrimSpace(pluginsDir) == "" {
		return nil, fmt.Errorf("插件目录未配置")
	}
	if isLoadedInproc {
		if err := plugins.MarkPluginPendingDelete(pluginsDir, pluginID, removeData); err != nil {
			return nil, err
		}
		return &InstallResult{
			Kind:            "plugin",
			ID:              pluginID,
			RestartRequired: true,
			Message:         "进程内插件已标记卸载，重启核心服务后生效",
		}, nil
	}

	targetDir := filepath.Join(pluginsDir, pluginID)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("未找到已安装的外部插件目录: %s", pluginID)
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return nil, fmt.Errorf("删除插件目录失败: %w", err)
	}
	if removeData && strings.TrimSpace(configDir) != "" {
		if dataDir, err := plugins.PluginDataDir(configDir, pluginID); err == nil {
			_ = os.RemoveAll(dataDir)
		}
	}
	return &InstallResult{
		Kind:            "plugin",
		ID:              pluginID,
		RestartRequired: false,
		Message:         "插件已卸载",
	}, nil
}

// InstallThemeFromZip validates theme.json + theme.css and atomically installs into themesDir/<id>.
func InstallThemeFromZip(zipPath, themesDir, expectedID string) (*InstallResult, error) {
	if strings.TrimSpace(themesDir) == "" {
		return nil, fmt.Errorf("主题目录未配置")
	}
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建主题目录失败: %w", err)
	}

	unpackTemp, err := os.MkdirTemp(themesDir, ".unpack-*")
	if err != nil {
		return nil, fmt.Errorf("创建主题临时目录失败: %w", err)
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(unpackTemp)
		}
	}()

	if err := SafeExtractZip(zipPath, unpackTemp); err != nil {
		return nil, err
	}

	rawMeta, err := os.ReadFile(filepath.Join(unpackTemp, "theme.json"))
	if err != nil {
		return nil, fmt.Errorf("缺少 theme.json: %w", err)
	}
	var meta theme.Meta
	if err := json.Unmarshal(rawMeta, &meta); err != nil {
		return nil, fmt.Errorf("解析 theme.json 失败: %w", err)
	}
	meta.ID = strings.TrimSpace(meta.ID)
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Version = strings.TrimSpace(meta.Version)
	if meta.ID == "" || strings.ContainsAny(meta.ID, `/\..`) {
		return nil, fmt.Errorf("无效的主题 ID: %s", meta.ID)
	}
	if expectedID != "" && meta.ID != expectedID {
		return nil, fmt.Errorf("主题包 ID (%s) 与目标 (%s) 不一致", meta.ID, expectedID)
	}
	if meta.Name == "" {
		meta.Name = meta.ID
	}
	cssBytes, err := os.ReadFile(filepath.Join(unpackTemp, "theme.css"))
	if err != nil || len(strings.TrimSpace(string(cssBytes))) == 0 {
		return nil, fmt.Errorf("主题包缺少有效的 theme.css")
	}

	targetDir := filepath.Join(themesDir, meta.ID)
	backupDir := filepath.Join(themesDir, ".bak-"+meta.ID)
	_ = os.RemoveAll(backupDir)
	_, targetStatErr := os.Stat(targetDir)
	targetExists := targetStatErr == nil
	if targetExists {
		if err := os.Rename(targetDir, backupDir); err != nil {
			return nil, fmt.Errorf("备份现有主题失败: %w", err)
		}
	}
	if err := os.Rename(unpackTemp, targetDir); err != nil {
		if targetExists {
			_ = os.Rename(backupDir, targetDir)
		}
		return nil, fmt.Errorf("安装主题失败: %w", err)
	}
	cleanupTemp = false
	_ = os.RemoveAll(backupDir)

	return &InstallResult{
		Kind:            "theme",
		ID:              meta.ID,
		Name:            meta.Name,
		Version:         meta.Version,
		RestartRequired: false,
		Message:         "主题安装完成，可立即切换使用",
	}, nil
}

// UninstallTheme removes an installed theme folder under themesDir.
func UninstallTheme(themesDir, themeID string) error {
	id := strings.TrimSpace(themeID)
	if id == "" || strings.ContainsAny(id, `/\..`) {
		return fmt.Errorf("无效的主题 ID: %s", themeID)
	}
	if strings.TrimSpace(themesDir) == "" {
		return fmt.Errorf("主题目录未配置")
	}
	targetDir := filepath.Join(themesDir, id)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return fmt.Errorf("主题未安装于当前目录: %s", id)
	}
	return os.RemoveAll(targetDir)
}
