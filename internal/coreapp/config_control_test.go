package coreapp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/types"
)

func TestSetAutoControlKeepsConfigWhenPersistenceFails(t *testing.T) {
	blockedPath := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blockedPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("create blocked path: %v", err)
	}
	t.Setenv("USERPROFILE", blockedPath)
	t.Setenv("HOME", blockedPath)

	manager := config.NewManager(blockedPath, nil)
	initial := types.GetDefaultConfig(false)
	initial.AutoControl = true
	manager.Set(initial)
	app := &CoreApp{configManager: manager}

	if err := app.SetAutoControl(false); err == nil {
		t.Fatal("SetAutoControl() error = nil, want persistence failure")
	}
	if got := manager.Get().AutoControl; !got {
		t.Fatal("AutoControl changed after failed persistence")
	}
}

func TestSetAutoControlDefersRuntimeEffectsUntilPersistenceSucceeds(t *testing.T) {
	blockedPath := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blockedPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("create blocked path: %v", err)
	}
	t.Setenv("USERPROFILE", blockedPath)
	t.Setenv("HOME", blockedPath)

	manager := config.NewManager(blockedPath, nil)
	initial := types.GetDefaultConfig(false)
	initial.AutoControl = false
	manager.Set(initial)
	app := &CoreApp{configManager: manager}

	if err := app.SetAutoControl(true); err == nil {
		t.Fatal("SetAutoControl() error = nil, want persistence failure")
	}
	if app.userSetAutoControl || app.forceNextAutoTarget.Load() {
		t.Fatal("runtime auto-control effects applied before persistence succeeded")
	}
}

func TestPersistConfigMutationCommitsNarrowChange(t *testing.T) {
	manager := config.NewManager(t.TempDir(), nil)
	manager.Set(types.GetDefaultConfig(false))
	app := &CoreApp{configManager: manager}

	updated, err := app.persistConfigMutation(func(current *types.AppConfig) {
		current.Brightness = 42
	})
	if err != nil {
		t.Fatalf("persistConfigMutation() error = %v", err)
	}
	if updated.Brightness != 42 || manager.Get().Brightness != 42 {
		t.Fatalf("Brightness not committed: returned=%d stored=%d", updated.Brightness, manager.Get().Brightness)
	}
}

func TestSetCustomSpeedKeepsConfigWhenPersistenceFails(t *testing.T) {
	blockedPath := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blockedPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("create blocked path: %v", err)
	}
	t.Setenv("USERPROFILE", blockedPath)
	t.Setenv("HOME", blockedPath)

	manager := config.NewManager(blockedPath, nil)
	initial := types.GetDefaultConfig(false)
	initial.CustomSpeedEnabled = true
	manager.Set(initial)
	app := &CoreApp{configManager: manager}

	if err := app.SetCustomSpeed(false, initial.CustomSpeedRPM); err == nil {
		t.Fatal("SetCustomSpeed() error = nil, want persistence failure")
	}
	if got := manager.Get().CustomSpeedEnabled; !got {
		t.Fatal("CustomSpeedEnabled changed after failed persistence")
	}
}
