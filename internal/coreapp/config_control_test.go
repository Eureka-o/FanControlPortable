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

	updated, err := app.commitConfigMutation(func(current *types.AppConfig) {
		current.Brightness = 42
	}, nil)
	if err != nil {
		t.Fatalf("commitConfigMutation() error = %v", err)
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

func TestCommitConfigMutationIfRevisionDefersEffectsUntilPersistenceSucceeds(t *testing.T) {
	blockedPath := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blockedPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("create blocked path: %v", err)
	}
	t.Setenv("USERPROFILE", blockedPath)
	t.Setenv("HOME", blockedPath)

	manager := config.NewManager(blockedPath, nil)
	manager.Set(types.GetDefaultConfig(false))
	before, revision := manager.GetWithRevision()
	app := &CoreApp{configManager: manager}
	effectCalls := 0

	_, _, applied, err := app.commitConfigMutationIfRevision(revision, func(current *types.AppConfig) {
		current.Brightness = 42
	}, func(types.AppConfig) {
		effectCalls++
	})
	if err == nil {
		t.Fatal("commitConfigMutationIfRevision() error = nil, want persistence failure")
	}
	if applied {
		t.Fatal("commitConfigMutationIfRevision() applied failed persistence")
	}
	after, afterRevision := manager.GetWithRevision()
	if after.Brightness != before.Brightness || afterRevision != revision {
		t.Fatalf("failed commit changed config: brightness=%d revision=%d", after.Brightness, afterRevision)
	}
	if effectCalls != 0 {
		t.Fatalf("post-commit effect calls = %d, want 0", effectCalls)
	}
}

func TestCommitConfigMutationRunsEffectOnceAfterCommit(t *testing.T) {
	manager := config.NewManager(t.TempDir(), nil)
	manager.Set(types.GetDefaultConfig(false))
	app := &CoreApp{configManager: manager}
	effectCalls := 0

	committed, err := app.commitConfigMutation(func(current *types.AppConfig) {
		current.Brightness = 42
	}, func(cfg types.AppConfig) {
		effectCalls++
		if cfg.Brightness != 42 || manager.Get().Brightness != 42 {
			t.Fatalf("post-commit effect observed uncommitted config: callback=%d stored=%d", cfg.Brightness, manager.Get().Brightness)
		}
	})
	if err != nil {
		t.Fatalf("commitConfigMutation() error = %v", err)
	}
	if committed.Brightness != 42 || effectCalls != 1 {
		t.Fatalf("commit result brightness=%d effect calls=%d", committed.Brightness, effectCalls)
	}
}

func TestCommitConfigMutationIfRevisionSkipsEffectOnConflict(t *testing.T) {
	manager := config.NewManager(t.TempDir(), nil)
	manager.Set(types.GetDefaultConfig(false))
	_, staleRevision := manager.GetWithRevision()
	manager.Set(types.AppConfig{Brightness: 77})
	app := &CoreApp{configManager: manager}
	effectCalls := 0

	committed, _, applied, err := app.commitConfigMutationIfRevision(staleRevision, func(current *types.AppConfig) {
		current.Brightness = 42
	}, func(types.AppConfig) {
		effectCalls++
	})
	if err != nil {
		t.Fatalf("commitConfigMutationIfRevision() error = %v", err)
	}
	if applied || committed.Brightness != 77 || effectCalls != 0 {
		t.Fatalf("conflict applied=%v brightness=%d effect calls=%d", applied, committed.Brightness, effectCalls)
	}
}
