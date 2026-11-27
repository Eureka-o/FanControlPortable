package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		runHelperSidecarProcess()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runHelperSidecarProcess acts as the fake external sidecar executable in subprocess tests.
func runHelperSidecarProcess() {

	mode := os.Getenv("FC_TEST_SIDECAR_MODE")
	pluginID := os.Getenv("FC_TEST_PLUGIN_ID")
	if pluginID == "" {
		pluginID = "test-sidecar"
	}

	if mode == "crash-immediately" {
		fmt.Fprintln(os.Stderr, "simulated immediate crash")
		os.Exit(2)
	}
	if mode == "unsupported" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"type":            "hello",
			"protocolVersion": 1,
			"pluginId":        pluginID,
			"version":         "1.0.0",
			"supported":       false,
			"reason":          "hardware not found",
		})
		os.Exit(0)
	}

	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"type":            "hello",
		"protocolVersion": 1,
		"pluginId":        pluginID,
		"version":         "1.0.0",
		"supported":       true,
		"capabilities":    []string{"status", "fan-telemetry"},
	})

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var env sidecarEnvelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			continue
		}
		switch env.Type {
		case "request":
			switch env.Method {
			case "ping":
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "response", "id": env.ID, "ok": true, "payload": map[string]any{"pong": true}})
			case "echo":
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "response", "id": env.ID, "ok": true, "payload": env.Payload})
			case "slow":
				go func(id string) {
					time.Sleep(400 * time.Millisecond)
					_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "response", "id": id, "ok": true, "payload": map[string]any{"late": true}})
				}(env.ID)
			case "emit":
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "event", "event": "telemetry-ack", "payload": env.Payload})
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "response", "id": env.ID, "ok": true, "payload": map[string]any{"emitted": true}})
			case "crash-now":
				fmt.Fprintln(os.Stderr, "triggered crash")
				os.Exit(3)
			case "prepare-suspend":
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "event", "event": "prepared-suspend", "payload": map[string]any{}})
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "response", "id": env.ID, "ok": true, "payload": map[string]any{"restored": true}})
			case "shutdown":
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "response", "id": env.ID, "ok": true, "payload": map[string]any{}})
				return
			default:
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "response", "id": env.ID, "ok": false, "error": "unknown method"})
			}
		case "host-event":
			if env.Event == "fan-data" || env.Event == "device-disconnected" {
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "event", "event": "saw-" + env.Event, "payload": env.Payload})
			}
		}
	}
}

func makeHelperCommandFactory(mode, pluginID string) func(context.Context, Manifest, string) *exec.Cmd {
	return func(_ context.Context, _ Manifest, dir string) *exec.Cmd {
		cmd := exec.Command(os.Args[0])
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			"FC_TEST_SIDECAR_MODE="+mode,
			"FC_TEST_PLUGIN_ID="+pluginID,
		)
		return cmd
	}
}

func TestManifestV2SettingsRuntimeUISlotAndDevDiscovery(t *testing.T) {
	pluginsDir := t.TempDir()
	devDir := filepath.Join(t.TempDir(), "dev-plug")
	if err := os.MkdirAll(filepath.Join(pluginsDir, ".staging", "ignored-stage"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(devDir, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, "ui", "index.js"), []byte("export default {};"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, "plugin.json"), []byte(`{
		"id": "dev-plug",
		"name": "Dev Plugin",
		"version": "0.1.0",
		"platform": "windows-amd64",
		"protocolVersion": 1,
		"runtime": "frontend-only",
		"uiSlot": "sidebar-page",
		"frontend": "ui/index.js",
		"icon": "cpu",
		"settings": [
			{"key": "pollMs", "title": "Poll", "type": "number", "default": 500}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	list, manifests := DiscoverExternalWithDev(pluginsDir, []string{devDir}, "windows-amd64", "3.0.0")
	if len(list) != 1 {
		t.Fatalf("discovered = %d, want 1 (staging dir must be ignored)", len(list))
	}
	if !list[0].DevMode || list[0].Runtime != RuntimeFrontendOnly || list[0].UISlot != UISlotSidebarPage {
		t.Fatalf("unexpected dev plugin info: %+v", list[0])
	}
	if len(manifests["dev-plug"].Settings) != 1 || manifests["dev-plug"].Settings[0].Key != "pollMs" {
		t.Fatalf("unexpected settings schema: %+v", manifests["dev-plug"].Settings)
	}

	asset, err := ReadPluginAsset(pluginsDir, []string{devDir}, "dev-plug", "ui/index.js")
	if err != nil || string(asset.Data) != "export default {};" {
		t.Fatalf("ReadPluginAsset failed: %v, data=%q", err, string(asset.Data))
	}
	if _, err := ReadPluginAsset(pluginsDir, []string{devDir}, "dev-plug", "../plugin.json"); err == nil {
		t.Fatal("expected traversal rejection")
	}
	if _, err := ReadPluginAsset(pluginsDir, []string{devDir}, "dev-plug", "plugin.exe"); err == nil {
		t.Fatal("expected disallowed extension rejection")
	}
}

func TestPluginSettingsStorageAndStagingOps(t *testing.T) {
	cfgDir := t.TempDir()
	pluginsDir := t.TempDir()

	schema := []SettingField{
		{Key: "enabledLog", Title: "Log", Type: "boolean", Default: true},
		{Key: "interval", Title: "Interval", Type: "string", Default: "1s"},
	}
	loaded, err := LoadPluginSettings(cfgDir, "sample-ext", schema)
	if err != nil {
		t.Fatalf("LoadPluginSettings defaults failed: %v", err)
	}
	if loaded["enabledLog"] != true || loaded["interval"] != "1s" {
		t.Fatalf("unexpected defaults: %#v", loaded)
	}
	loaded["interval"] = "500ms"
	if err := SavePluginSettings(cfgDir, "sample-ext", loaded); err != nil {
		t.Fatalf("SavePluginSettings failed: %v", err)
	}
	reloaded, err := LoadPluginSettings(cfgDir, "sample-ext", schema)
	if err != nil || reloaded["interval"] != "500ms" {
		t.Fatalf("reloaded = %#v, err=%v", reloaded, err)
	}

	// Stage update for sample-ext and pending-delete for old-ext.
	srcNew := filepath.Join(t.TempDir(), "new-src")
	if err := os.MkdirAll(srcNew, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcNew, "marker.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTarget := filepath.Join(pluginsDir, "sample-ext")
	if err := os.MkdirAll(oldTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldTarget, "marker.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := StagePluginDir(pluginsDir, "sample-ext", srcNew); err != nil {
		t.Fatalf("StagePluginDir failed: %v", err)
	}

	delTarget := filepath.Join(pluginsDir, "old-ext")
	if err := os.MkdirAll(delTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SavePluginSettings(cfgDir, "old-ext", map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	if err := MarkPluginPendingDelete(pluginsDir, "old-ext", true); err != nil {
		t.Fatalf("MarkPluginPendingDelete failed: %v", err)
	}
	if !HasPendingStaging(pluginsDir, "sample-ext") || !HasPendingStaging(pluginsDir, "old-ext") {
		t.Fatal("expected pending staging checks to return true")
	}

	if err := ApplyPendingStagedOps(pluginsDir, cfgDir, nil); err != nil {
		t.Fatalf("ApplyPendingStagedOps failed: %v", err)
	}
	updatedMarker, err := os.ReadFile(filepath.Join(pluginsDir, "sample-ext", "marker.txt"))
	if err != nil || string(updatedMarker) != "v2" {
		t.Fatalf("updated marker = %q, err=%v", string(updatedMarker), err)
	}
	if _, err := os.Stat(delTarget); !os.IsNotExist(err) {
		t.Fatalf("expected old-ext dir deleted, err=%v", err)
	}
	oldDataDir, _ := PluginDataDir(cfgDir, "old-ext")
	if _, err := os.Stat(oldDataDir); !os.IsNotExist(err) {
		t.Fatalf("expected old-ext data dir deleted, err=%v", err)
	}
}

func TestSidecarRunnerLifecycleInvokeSuspendAndBackoff(t *testing.T) {
	dir := t.TempDir()
	manifest := Manifest{
		ID:              "test-sidecar",
		Name:            "Test Sidecar",
		Version:         "1.0.0",
		Platform:        CurrentPlatform(),
		ProtocolVersion: 1,
		Runtime:         RuntimeSidecar,
		Capabilities:    []string{"status", "fan-telemetry"},
	}

	var mu sync.Mutex
	var events []string
	readyCh := make(chan struct{}, 4)

	runner := NewSidecarRunner(SidecarOptions{
		Manifest:          manifest,
		PluginDir:         dir,
		DataDir:           t.TempDir(),
		CoreVersion:       "3.0.0",
		HandshakeTimeout:  2 * time.Second,
		InvokeTimeout:     150 * time.Millisecond,
		HeartbeatInterval: -1,
		BackoffDelays:     []time.Duration{40 * time.Millisecond, 60 * time.Millisecond},
		CommandFactory:    makeHelperCommandFactory("normal", "test-sidecar"),
		OnEvent: func(pluginID, event string, payload json.RawMessage) {
			mu.Lock()
			events = append(events, pluginID+":"+event)
			mu.Unlock()
		},
	})
	runner.opts.OnStateChange = func() {
		st, running, _, _, _, _ := runner.StateSnapshot()
		if st == StateReady && running {
			select {
			case readyCh <- struct{}{}:
			default:
			}
		}
	}
	t.Cleanup(func() {
		_ = runner.Stop()
	})

	if err := runner.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	select {
	case <-readyCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for sidecar ready")
	}

	res, err := runner.Invoke(context.Background(), "echo", json.RawMessage(`{"x":42}`))
	if err != nil || !strings.Contains(string(res), "42") {
		t.Fatalf("Invoke echo = %s, err=%v", string(res), err)
	}

	// Timeout must NOT replay.
	if _, err := runner.Invoke(context.Background(), "slow", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected Invoke slow to time out")
	}

	if err := runner.SendHostEvent("fan-data", map[string]any{"rpm": 2100}); err != nil {
		t.Fatalf("SendHostEvent failed: %v", err)
	}
	if _, err := runner.Invoke(context.Background(), "emit", json.RawMessage(`{"n":1}`)); err != nil {
		t.Fatalf("Invoke emit failed: %v", err)
	}

	// Suspend and Resume.
	if err := runner.Suspend(1 * time.Second); err != nil {
		t.Fatalf("Suspend failed: %v", err)
	}
	if _, err := runner.Invoke(context.Background(), "echo", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected Invoke during suspend to fail")
	}
	st, _, _, _, _, _ := runner.StateSnapshot()
	if st != StateSuspended {
		t.Fatalf("state after Suspend = %s, want suspended", st)
	}

	runner.Resume()
	select {
	case <-readyCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for sidecar resume ready")
	}
	if err := runner.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	mu.Lock()
	joined := strings.Join(events, ",")
	mu.Unlock()
	if !strings.Contains(joined, "test-sidecar:telemetry-ack") || !strings.Contains(joined, "test-sidecar:prepared-suspend") {
		t.Fatalf("unexpected events captured: %s", joined)
	}
}

func TestSidecarRunnerCrashBackoffStopsAtFailed(t *testing.T) {
	runner := NewSidecarRunner(SidecarOptions{
		Manifest: Manifest{
			ID:              "crash-sidecar",
			Name:            "Crash Sidecar",
			Version:         "1.0.0",
			Platform:        CurrentPlatform(),
			ProtocolVersion: 1,
			Runtime:         RuntimeSidecar,
		},
		PluginDir:         t.TempDir(),
		HandshakeTimeout:  2 * time.Second,
		HeartbeatInterval: -1,
		BackoffDelays:     []time.Duration{20 * time.Millisecond, 20 * time.Millisecond},
		CommandFactory:    makeHelperCommandFactory("crash-immediately", "crash-sidecar"),
	})

	failedCh := make(chan struct{}, 1)
	runner.opts.OnStateChange = func() {
		st, _, _, _, _, _ := runner.StateSnapshot()
		if st == StateFailed {
			select {
			case failedCh <- struct{}{}:
			default:
			}
		}
	}

	if err := runner.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-failedCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for StateFailed, state=%+v", runner.state)
	}
	if len(runner.Logs()) == 0 {
		t.Fatal("expected stderr crash log to be captured")
	}
	_ = runner.Stop()
}
