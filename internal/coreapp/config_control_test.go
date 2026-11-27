package coreapp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Eureka-o/FanControlPortable/internal/config"
	"github.com/Eureka-o/FanControlPortable/internal/temperature"
	"github.com/Eureka-o/FanControlPortable/internal/types"
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

func TestMonitorOnlyRejectsLightingControls(t *testing.T) {
	app := &CoreApp{}
	app.monitorOnly.Store(true)
	if app.SetGearLight(true) {
		t.Fatal("SetGearLight() succeeded in monitor-only mode")
	}
	if app.SetBrightness(50) {
		t.Fatal("SetBrightness() succeeded in monitor-only mode")
	}
	if err := app.SetLightStrip(types.GetDefaultLightStripConfig()); err == nil {
		t.Fatal("SetLightStrip() succeeded in monitor-only mode")
	}
	if err := app.applyConfiguredLightStrip(); err != nil {
		t.Fatalf("applyConfiguredLightStrip() in monitor-only mode = %v, want nil", err)
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

func TestSetTemperatureHistoryEnabledPersistsAndSurvivesUpdateConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOME", dir)

	manager := config.NewManager(dir, nil)
	manager.Load(false)
	recorder := temperature.NewHistoryRecorder(filepath.Join(dir, "telemetry", "history.bin"), 720, 5*time.Second, nil)
	app := &CoreApp{configManager: manager, tempHistory: recorder}

	if err := app.SetTemperatureHistoryEnabled(false); err != nil {
		t.Fatalf("SetTemperatureHistoryEnabled(false) error = %v", err)
	}
	if recorder.IsEnabled() {
		t.Fatal("recorder remained enabled after disabling")
	}
	if got := manager.Get().HistoryEnabled; got == nil || *got {
		t.Fatalf("HistoryEnabled = %v, want false", got)
	}

	reloaded := config.NewManager(dir, nil).Load(false)
	if reloaded.HistoryEnabled == nil || *reloaded.HistoryEnabled {
		t.Fatalf("reloaded HistoryEnabled = %v, want false", reloaded.HistoryEnabled)
	}

	// UpdateConfig with stale defaults must not overwrite persisted HistoryEnabled.
	if err := app.UpdateConfig(types.GetDefaultConfig(false)); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}
	if got := manager.Get().HistoryEnabled; got == nil || *got {
		t.Fatalf("UpdateConfig overwrote HistoryEnabled = %v, want false", got)
	}

	if err := app.SetTemperatureHistoryEnabled(true); err != nil {
		t.Fatalf("SetTemperatureHistoryEnabled(true) error = %v", err)
	}
	if !recorder.IsEnabled() {
		t.Fatal("recorder remained disabled after enabling")
	}
	if got := manager.Get().HistoryEnabled; got == nil || !*got {
		t.Fatalf("HistoryEnabled = %v, want true", got)
	}
}

func TestInitStartupTemperatureHistoryMigratesLegacyAndPreservesRetention(t *testing.T) {
	legacyDir := t.TempDir()
	t.Setenv("USERPROFILE", legacyDir)
	t.Setenv("HOME", legacyDir)

	legacyCfgPath := filepath.Join(legacyDir, "config", "config.json")
	if err := os.MkdirAll(filepath.Dir(legacyCfgPath), 0755); err != nil {
		t.Fatalf("mkdir legacy config: %v", err)
	}
	legacyJSON := `{"themeMode":"dune","temperatureHistoryRetentionHours":6,"fanCurveProfiles":[{"id":"default","name":"默认","curve":[{"temperature":20,"rpm":994},{"temperature":110,"rpm":1975}]},{"id":"p1","name":"性能","curve":[{"temperature":20,"rpm":1200},{"temperature":110,"rpm":2500}]}]}`
	if err := os.WriteFile(legacyCfgPath, []byte(legacyJSON), 0644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
	historyPath := filepath.Join(legacyDir, "telemetry", "history.bin")
	seededRecorder := temperature.NewHistoryRecorder(historyPath, 6*720, 5*time.Second, nil)
	baseTs := time.Now().Add(-2 * time.Hour).UnixMilli()
	for i := 0; i < 800; i++ {
		ts := baseTs + int64(i)*5000
		if _, ok := seededRecorder.Add(types.TemperatureData{UpdateTime: ts, CPUTemp: 55}, nil); !ok {
			t.Fatalf("seed point %d failed", i)
		}
	}
	if err := seededRecorder.Flush(); err != nil {
		t.Fatalf("flush seeded history: %v", err)
	}

	legacyManager := config.NewManager(legacyDir, nil)
	// Constructing manager before Load() must not touch existing config on disk.
	beforeLoadBytes, err := os.ReadFile(legacyCfgPath)
	if err != nil {
		t.Fatalf("read config before Load: %v", err)
	}
	if string(beforeLoadBytes) != legacyJSON {
		t.Fatal("NewManager mutated existing config.json before Load()")
	}

	loadedLegacy := legacyManager.Load(false)
	if loadedLegacy.HistoryEnabled != nil {
		t.Fatalf("legacy HistoryEnabled = %v, want nil before init", *loadedLegacy.HistoryEnabled)
	}
	app := &CoreApp{configManager: legacyManager, historyPath: historyPath}
	if changed := app.initStartupTemperatureHistory(&loadedLegacy); !changed {
		t.Fatal("initStartupTemperatureHistory() changed = false, want true for legacy nil migration")
	}
	if loadedLegacy.HistoryEnabled == nil || !*loadedLegacy.HistoryEnabled {
		t.Fatalf("migrated HistoryEnabled = %v, want true", loadedLegacy.HistoryEnabled)
	}
	if app.tempHistory.RetentionHours() != 6 {
		t.Fatalf("RetentionHours() = %d, want 6", app.tempHistory.RetentionHours())
	}
	if gotPoints := len(app.tempHistory.Snapshot().Points); gotPoints != 800 {
		t.Fatalf("loaded history points = %d, want 800 (must not truncate >1h points)", gotPoints)
	}
	if len(loadedLegacy.FanCurveProfiles) != 2 || loadedLegacy.ThemeMode != "dune" {
		t.Fatalf("startup history init lost user config fields: profiles=%d theme=%q", len(loadedLegacy.FanCurveProfiles), loadedLegacy.ThemeMode)
	}

	configuredDir := t.TempDir()
	t.Setenv("USERPROFILE", configuredDir)
	t.Setenv("HOME", configuredDir)
	configuredManager := config.NewManager(configuredDir, nil)
	cfg := configuredManager.Load(false)
	disabled := false
	cfg.HistoryEnabled = &disabled
	if err := configuredManager.Update(cfg); err != nil {
		t.Fatalf("Update() configured disabled: %v", err)
	}
	loadedCfg := configuredManager.Load(false)
	app2 := &CoreApp{
		configManager: configuredManager,
		historyPath:   filepath.Join(configuredDir, "telemetry", "history.bin"),
	}
	if changed := app2.initStartupTemperatureHistory(&loadedCfg); changed {
		t.Fatal("initStartupTemperatureHistory() reported changed = true when HistoryEnabled already present")
	}
	if app2.tempHistory.IsEnabled() {
		t.Fatal("initStartupTemperatureHistory() did not enforce config disabled state on recorder")
	}
}
