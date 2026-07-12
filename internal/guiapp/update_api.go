package guiapp

import (
	"context"
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
	updateInstallerName   = "FanControl-amd64-installer.exe"
	updateProgressEvent   = "update-download-progress"
	updateDownloadTimeout = 10 * time.Minute
	updateMaxDownloadSize = 256 * 1024 * 1024
)

type updateProgress struct {
	Percent  int    `json:"percent"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Stage    string `json:"stage"`
	Message  string `json:"message"`
}

func (a *App) emitUpdateProgress(progress updateProgress) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, updateProgressEvent, progress)
	}
}

// DownloadAndInstallUpdate downloads a release installer, starts it after the
// current GUI exits, and lets the existing installer preserve user data.
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

	installerPath, err := a.downloadUpdateInstaller(parsed.String())
	if err != nil {
		guiLogger.Errorf("update installer download failed: %v", err)
		a.emitUpdateProgress(updateProgress{Percent: -1, Stage: "error", Message: err.Error()})
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
		a.emitUpdateProgress(updateProgress{Percent: 100, Stage: "error", Message: err.Error()})
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

func (a *App) downloadUpdateInstaller(downloadURL string) (string, error) {
	dir := filepath.Join(os.TempDir(), "FanControl-update")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create update directory: %w", err)
	}
	target := filepath.Join(dir, updateInstallerName)
	part, err := os.CreateTemp(dir, "download-*.part")
	if err != nil {
		return "", fmt.Errorf("create download file: %w", err)
	}
	partPath := part.Name()
	defer os.Remove(partPath)

	ctx, cancel := context.WithTimeout(context.Background(), updateDownloadTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		_ = part.Close()
		return "", fmt.Errorf("create download request: %w", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	client := &http.Client{
		Timeout: updateDownloadTimeout,
		CheckRedirect: func(next *http.Request, _ []*http.Request) error {
			_, err := validateUpdateURL(next.URL.String())
			return err
		},
	}
	response, err := client.Do(request)
	if err != nil {
		_ = part.Close()
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_ = part.Close()
		return "", fmt.Errorf("download failed: HTTP %d", response.StatusCode)
	}
	if response.ContentLength > updateMaxDownloadSize {
		_ = part.Close()
		return "", fmt.Errorf("update installer exceeds the size limit")
	}

	total := response.ContentLength
	a.emitUpdateProgress(updateProgress{Percent: 0, Total: max64(total, 0), Stage: "downloading"})
	reader := io.LimitReader(response.Body, updateMaxDownloadSize+1)
	var received int64
	lastPercent := -1
	buffer := make([]byte, 64*1024)
	for {
		count, readErr := reader.Read(buffer)
		if count > 0 {
			if _, writeErr := part.Write(buffer[:count]); writeErr != nil {
				_ = part.Close()
				return "", fmt.Errorf("write update installer: %w", writeErr)
			}
			received += int64(count)
			if received > updateMaxDownloadSize {
				_ = part.Close()
				return "", fmt.Errorf("update installer exceeds the size limit")
			}
			percent := -1
			if total > 0 {
				percent = int(received * 100 / total)
			}
			if percent != lastPercent && (percent < 0 || percent == 100 || percent%2 == 0) {
				lastPercent = percent
				a.emitUpdateProgress(updateProgress{Percent: percent, Received: received, Total: max64(total, 0), Stage: "downloading"})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = part.Close()
			return "", fmt.Errorf("download interrupted: %w", readErr)
		}
	}
	if err := part.Sync(); err != nil {
		_ = part.Close()
		return "", fmt.Errorf("flush update installer: %w", err)
	}
	if err := part.Close(); err != nil {
		return "", fmt.Errorf("close update installer: %w", err)
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
