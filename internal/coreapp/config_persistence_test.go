package coreapp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/types"
)

func TestPersistConfigUpdateReturnsSaveError(t *testing.T) {
	blockedPath := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blockedPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("create blocked path: %v", err)
	}
	t.Setenv("USERPROFILE", blockedPath)
	t.Setenv("HOME", blockedPath)

	manager := config.NewManager(blockedPath, nil)
	initial := types.GetDefaultConfig(false)
	manager.Set(initial)
	app := &CoreApp{configManager: manager}

	next := initial
	next.GearLight = !initial.GearLight
	if err := app.commitConfigUpdate(next, nil); err == nil {
		t.Fatal("commitConfigUpdate() error = nil, want persistence failure")
	}
}
