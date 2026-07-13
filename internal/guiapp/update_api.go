package guiapp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	updateInstallerName    = "FanControl-amd64-installer.exe"
	updateProgressEvent    = "update-download-progress"
	updateDownloadTimeout  = 10 * time.Minute
	updateMetadataTimeout  = 12 * time.Second
	updateMaxDownloadSize  = 256 * 1024 * 1024
	updateMaxMetadataSize  = 4 * 1024 * 1024
	updateDownloadAttempts = 3
	updateStableReleaseAPI = "https://api.github.com/repos/Eureka-o/FanControlPortable/releases/latest"
	updateReleaseListAPI   = "https://api.github.com/repos/Eureka-o/FanControlPortable/releases?per_page=30"
)

type updateProgress struct {
	Percent     int    `json:"percent"`
	Received    int64  `json:"received"`
	Total       int64  `json:"total"`
	Stage       string `json:"stage"`
	Message     string `json:"message"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"maxAttempts"`
}

type UpdateRelease struct {
	TagName      string `json:"tag_name"`
	HTMLURL      string `json:"html_url"`
	Body         string `json:"body"`
	Prerelease   bool   `json:"prerelease"`
	InstallerURL string `json:"installer_url"`
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Body       string `json:"body"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
	Assets     []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (a *App) emitUpdateProgress(progress updateProgress) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, updateProgressEvent, progress)
	}
}

// CheckLatestRelease keeps release metadata on the same proxy-aware network
// path as the installer download.
func (a *App) CheckLatestRelease(channel string) (UpdateRelease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), updateMetadataTimeout)
	defer cancel()

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		release, err := checkLatestRelease(ctx, newUpdateHTTPClient(updateMetadataTimeout, shouldBypassUpdateProxy(attempt, 2)), channel, updateStableReleaseAPI, updateReleaseListAPI)
		if err == nil {
			return release, nil
		}
		lastErr = err
		if attempt < 2 {
			if err := waitForUpdateRetry(ctx, 300*time.Millisecond); err != nil {
				break
			}
		}
	}
	return UpdateRelease{}, fmt.Errorf("check for updates failed: %w", lastErr)
}

func (a *App) UpdateCompletedOnLaunch() bool {
	return hasUpdateCompletedArg(os.Args)
}

func hasUpdateCompletedArg(args []string) bool {
	for _, arg := range args[1:] {
		if strings.EqualFold(strings.TrimSpace(arg), "--update-complete") {
			return true
		}
	}
	return false
}

func checkLatestRelease(ctx context.Context, client *http.Client, channel, stableURL, releasesURL string) (UpdateRelease, error) {
	isPrerelease := strings.EqualFold(strings.TrimSpace(channel), "prerelease")
	endpoint := stableURL
	if isPrerelease {
		endpoint = releasesURL
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return UpdateRelease{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "FanControl-Updater")

	response, err := client.Do(request)
	if err != nil {
		return UpdateRelease{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return UpdateRelease{}, fmt.Errorf("GitHub API returned HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, updateMaxMetadataSize+1))
	if err != nil {
		return UpdateRelease{}, err
	}
	if len(body) > updateMaxMetadataSize {
		return UpdateRelease{}, fmt.Errorf("release metadata exceeds the size limit")
	}

	var release githubRelease
	if isPrerelease {
		var releases []githubRelease
		if err := json.Unmarshal(body, &releases); err != nil {
			return UpdateRelease{}, fmt.Errorf("decode release metadata: %w", err)
		}
		for _, candidate := range releases {
			if !candidate.Draft && candidate.Prerelease {
				release = candidate
				break
			}
		}
		if release.TagName == "" {
			return UpdateRelease{}, nil
		}
	} else if err := json.Unmarshal(body, &release); err != nil {
		return UpdateRelease{}, fmt.Errorf("decode release metadata: %w", err)
	}

	result := UpdateRelease{
		TagName:    release.TagName,
		HTMLURL:    release.HTMLURL,
		Body:       release.Body,
		Prerelease: release.Prerelease,
	}
	for _, asset := range release.Assets {
		name := strings.ToLower(strings.TrimSpace(asset.Name))
		if strings.HasPrefix(name, "fancontrol-") && strings.HasSuffix(name, "-amd64-installer.exe") {
			result.InstallerURL = asset.DownloadURL
			break
		}
	}
	return result, nil
}

// DownloadAndInstallUpdate downloads and validates the release first, then
// starts the CMD installer after the GUI exits.
func (a *App) DownloadAndInstallUpdate(downloadURL, windowTitle, windowBody, windowRestarting string) error {
	if strings.TrimSpace(windowTitle) == "" {
		windowTitle = "FanControl is updating"
	}
	if strings.TrimSpace(windowBody) == "" {
		windowBody = "Installing the new version. Please keep this window open"
	}
	if strings.TrimSpace(windowRestarting) == "" {
		windowRestarting = "Update complete. Restarting FanControl"
	}

	parsed, err := validateReleaseAssetURL(downloadURL)
	if err != nil {
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: err.Error()})
		return err
	}

	updateDir := filepath.Join(os.TempDir(), "FanControl-update")
	if err := os.MkdirAll(updateDir, 0o755); err != nil {
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: err.Error()})
		return fmt.Errorf("create update directory: %w", err)
	}
	installerPath := filepath.Join(updateDir, updateInstallerName)
	_ = os.Remove(installerPath)

	installerPath, err = a.downloadUpdateInstaller(parsed.String())
	if err != nil {
		guiLogger.Errorf("update installer download failed: %v", err)
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: err.Error(), MaxAttempts: updateDownloadAttempts})
		return err
	}

	a.emitUpdateProgress(updateProgress{Percent: 100, Stage: "installing"})

	exePath, exeErr := os.Executable()
	if exeErr != nil {
		guiLogger.Warnf("failed to resolve current executable path: %v", exeErr)
		exePath = ""
	}
	if err := launchUpdateInstaller(installerPath, exePath, windowTitle, windowBody, windowRestarting); err != nil {
		guiLogger.Errorf("failed to launch update installer: %v", err)
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: err.Error()})
		return err
	}

	go func() {
		time.Sleep(800 * time.Millisecond)
		if a.ctx != nil {
			runtime.Quit(a.ctx)
			return
		}
		os.Exit(0)
	}()
	return nil
}

func validateUpdateURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return nil, fmt.Errorf("invalid update download URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && !strings.HasSuffix(host, ".github.com") &&
		host != "githubusercontent.com" && !strings.HasSuffix(host, ".githubusercontent.com") &&
		host != "objects.githubusercontent.com" {
		return nil, fmt.Errorf("unsupported update download source")
	}
	return parsed, nil
}

func validateReleaseAssetURL(raw string) (*url.URL, error) {
	parsed, err := validateUpdateURL(raw)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(parsed.Hostname()) != "github.com" ||
		!strings.HasPrefix(parsed.Path, "/Eureka-o/FanControlPortable/releases/download/") {
		return nil, fmt.Errorf("URL is not a FanControl release asset")
	}
	return parsed, nil
}

func newUpdateHTTPClient(timeout time.Duration, bypassProxy bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if bypassProxy {
		transport.Proxy = nil
	} else {
		enabled, server, err := readUpdateSystemProxy()
		if err != nil {
			transport.Proxy = func(*http.Request) (*url.URL, error) {
				return nil, fmt.Errorf("read Windows proxy settings: %w", err)
			}
		} else if enabled && strings.TrimSpace(server) != "" {
			transport.Proxy = func(request *http.Request) (*url.URL, error) {
				return parseWindowsProxyServer(server, request.URL.Scheme)
			}
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(next *http.Request, previous []*http.Request) error {
			if len(previous) >= 10 {
				return fmt.Errorf("too many update redirects")
			}
			_, err := validateUpdateURL(next.URL.String())
			return err
		},
	}
}

func parseWindowsProxyServer(value, requestScheme string) (*url.URL, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("Windows proxy server is empty")
	}

	selected := value
	selectedKind := "http"
	if strings.Contains(value, "=") {
		entries := make(map[string]string)
		for _, entry := range strings.Split(value, ";") {
			key, server, ok := strings.Cut(entry, "=")
			if ok && strings.TrimSpace(server) != "" {
				entries[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(server)
			}
		}
		requestScheme = strings.ToLower(strings.TrimSpace(requestScheme))
		if server := entries[requestScheme]; server != "" {
			selected = server
			selectedKind = requestScheme
		} else if server := entries["http"]; server != "" {
			selected = server
			selectedKind = "http"
		} else if server := entries["socks"]; server != "" {
			selected = server
			selectedKind = "socks"
		} else {
			return nil, fmt.Errorf("Windows proxy has no server for %s", requestScheme)
		}
	}

	if !strings.Contains(selected, "://") {
		if selectedKind == "socks" {
			selected = "socks5://" + selected
		} else {
			selected = "http://" + selected
		}
	}
	proxyURL, err := url.Parse(selected)
	if err != nil || proxyURL.Hostname() == "" {
		return nil, fmt.Errorf("invalid Windows proxy server")
	}
	return proxyURL, nil
}

func (a *App) downloadUpdateInstaller(downloadURL string) (string, error) {
	dir := filepath.Join(os.TempDir(), "FanControl-update")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create update directory: %w", err)
	}
	target := filepath.Join(dir, updateInstallerName)
	urlHash := sha256.Sum256([]byte(downloadURL))
	partPath := filepath.Join(dir, fmt.Sprintf("download-%x.part", urlHash[:8]))

	ctx, cancel := context.WithTimeout(context.Background(), updateDownloadTimeout)
	defer cancel()
	lastPercent := -1
	err := downloadWithRetry(
		ctx,
		func(attempt int) *http.Client {
			return newUpdateHTTPClient(updateDownloadTimeout, shouldBypassUpdateProxy(attempt, updateDownloadAttempts))
		},
		downloadURL,
		partPath,
		updateDownloadAttempts,
		func(received, total int64) {
			percent := -1
			if total > 0 {
				percent = int(received * 100 / total)
			}
			if percent == lastPercent || (percent >= 0 && percent != 100 && percent%2 != 0) {
				return
			}
			lastPercent = percent
			a.emitUpdateProgress(updateProgress{Percent: percent, Received: received, Total: max64(total, 0), Stage: "downloading", MaxAttempts: updateDownloadAttempts})
		},
		func(attempt int, retryErr error) {
			a.emitUpdateProgress(updateProgress{Percent: lastPercent, Stage: "retrying", Message: retryErr.Error(), Attempt: attempt, MaxAttempts: updateDownloadAttempts})
		},
	)
	if err != nil {
		return "", fmt.Errorf("download failed after %d attempts: %w", updateDownloadAttempts, err)
	}
	if err := validateInstallerFile(partPath); err != nil {
		return "", err
	}
	_ = os.Remove(target)
	if err := os.Rename(partPath, target); err != nil {
		return "", fmt.Errorf("prepare update installer: %w", err)
	}
	return target, nil
}

func downloadWithRetry(
	ctx context.Context,
	clientFactory func(attempt int) *http.Client,
	downloadURL string,
	partPath string,
	attempts int,
	onProgress func(received, total int64),
	onRetry func(attempt int, err error),
) error {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = downloadWithResume(ctx, clientFactory(attempt), downloadURL, partPath, onProgress)
		if lastErr == nil {
			return nil
		}
		if attempt == attempts {
			break
		}
		if onRetry != nil {
			onRetry(attempt+1, lastErr)
		}
		if err := waitForUpdateRetry(ctx, time.Duration(attempt)*350*time.Millisecond); err != nil {
			return err
		}
	}
	return lastErr
}

func shouldBypassUpdateProxy(attempt, attempts int) bool {
	return attempts > 1 && attempt >= attempts
}

func downloadWithResume(ctx context.Context, client *http.Client, downloadURL, partPath string, onProgress func(received, total int64)) error {
	part, err := os.OpenFile(partPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open partial update: %w", err)
	}
	defer part.Close()
	info, err := part.Stat()
	if err != nil {
		return err
	}
	received := info.Size()
	if received > updateMaxDownloadSize {
		return fmt.Errorf("partial update exceeds the size limit")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	if received > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", received))
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	var total int64
	switch response.StatusCode {
	case http.StatusOK:
		if err := part.Truncate(0); err != nil {
			return err
		}
		if _, err := part.Seek(0, io.SeekStart); err != nil {
			return err
		}
		received = 0
		total = response.ContentLength
	case http.StatusPartialContent:
		start, contentTotal, ok := parseDownloadContentRange(response.Header.Get("Content-Range"))
		if !ok || start != received {
			_ = part.Truncate(0)
			return fmt.Errorf("server returned an invalid resume range")
		}
		total = contentTotal
		if _, err := part.Seek(received, io.SeekStart); err != nil {
			return err
		}
	case http.StatusRequestedRangeNotSatisfiable:
		if totalSize, ok := parseUnsatisfiedContentRange(response.Header.Get("Content-Range")); ok && totalSize == received {
			return nil
		}
		_ = part.Truncate(0)
		return fmt.Errorf("server rejected the resume range")
	default:
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	if total > updateMaxDownloadSize {
		return fmt.Errorf("update installer exceeds the size limit")
	}
	if onProgress != nil {
		onProgress(received, total)
	}

	reader := io.LimitReader(response.Body, updateMaxDownloadSize-received+1)
	buffer := make([]byte, 64*1024)
	for {
		count, readErr := reader.Read(buffer)
		if count > 0 {
			if _, writeErr := part.Write(buffer[:count]); writeErr != nil {
				return fmt.Errorf("write update installer: %w", writeErr)
			}
			received += int64(count)
			if received > updateMaxDownloadSize {
				return fmt.Errorf("update installer exceeds the size limit")
			}
			if onProgress != nil {
				onProgress(received, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("download interrupted: %w", readErr)
		}
	}
	if total > 0 && received != total {
		return fmt.Errorf("download interrupted at %d of %d bytes", received, total)
	}
	if err := part.Sync(); err != nil {
		return fmt.Errorf("flush update installer: %w", err)
	}
	return nil
}

func parseDownloadContentRange(value string) (start, total int64, ok bool) {
	var end int64
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "bytes %d-%d/%d", &start, &end, &total); err != nil {
		return 0, 0, false
	}
	return start, total, start >= 0 && end >= start && total > end
}

func parseUnsatisfiedContentRange(value string) (total int64, ok bool) {
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "bytes */%d", &total); err != nil {
		return 0, false
	}
	return total, total >= 0
}

func waitForUpdateRetry(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validateInstallerFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read update installer: %w", err)
	}
	defer file.Close()
	var signature [2]byte
	if _, err := io.ReadFull(file, signature[:]); err != nil || signature != [2]byte{'M', 'Z'} {
		return fmt.Errorf("downloaded update is not a Windows installer")
	}
	return nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
