# 2.5.0 Device Safety Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose a compact, cross-transport device runtime state and prevent automatic fan writes from acting on invalid telemetry or an unready device.

**Architecture:** CoreApp derives persistent readiness from the existing device manager, suspend flag, capability profile, and refreshed device settings. A minimal transient connection phase distinguishes discovery from connecting. The monitor loop uses the derived readiness plus fresh valid temperature before issuing its existing target write.

**Tech Stack:** Go, Wails IPC maps, React, TypeScript, i18next.

---

### Task 1: Runtime state contract

**Files:**
- Create: `internal/coreapp/device_runtime_state.go`
- Create: `internal/coreapp/device_runtime_state_test.go`
- Modify: `internal/coreapp/app.go`
- Modify: `internal/coreapp/system_device.go`
- Modify: `internal/coreapp/device_candidates.go`
- Modify: `internal/coreapp/device_connection_flow.go`

- [ ] **Step 1: Write failing state-resolution tests**

```go
func TestDeviceRuntimeStatusPrefersSuspendAndConnectionPhases(t *testing.T) {
    app := newDeviceProfileTestApp(t, types.GetDefaultConfig(false))
    app.connectionPhase.Store(int32(deviceConnectionPhaseDiscovering))
    if got := app.deviceRuntimeStatus().State; got != deviceRuntimeStateDiscovering {
        t.Fatalf("state = %q", got)
    }
    app.systemSuspended.Store(true)
    if got := app.deviceRuntimeStatus().State; got != deviceRuntimeStateUnavailable {
        t.Fatalf("state = %q", got)
    }
}
```

- [ ] **Step 2: Run the focused test and verify it fails because the runtime-status API is missing**

Run: `go test ./internal/coreapp -run TestDeviceRuntimeStatusPrefersSuspendAndConnectionPhases -count=1`

- [ ] **Step 3: Add the smallest derived state implementation**

```go
func (a *CoreApp) deviceRuntimeStatus() deviceRuntimeStatus {
    if a.systemSuspended.Load() { return deviceRuntimeStatus{State: deviceRuntimeStateUnavailable} }
    if a.reconnectInProgress.Load() || a.connectionPhase.Load() == int32(deviceConnectionPhaseDiscovering) { return deviceRuntimeStatus{State: deviceRuntimeStateDiscovering} }
    if a.connectionPhase.Load() == int32(deviceConnectionPhaseConnecting) { return deviceRuntimeStatus{State: deviceRuntimeStateConnecting} }
    // derive disconnected/capabilities/ready/unavailable from the existing manager and settings
}
```

- [ ] **Step 4: Run focused CoreApp tests**

Run: `go test ./internal/coreapp -run 'TestDeviceRuntimeStatus|TestManualDisconnect|TestReconnect' -count=1`

### Task 2: Automatic-control safety gate

**Files:**
- Modify: `internal/coreapp/monitoring.go`
- Modify: `internal/coreapp/device_runtime_state_test.go`

- [ ] **Step 1: Write failing tests for fresh control input and recovery**

```go
func TestAutomaticControlInputReadyRejectsStaleTelemetry(t *testing.T) {
    temp := types.TemperatureData{BridgeOk: true, TelemetryFresh: false, ControlTemp: 65}
    if automaticControlInputReady(temp) { t.Fatal("stale telemetry must not drive a new target") }
}
```

- [ ] **Step 2: Run the focused test and verify it fails because the gate is missing**

Run: `go test ./internal/coreapp -run TestAutomaticControlInputReadyRejectsStaleTelemetry -count=1`

- [ ] **Step 3: Gate the existing monitor write path**

```go
func automaticControlInputReady(temp types.TemperatureData) bool {
    return temp.BridgeOk && temp.TelemetryFresh && validSmartControlTemperature(temp.ControlTemp)
}
```

Use the predicate with `deviceControlReady()` before calculating or sending a new automatic target. Reset transient learning state while paused and set `forceNextAutoTarget` when valid input returns.

- [ ] **Step 4: Run focused monitoring tests**

Run: `go test ./internal/coreapp -run 'TestAutomaticControlInputReady|Test.*SmartControl|Test.*Temperature' -count=1`

### Task 3: Status API and compact UI

**Files:**
- Modify: `internal/coreapp/system_device.go`
- Modify: `frontend/src/app/services/device-service.ts`
- Modify: `frontend/src/app/store/app-store.ts`
- Modify: `frontend/src/app/page.tsx`
- Modify: `frontend/src/app/components/DeviceStatus.tsx`
- Modify: `frontend/src/app/locales/zh-CN/translation.json`
- Modify: `frontend/src/app/locales/en-US/translation.json`
- Modify: `frontend/src/app/locales/ja-JP/translation.json`

- [ ] **Step 1: Write a failing UI contract test**

```js
test('renders one compact device runtime state and keeps the normal connection label', () => {
  assert.match(source, /runtimeState/);
  assert.match(source, /deviceStatus\.runtimeState/);
});
```

- [ ] **Step 2: Run the test and verify it fails because the compact state is absent**

Run: `node --test frontend/tests/device-runtime-state-ui.test.mjs`

- [ ] **Step 3: Pass the runtime object through the existing status payload and add one compact label**

The state label must not resize the hero card, replace the temperature/fan metrics, or add a new panel.

- [ ] **Step 4: Verify frontend contract and types**

Run: `node --test frontend/tests/device-runtime-state-ui.test.mjs`

Run: `cd frontend; npx tsc --noEmit`

### Task 4: Scope documentation and final verification

**Files:**
- Modify: `docs/fancontrol-2.5.0-roadmap.md`

- [ ] **Step 1: Mark the completed status/safety sub-items without marking the deferred configuration-center work complete**

- [ ] **Step 2: Run final scoped verification**

Run: `go test ./internal/coreapp ./internal/device -count=1`

Run: `cd frontend; npx tsc --noEmit`

Run: `git diff --check`
