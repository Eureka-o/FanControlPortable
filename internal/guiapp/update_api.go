package guiapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/version"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	updateInstallerName    = "FanControl-amd64-installer.exe"
	updateProgressEvent    = "update-download-progress"
	updateResponseTimeout  = 30 * time.Second
	updateMetadataTimeout  = 12 * time.Second
	updateMaxDownloadSize  = 256 * 1024 * 1024
	updateMaxMetadataSize  = 4 * 1024 * 1024
	updateDownloadAttempts = 3
	updateStableReleaseAPI = "https://api.github.com/repos/Eureka-o/FanControlPortable/releases/latest"
	updateReleaseListAPI   = "https://api.github.com/repos/Eureka-o/FanControlPortable/releases?per_page=30"
)

var errUpdateCanceled = errors.New("update download canceled")

type updateDownloadControl struct {
	mutex    sync.Mutex
	paused   bool
	canceled bool
	wake     chan struct{}
	cancel   context.CancelFunc
	done     chan struct{}
	doneOnce sync.Once
}

func newUpdateDownloadControl() *updateDownloadControl {
	return &updateDownloadControl{
		wake: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (c *updateDownloadControl) signalLocked() {
	close(c.wake)
	c.wake = make(chan struct{})
}

func (c *updateDownloadControl) Pause() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.canceled || c.paused {
		return false
	}
	c.paused = true
	c.signalLocked()
	return true
}

func (c *updateDownloadControl) Resume() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.canceled || !c.paused {
		return false
	}
	c.paused = false
	c.signalLocked()
	return true
}

func (c *updateDownloadControl) Cancel() {
	c.mutex.Lock()
	if c.canceled {
		c.mutex.Unlock()
		return
	}
	c.canceled = true
	c.paused = false
	cancel := c.cancel
	c.signalLocked()
	c.mutex.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *updateDownloadControl) setCancel(cancel context.CancelFunc) {
	c.mutex.Lock()
	if c.canceled {
		c.mutex.Unlock()
		if cancel != nil {
			cancel()
		}
		return
	}
	c.cancel = cancel
	c.mutex.Unlock()
}

func (c *updateDownloadControl) Wait(ctx context.Context) error {
	for {
		c.mutex.Lock()
		if c.canceled {
			c.mutex.Unlock()
			return errUpdateCanceled
		}
		if !c.paused {
			c.mutex.Unlock()
			return nil
		}
		wake := c.wake
		c.mutex.Unlock()

		select {
		case <-ctx.Done():
			c.mutex.Lock()
			canceled := c.canceled
			c.mutex.Unlock()
			if canceled {
				return errUpdateCanceled
			}
			return ctx.Err()
		case <-wake:
		}
	}
}

func (c *updateDownloadControl) IsCanceled() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.canceled
}

func (c *updateDownloadControl) finish() {
	c.doneOnce.Do(func() { close(c.done) })
}

type partialDownloadMetadata struct {
	URL          string `json:"url"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
}

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
	TagName         string `json:"tag_name"`
	HTMLURL         string `json:"html_url"`
	Body            string `json:"body"`
	Prerelease      bool   `json:"prerelease"`
	UpdateAvailable bool   `json:"update_available"`
	InstallerURL    string `json:"installer_url"`
	InstallerSHA256 string `json:"installer_sha256"`
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
		Digest      string `json:"digest"`
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
		release, err := checkLatestRelease(ctx, newUpdateHTTPClient(updateMetadataTimeout, updateMetadataTimeout, shouldBypassUpdateProxy(attempt, 2)), channel, updateStableReleaseAPI, updateReleaseListAPI)
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

func updateDownloadDirectory(installDir string) string {
	return filepath.Join(installDir, "updates")
}

func updatePartPath(updateDir, downloadURL string) string {
	urlHash := sha256.Sum256([]byte(downloadURL))
	return filepath.Join(updateDir, fmt.Sprintf("download-%x.part", urlHash[:8]))
}

func partialDownloadMetadataPath(partPath string) string {
	return partPath + ".json"
}

func cleanupPartialDownload(partPath string) error {
	var errs []error
	for _, path := range []string{partPath, partialDownloadMetadataPath(partPath)} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func cleanupSupersededPartialDownloads(updateDir, keepPartPath string) error {
	entries, err := os.ReadDir(updateDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	keepMetadataPath := partialDownloadMetadataPath(keepPartPath)
	var errs []error
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "download-") ||
			(!strings.HasSuffix(name, ".part") && !strings.HasSuffix(name, ".part.json")) {
			continue
		}
		path := filepath.Join(updateDir, name)
		if path == keepPartPath || path == keepMetadataPath {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func cleanupStaleUpdateArtifacts(legacyDir, updateDir string, cutoff time.Time) error {
	var errs []error
	if err := os.RemoveAll(legacyDir); err != nil {
		errs = append(errs, err)
	}

	entries, err := os.ReadDir(updateDir)
	if os.IsNotExist(err) {
		return errors.Join(errs...)
	}
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, entry := range entries {
		name := entry.Name()
		knownPart := strings.HasPrefix(name, "download-") &&
			(strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".part.json"))
		if entry.IsDir() || (!knownPart && name != updateInstallerName && !strings.EqualFold(name, "run-update.bat")) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(updateDir, name)); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (a *App) beginUpdateDownload() (*updateDownloadControl, error) {
	a.updateMutex.Lock()
	defer a.updateMutex.Unlock()
	if a.updateControl != nil {
		return nil, fmt.Errorf("an update download is already running")
	}
	control := newUpdateDownloadControl()
	a.updateControl = control
	return control, nil
}

func (a *App) finishUpdateDownload(control *updateDownloadControl) {
	control.finish()
	a.updateMutex.Lock()
	if a.updateControl == control {
		a.updateControl = nil
	}
	a.updateMutex.Unlock()
}

func (a *App) PauseUpdateDownload() bool {
	a.updateMutex.Lock()
	control := a.updateControl
	a.updateMutex.Unlock()
	if control == nil || !control.Pause() {
		return false
	}
	a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "paused"})
	return true
}

func (a *App) ResumeUpdateDownload() bool {
	a.updateMutex.Lock()
	control := a.updateControl
	a.updateMutex.Unlock()
	if control == nil || !control.Resume() {
		return false
	}
	a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "downloading"})
	return true
}

func (a *App) CancelUpdateDownload(downloadURL string) error {
	parsed, err := validateReleaseAssetURL(downloadURL)
	if err != nil {
		return err
	}
	updateDir := updateDownloadDirectory(config.GetInstallDir())
	partPath := updatePartPath(updateDir, parsed.String())

	a.updateMutex.Lock()
	control := a.updateControl
	a.updateMutex.Unlock()
	if control != nil {
		control.Cancel()
		select {
		case <-control.done:
		case <-time.After(3 * time.Second):
			return fmt.Errorf("timed out while canceling update download")
		}
	}
	if err := cleanupPartialDownload(partPath); err != nil {
		return fmt.Errorf("remove partial update: %w", err)
	}
	a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "canceled"})
	return nil
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
		TagName:         release.TagName,
		HTMLURL:         release.HTMLURL,
		Body:            release.Body,
		Prerelease:      release.Prerelease,
		UpdateAvailable: version.IsNewer(version.Get(), release.TagName),
	}
	for _, asset := range release.Assets {
		name := strings.ToLower(strings.TrimSpace(asset.Name))
		if strings.HasPrefix(name, "fancontrol-") && strings.HasSuffix(name, "-amd64-installer.exe") {
			digest, err := normalizeSHA256Digest(asset.Digest)
			if err != nil {
				return UpdateRelease{}, fmt.Errorf("invalid installer digest: %w", err)
			}
			result.InstallerURL = asset.DownloadURL
			result.InstallerSHA256 = digest
			break
		}
	}
	return result, nil
}

// DownloadAndInstallUpdate downloads and validates the release first, then
// starts the CMD installer after the GUI exits.
func (a *App) DownloadAndInstallUpdate(downloadURL, windowTitle, windowBody, windowRestarting, expectedSHA256 string) error {
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
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: err.Error(), MaxAttempts: updateDownloadAttempts})
		return err
	}
	expectedSHA256, err = normalizeSHA256Digest(expectedSHA256)
	if err != nil {
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: err.Error(), MaxAttempts: updateDownloadAttempts})
		return err
	}
	control, err := a.beginUpdateDownload()
	if err != nil {
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: err.Error(), MaxAttempts: updateDownloadAttempts})
		return err
	}
	defer a.finishUpdateDownload(control)
	a.emitUpdateProgress(updateProgress{Percent: 0, Stage: "downloading", Attempt: 1, MaxAttempts: updateDownloadAttempts})

	updateDir := updateDownloadDirectory(config.GetInstallDir())
	if err := os.MkdirAll(updateDir, 0o755); err != nil {
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: err.Error()})
		return fmt.Errorf("create update directory: %w", err)
	}
	installerPath := filepath.Join(updateDir, updateInstallerName)

	installerPath, err = a.downloadUpdateInstaller(parsed.String(), updateDir, expectedSHA256, control)
	if err != nil {
		if errors.Is(err, errUpdateCanceled) {
			_ = cleanupPartialDownload(updatePartPath(updateDir, parsed.String()))
			a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "canceled"})
			return err
		}
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

func newUpdateHTTPClient(timeout, responseHeaderTimeout time.Duration, bypassProxy bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = responseHeaderTimeout
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

func (a *App) downloadUpdateInstaller(downloadURL, updateDir, expectedSHA256 string, control *updateDownloadControl) (string, error) {
	target := filepath.Join(updateDir, updateInstallerName)
	partPath := updatePartPath(updateDir, downloadURL)
	if err := cleanupSupersededPartialDownloads(updateDir, partPath); err != nil {
		guiLogger.Warnf("clean superseded update downloads failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	control.setCancel(cancel)
	lastPercent := -1
	err := downloadWithRetryControlled(
		ctx,
		func(attempt int) *http.Client {
			return newUpdateHTTPClient(0, updateResponseTimeout, shouldBypassUpdateProxy(attempt, updateDownloadAttempts))
		},
		downloadURL,
		partPath,
		updateDownloadAttempts,
		control,
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
	if err := validateInstallerFile(partPath, expectedSHA256); err != nil {
		_ = cleanupPartialDownload(partPath)
		return "", err
	}
	_ = os.Remove(target)
	if err := os.Rename(partPath, target); err != nil {
		return "", fmt.Errorf("prepare update installer: %w", err)
	}
	_ = os.Remove(partialDownloadMetadataPath(partPath))
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
	return downloadWithRetryControlled(ctx, clientFactory, downloadURL, partPath, attempts, nil, onProgress, onRetry)
}

func downloadWithRetryControlled(
	ctx context.Context,
	clientFactory func(attempt int) *http.Client,
	downloadURL string,
	partPath string,
	attempts int,
	control *updateDownloadControl,
	onProgress func(received, total int64),
	onRetry func(attempt int, err error),
) error {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if control != nil {
			if err := control.Wait(ctx); err != nil {
				return err
			}
		}
		lastErr = downloadWithResume(ctx, control, clientFactory(attempt), downloadURL, partPath, onProgress)
		if lastErr == nil {
			return nil
		}
		if errors.Is(lastErr, errUpdateCanceled) || (control != nil && control.IsCanceled()) {
			return errUpdateCanceled
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

func loadPartialDownloadMetadata(path string) (partialDownloadMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return partialDownloadMetadata{}, err
	}
	var metadata partialDownloadMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return partialDownloadMetadata{}, err
	}
	return metadata, nil
}

func savePartialDownloadMetadata(path string, metadata partialDownloadMetadata) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func responseDownloadMetadata(downloadURL string, response *http.Response) partialDownloadMetadata {
	etag := strings.TrimSpace(response.Header.Get("ETag"))
	if strings.HasPrefix(strings.ToUpper(etag), "W/") {
		etag = ""
	}
	return partialDownloadMetadata{
		URL:          downloadURL,
		ETag:         etag,
		LastModified: strings.TrimSpace(response.Header.Get("Last-Modified")),
	}
}

func partialDownloadValidator(metadata partialDownloadMetadata) string {
	if metadata.ETag != "" {
		return metadata.ETag
	}
	return metadata.LastModified
}

func downloadWithResume(ctx context.Context, control *updateDownloadControl, client *http.Client, downloadURL, partPath string, onProgress func(received, total int64)) error {
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
	metadataPath := partialDownloadMetadataPath(partPath)
	metadata := partialDownloadMetadata{}
	if received > 0 {
		metadata, err = loadPartialDownloadMetadata(metadataPath)
		if err != nil || metadata.URL != downloadURL || partialDownloadValidator(metadata) == "" {
			if err := part.Truncate(0); err != nil {
				return err
			}
			if _, err := part.Seek(0, io.SeekStart); err != nil {
				return err
			}
			received = 0
			metadata = partialDownloadMetadata{}
			_ = os.Remove(metadataPath)
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	if received > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", received))
		request.Header.Set("If-Range", partialDownloadValidator(metadata))
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	var total int64
	responseMetadata := responseDownloadMetadata(downloadURL, response)
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
		if partialDownloadValidator(responseMetadata) != "" {
			if err := savePartialDownloadMetadata(metadataPath, responseMetadata); err != nil {
				return fmt.Errorf("save partial update metadata: %w", err)
			}
		} else {
			_ = os.Remove(metadataPath)
		}
	case http.StatusPartialContent:
		start, contentTotal, ok := parseDownloadContentRange(response.Header.Get("Content-Range"))
		if !ok || start != received || partialDownloadValidator(responseMetadata) == "" || partialDownloadValidator(responseMetadata) != partialDownloadValidator(metadata) {
			_ = part.Truncate(0)
			_ = os.Remove(metadataPath)
			return fmt.Errorf("server returned an invalid or stale resume range")
		}
		total = contentTotal
		if _, err := part.Seek(received, io.SeekStart); err != nil {
			return err
		}
	case http.StatusRequestedRangeNotSatisfiable:
		_ = part.Truncate(0)
		_ = os.Remove(metadataPath)
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
		if control != nil {
			if err := control.Wait(ctx); err != nil {
				return err
			}
		}
		count, readErr := reader.Read(buffer)
		if count > 0 {
			if control != nil {
				if err := control.Wait(ctx); err != nil {
					return err
				}
			}
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
			if control != nil && control.IsCanceled() {
				return errUpdateCanceled
			}
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

func normalizeSHA256Digest(digest string) (string, error) {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return "", nil
	}
	if algorithm, value, ok := strings.Cut(digest, ":"); ok {
		if !strings.EqualFold(strings.TrimSpace(algorithm), "sha256") {
			return "", fmt.Errorf("unsupported digest algorithm")
		}
		digest = strings.TrimSpace(value)
	}
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("invalid SHA-256 digest")
	}
	return strings.ToLower(digest), nil
}

func validateInstallerFile(path, expectedSHA256 string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read update installer: %w", err)
	}
	defer file.Close()
	var signature [2]byte
	if _, err := io.ReadFull(file, signature[:]); err != nil || signature != [2]byte{'M', 'Z'} {
		return fmt.Errorf("downloaded update is not a Windows installer")
	}
	expectedSHA256, err = normalizeSHA256Digest(expectedSHA256)
	if err != nil {
		return err
	}
	if expectedSHA256 == "" {
		return nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("read update installer: %w", err)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash update installer: %w", err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != expectedSHA256 {
		return fmt.Errorf("downloaded update failed SHA-256 verification")
	}
	return nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
