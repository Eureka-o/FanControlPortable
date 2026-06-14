package guiapp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/types"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ExportDiagnosticsToFile() (string, error) {
	resp, err := a.sendRequestWithTimeout(ipc.ReqExportDiagnostics, nil, 30*time.Second)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}

	var bundle types.DiagnosticsBundle
	if err := json.Unmarshal(resp.Data, &bundle); err != nil {
		return "", err
	}
	if strings.TrimSpace(bundle.DataBase64) == "" {
		return "", fmt.Errorf("诊断包为空")
	}

	data, err := base64.StdEncoding.DecodeString(bundle.DataBase64)
	if err != nil {
		return "", err
	}
	if a.ctx == nil {
		return "", fmt.Errorf("window context is not ready")
	}

	defaultName := strings.TrimSpace(bundle.FileName)
	if defaultName == "" {
		defaultName = "FanControl-diagnostics.zip"
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出诊断日志",
		DefaultFilename: defaultName,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "FanControl Diagnostics (*.zip)", Pattern: "*.zip"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if filepath.Ext(path) == "" {
		path += ".zip"
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
