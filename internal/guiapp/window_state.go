package guiapp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/config"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	defaultWindowWidth  = 1024
	defaultWindowHeight = 768
	minWindowWidth      = 800
	minWindowHeight     = 600
)

type windowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximised bool `json:"maximised"`
}

var pendingWindowMaximise atomic.Bool

func windowStatePaths() []string {
	paths := []string{filepath.Join(config.GetInstallDir(), "config", "window-state.json")}
	if homeDir, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(appmeta.UserConfigDir(homeDir), "window-state.json"))
	}
	return paths
}

func loadWindowStateFile(path string) (windowState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return windowState{}, err
	}
	var state windowState
	err = json.Unmarshal(data, &state)
	return state, err
}

func saveWindowStateFile(path string, state windowState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadSavedWindowState() (windowState, error) {
	for _, path := range windowStatePaths() {
		state, err := loadWindowStateFile(path)
		if err == nil {
			return state, nil
		}
	}
	return windowState{}, os.ErrNotExist
}

func saveSavedWindowState(state windowState) error {
	var lastErr error
	for _, path := range windowStatePaths() {
		if err := saveWindowStateFile(path, state); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func normaliseWindowState(state windowState, screenWidth, screenHeight int) (windowState, bool) {
	if state.Width < minWindowWidth || state.Height < minWindowHeight || state.Width > 16384 || state.Height > 16384 {
		return windowState{}, false
	}
	if screenWidth > 0 {
		state.Width = min(state.Width, screenWidth)
		state.X = min(max(state.X, 0), max(screenWidth-100, 0))
	}
	if screenHeight > 0 {
		state.Height = min(state.Height, screenHeight)
		state.Y = min(max(state.Y, 0), max(screenHeight-80, 0))
	}
	return state, true
}

func restoreWindowState(ctx context.Context) error {
	state, err := loadSavedWindowState()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	screenWidth, screenHeight := 0, 0
	if screens, screenErr := wailsruntime.ScreenGetAll(ctx); screenErr == nil {
		for _, screen := range screens {
			if screen.IsCurrent || (screenWidth == 0 && screen.IsPrimary) {
				screenWidth, screenHeight = screen.Size.Width, screen.Size.Height
				if screenWidth == 0 || screenHeight == 0 {
					screenWidth, screenHeight = screen.Width, screen.Height
				}
				if screen.IsCurrent {
					break
				}
			}
		}
	}
	state, ok := normaliseWindowState(state, screenWidth, screenHeight)
	if !ok {
		return nil
	}
	wailsruntime.WindowSetSize(ctx, state.Width, state.Height)
	wailsruntime.WindowSetPosition(ctx, state.X, state.Y)
	if state.Maximised {
		if launchedWithAutoStart() {
			pendingWindowMaximise.Store(true)
		} else {
			wailsruntime.WindowMaximise(ctx)
		}
	}
	return nil
}

func saveCurrentWindowState(ctx context.Context) error {
	state, err := loadSavedWindowState()
	if err != nil {
		state = windowState{Width: defaultWindowWidth, Height: defaultWindowHeight}
	}
	if wailsruntime.WindowIsMaximised(ctx) {
		state.Maximised = true
	} else if wailsruntime.WindowIsNormal(ctx) {
		state.Maximised = false
		state.X, state.Y = wailsruntime.WindowGetPosition(ctx)
		state.Width, state.Height = wailsruntime.WindowGetSize(ctx)
	}
	return saveSavedWindowState(state)
}

func restorePendingWindowMaximise(ctx context.Context) {
	if pendingWindowMaximise.Swap(false) {
		wailsruntime.WindowMaximise(ctx)
	}
}

func launchedWithAutoStart() bool {
	for _, arg := range os.Args {
		switch strings.ToLower(strings.TrimSpace(arg)) {
		case "--autostart", "/autostart", "-autostart":
			return true
		}
	}
	return false
}
