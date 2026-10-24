# FanControl Architecture Deepening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify configuration commits, isolate WiFi/serial compatibility runtime, centralize device snapshots, and extract frontend state transitions without changing native BLE/HID behavior.

**Architecture:** Keep existing public interfaces and configuration schemas stable. Deepen existing modules behind private seams, migrate callers incrementally, and protect every behavior change with a failing test before implementation.

**Tech Stack:** Go, TypeScript, React, Zustand, Node test runner, CodeGraph.

---

### Task 1: Baseline and configuration commit transaction

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/coreapp/config_control.go`
- Modify: config mutation call sites under `internal/coreapp/`
- Test: `internal/config/config_test.go`
- Test: `internal/coreapp/config_control_test.go`

- [ ] Run `go test ./internal/config ./internal/coreapp` and record the baseline.
- [ ] Add a failing test proving a failed persisted mutation leaves config and revision unchanged.
- [ ] Add the minimum atomic mutation-and-save implementation.
- [ ] Add a failing test proving post-commit effects and config broadcast run only after persistence succeeds.
- [ ] Migrate persisted `Set+Save` callers without changing runtime-only revision updates.
- [ ] Run `go test ./internal/config ./internal/coreapp`.

### Task 2: Compatibility runtime isolation

**Files:**
- Create: `internal/device/compatibility_runtime.go`
- Modify: `internal/device/manager_wifi.go`
- Modify: `internal/device/wifi.go`
- Modify: `internal/device/serial.go`
- Test: `internal/device/compatibility_runtime_test.go`

- [ ] Add failing routing tests for WiFi and serial compatibility operations.
- [ ] Add a private compatibility runtime that delegates only WiFi/serial connect, disconnect, refresh, and speed operations.
- [ ] Preserve native paths in `ble_profile.go`, `auto_native.go`, and FlyDigi HID files byte-for-byte.
- [ ] Run `go test ./internal/device ./internal/deviceprofileexec`.

### Task 3: Authoritative backend device snapshot

**Files:**
- Modify: `internal/coreapp/device_runtime_state.go`
- Modify: `internal/coreapp/device_connection_flow.go`
- Modify: `internal/coreapp/system_device.go`
- Modify: `internal/coreapp/health_monitor.go`
- Test: `internal/coreapp/device_runtime_state_test.go`

- [ ] Add failing transition-sequence tests for connecting, ready, disconnected, suspended, and reconnecting states.
- [ ] Centralize snapshot derivation and route CoreApp state publication through the existing connection flow.
- [ ] Keep device Manager connectivity as the hardware adapter truth.
- [ ] Run `go test ./internal/coreapp ./internal/device`.

### Task 4: Frontend device snapshot reducer

**Files:**
- Create: `frontend/src/app/store/device-snapshot.ts`
- Modify: `frontend/src/app/store/app-store.ts`
- Test: `frontend/tests/device-snapshot.test.mjs`

- [ ] Add failing tests for initialization, connection, disconnection, and core-error snapshots.
- [ ] Implement a pure reducer for correlated device fields.
- [ ] Route initialization, refresh, and IPC events through the reducer while leaving toasts and timeline events outside it.
- [ ] Run the focused Node tests and `npx tsc --noEmit`.

### Task 5: Curve editor session

**Files:**
- Create: `frontend/src/app/lib/curve-editor-session.ts`
- Modify: `frontend/src/app/components/FanCurve.tsx`
- Modify: `frontend/tests/fan-curve-profile-ui.test.mjs`
- Test: `frontend/tests/curve-editor-session.test.mjs`

- [ ] Add failing behavior tests for save, discard, pending profile switch, pending tab switch, and save failure.
- [ ] Implement the smallest pure session transition module.
- [ ] Replace only the matching local state transitions in `FanCurve.tsx`; leave chart rendering and calculations unchanged.
- [ ] Run focused Node tests, `npx tsc --noEmit`, and `npm run build`.

### Task 6: Final verification

- [ ] Run `go test ./...`.
- [ ] Run frontend Node tests, `npx tsc --noEmit`, and `npm run build`.
- [ ] Run `git diff --check`.
- [ ] Confirm native BLE/HID implementation files are unchanged.
- [ ] Run `codegraph sync` and `codegraph status`.
