//go:build windows

package plugins

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func findTestGCC() string {
	candidates := []string{
		`D:\FanControlTools\CodeBlocks\MinGW\bin\gcc.exe`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if path, err := exec.LookPath("gcc"); err == nil {
		return path
	}
	return ""
}

func TestInprocDLLLoadInvokeAndTelemetry(t *testing.T) {
	gcc := findTestGCC()
	if gcc == "" {
		t.Skip("MinGW gcc not available for C ABI DLL integration test")
	}

	tmpDir := t.TempDir()
	cSrc := filepath.Join(tmpDir, "test_plugin.c")
	dllPath := filepath.Join(tmpDir, "test_plugin.dll")

	code := `
#include <stdint.h>
#include <stdio.h>
#include <string.h>

static uint32_t g_last_rpm = 0;
static uint32_t g_last_pct = 0;
static uint32_t g_last_conn = 0;
static int g_inited = 0;

__declspec(dllexport) uint32_t fc_plugin_abi_version(void) {
    return 1u;
}

__declspec(dllexport) int32_t fc_plugin_init(const char* config_json_utf8) {
    if (!config_json_utf8) return -1;
    g_inited = 1;
    return 0;
}

__declspec(dllexport) void fc_plugin_on_fan_data(uint32_t rpm, uint32_t is_percent, uint32_t connected) {
    g_last_rpm = rpm;
    g_last_pct = is_percent;
    g_last_conn = connected;
}

__declspec(dllexport) int32_t fc_plugin_invoke(const char* method_utf8, const char* payload_json_utf8, char* out_buf, uint32_t out_cap) {
    if (!method_utf8 || !out_buf || out_cap < 32) return -1;
    if (strcmp(method_utf8, "get-telemetry") == 0) {
        int n = snprintf(out_buf, out_cap, "{\"inited\":%d,\"rpm\":%u,\"isPercent\":%u,\"connected\":%u}",
                         g_inited, g_last_rpm, g_last_pct, g_last_conn);
        return n;
    }
    return -2;
}

__declspec(dllexport) void fc_plugin_shutdown(void) {
    g_inited = 0;
}
`
	if err := os.WriteFile(cSrc, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(gcc, "-shared", "-O2", cSrc, "-o", dllPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gcc build DLL failed: %v\n%s", err, string(out))
	}

	runner := NewInprocRunner("test-inproc", dllPath)
	t.Cleanup(func() {
		runner.mutex.Lock()
		h := runner.dllHandle
		runner.dllHandle = 0
		runner.mutex.Unlock()
		if h != 0 {
			_ = windows.FreeLibrary(h)
		}
	})
	if err := runner.LoadAndStart([]byte(`{"rate":250}`)); err != nil {
		t.Fatalf("LoadAndStart failed: %v", err)
	}
	runner.OnFanData(2350, false, true)

	res, err := runner.Invoke("get-telemetry", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}
	if !strings.Contains(string(res), `"rpm":2350`) || !strings.Contains(string(res), `"connected":1`) {
		t.Fatalf("unexpected telemetry JSON from DLL: %s", string(res))
	}

	if err := runner.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	st, running, loaded, _ := runner.StateSnapshot()
	if st != StateDisabled || running || !loaded {
		t.Fatalf("unexpected state after Stop: st=%s running=%v loaded=%v", st, running, loaded)
	}
}
