# Noise Diagnostic Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a THRM-style, device-aware microphone noise diagnostic with editable bounds, explicit confirmation, reliable Core-owned control sessions, and compact per-device results.

**Architecture:** The Core owns an exclusive diagnostic lease and all fan writes; the frontend owns Web Audio capture, robust sampling, analysis, and the guided dialog. Existing device-profile speed-unit/range helpers, IPC routing, device events, and curve-page controls are reused. Axis-noise avoidance remains on its existing path and is never changed by this feature.

**Tech Stack:** Go, Wails v2 IPC, existing device manager, React/TypeScript, Web Audio API, Recharts, existing UI primitives, i18next.

---

## File Map

- Create `internal/types/noise_diagnostic.go`: wire types, range selection, and result normalization.
- Modify `internal/types/types.go`: add the compact per-device result map to `AppConfig` and preserve zero-value compatibility.
- Create `internal/types/noise_diagnostic_test.go`: range, unit, capability, and result normalization tests.
- Create `internal/coreapp/noise_diagnostic.go`: exclusive lease, target writes, state restoration, and result persistence.
- Modify `internal/coreapp/app.go`: add the lease state and shutdown-safe cleanup hook.
- Modify `internal/coreapp/monitoring.go`: skip automatic target writes while the lease is active.
- Modify `internal/coreapp/system_device.go` and `internal/coreapp/lifecycle.go`: cancel the lease on disconnect, sleep, and Core shutdown.
- Create `internal/coreapp/noise_diagnostic_test.go`: lease ownership, bounds, cancellation, disconnect, and restore tests.
- Modify `internal/ipc/ipc.go`: request types, parameter structs, and diagnostic result payloads.
- Modify `internal/coreapp/ipc.go` and create `internal/coreapp/ipc_noise_diagnostic.go`: route and handle the new requests.
- Create `internal/guiapp/noise_diagnostic_api.go`: Wails methods with bounded IPC timeouts and error translation.
- Modify `frontend/src/app/services/api.ts`: diagnostic methods and device-state accessors.
- Create `frontend/src/app/lib/noise-diagnostic.ts`: Web Audio meter, baseline sampling, robust statistics, sweep analysis, and range helpers.
- Create `frontend/src/app/lib/noise-diagnostic.test.ts`: pure frontend algorithm tests.
- Create `frontend/src/app/components/NoiseDiagnostic.tsx`: THRM-style four-phase dialog, editable range, confirmation, progress, and result view.
- Modify `frontend/src/app/components/FanCurve.tsx`: mount the dialog and add the existing learning-area entry point.
- Modify `frontend/src/app/locales/en-US/translation.json`, `frontend/src/app/locales/ja-JP/translation.json`, and `frontend/src/app/locales/zh-CN/translation.json`: all user-facing strings.
- Create `frontend/tests/noise-diagnostic-ui.test.mjs`: source-level UI contract checks for bounds, confirmation, and separation from axis-noise avoidance.

## Task 1: Add Device-Aware Diagnostic Types

**Files:**
- Create: `internal/types/noise_diagnostic.go`
- Modify: `internal/types/types.go`
- Test: `internal/types/noise_diagnostic_test.go`

- [ ] Define the wire types with JSON tags:

```go
type NoiseDiagnosticRange struct {
    Unit       string `json:"unit"`
    Min        int    `json:"min"`
    Max        int    `json:"max"`
    Step       int    `json:"step"`
    MinSource  string `json:"minSource"`
    MaxSource  string `json:"maxSource"`
}

type NoiseDiagnosticPoint struct {
    Requested int     `json:"requested"`
    Actual    int     `json:"actual"`
    LevelDB   float64 `json:"levelDb"`
    SpreadDB  float64 `json:"spreadDb"`
    Valid     bool    `json:"valid"`
}

type NoiseDiagnosticResult struct {
    DeviceKey        string                 `json:"deviceKey"`
    Unit             string                 `json:"unit"`
    Points           []NoiseDiagnosticPoint `json:"points"`
    BaselineDB       float64                `json:"baselineDb"`
    BaselineDriftDB  float64                `json:"baselineDriftDb"`
    RiseDB           float64                `json:"riseDb"`
    Knee             int                    `json:"knee"`
    Confidence       string                 `json:"confidence"`
    ConfidenceReason string                 `json:"confidenceReason"`
    Microphone       string                 `json:"microphone"`
    TestedAt         int64                  `json:"testedAt"`
}

type NoiseDiagnosticBeginRequest struct {
    DeviceKey string             `json:"deviceKey"`
    Range     NoiseDiagnosticRange `json:"range"`
}

type NoiseDiagnosticSession struct {
    SessionID string             `json:"sessionId"`
    DeviceKey string             `json:"deviceKey"`
    Range     NoiseDiagnosticRange `json:"range"`
    ConfigRevision uint64        `json:"configRevision"`
}

type NoiseDiagnosticTargetResult struct {
    Requested int `json:"requested"`
    Actual    int `json:"actual"`
    Unit      string `json:"unit"`
}
```

- [ ] Implement `NoiseDiagnosticRangeForProfile(profile, capabilities, fanData)` by reusing `NormalizeFanSpeedUnit`, `DeviceProfile.SpeedRange`, and the existing FlyDigi runtime maximum. Select the lower bound as `1000` for FlyDigi BLE/HID, `5` for percent devices, and the declared profile minimum for other RPM profiles. Reject an unknown RPM minimum instead of returning zero.
- [ ] Normalize the result by dropping invalid points, sorting by actual speed, and clamping stored values to finite ranges. Add `NoiseDiagnosticsByDevice map[string]NoiseDiagnosticResult` to `AppConfig` without changing existing fields or old config behavior.
- [ ] Test FlyDigi BLE, FlyDigi HID, percent WiFi, declared-minimum RPM, unknown-minimum RPM, runtime maximum below profile maximum, editable range clamping, and invalid result cleanup.
- [ ] Run `go test ./internal/types/... -count=1` and expect PASS.
- [ ] Commit only these type files and tests: `feat: add noise diagnostic device range types`.

## Task 2: Implement the Core-Owned Diagnostic Lease

**Files:**
- Create: `internal/coreapp/noise_diagnostic.go`
- Modify: `internal/coreapp/app.go`
- Modify: `internal/coreapp/monitoring.go`
- Modify: `internal/coreapp/system_device.go`
- Modify: `internal/coreapp/lifecycle.go`
- Test: `internal/coreapp/noise_diagnostic_test.go`

- [ ] Add a private lease containing a random session ID, device key, normalized range, config revision at start, snapshot metadata, and an expiry timestamp. Keep it behind a dedicated mutex; never hold the Core mutex while waiting on device I/O.
- [ ] Implement these Core methods:

```go
func (a *CoreApp) BeginNoiseDiagnostic(request types.NoiseDiagnosticBeginRequest) (types.NoiseDiagnosticSession, error)
func (a *CoreApp) SetNoiseDiagnosticTarget(sessionID string, value int) (types.NoiseDiagnosticTargetResult, error)
func (a *CoreApp) EndNoiseDiagnostic(sessionID string) error
func (a *CoreApp) CancelNoiseDiagnostic(sessionID string) error
func (a *CoreApp) SaveNoiseDiagnosticResult(result types.NoiseDiagnosticResult) error
```

- [ ] `BeginNoiseDiagnostic` must require a connected device with set-speed support, derive the device key and capability range, validate the requested editable range, mark the lease active, and return the normalized session/range. It must not mutate saved `AutoControl`, `CustomSpeedEnabled`, manual gear, or curve values.
- [ ] `SetNoiseDiagnosticTarget` must verify the session ID, current device identity, unit/range bounds, lease expiry, and connection state before calling the existing device manager target-speed path. Return the requested and latest actual values; do not persist the temporary target.
- [ ] Add a single monitoring guard that skips automatic target writes while a valid lease is active, without stopping temperature or fan telemetry.
- [ ] End/cancel cleanup must release the guard, re-read the latest config revision, and apply the latest user state rather than overwriting changes made during the test. If the revision is unchanged, restore the original mode exactly. Make cleanup idempotent.
- [ ] Hook cancellation into device-disconnect, sleep, Core shutdown, and the lease expiry watchdog. Emit the existing device/config events only when the restored state actually changes.
- [ ] Persist only a normalized compact result under `NoiseDiagnosticsByDevice`; reject a result for a different active device or unit.
- [ ] Test one active lease, competing lease rejection, below-floor/above-capability rejection, disconnect cleanup, cancellation idempotence, expiry cleanup, unchanged-revision restoration, changed-revision preservation, and no monitoring write during the lease.
- [ ] Run `go test ./internal/coreapp/... -count=1` and `go test -race ./internal/coreapp/... -run NoiseDiagnostic -count=1` and expect PASS.
- [ ] Commit only the Core lease files and tests: `feat: add core noise diagnostic lease`.

## Task 3: Wire IPC and Wails APIs

**Files:**
- Modify: `internal/ipc/ipc.go`
- Modify: `internal/coreapp/ipc.go`
- Create: `internal/coreapp/ipc_noise_diagnostic.go`
- Create: `internal/guiapp/noise_diagnostic_api.go`
- Modify: `frontend/src/app/services/api.ts`
- Test: `internal/ipc/ipc_test.go`, `internal/guiapp/ipc_client_test.go`

- [ ] Add request types `BeginNoiseDiagnostic`, `SetNoiseDiagnosticTarget`, `EndNoiseDiagnostic`, `CancelNoiseDiagnostic`, and `SaveNoiseDiagnosticResult`, plus explicit parameter structs carrying session ID, value, range, and result.
- [ ] Add a dedicated IPC route before generic control fallback. Decode malformed payloads into normal error responses; never panic on a missing session ID.
- [ ] Add Wails wrappers that use `sendRequestWithTimeout`: 3 seconds for begin/end/cancel/save and 5 seconds for target writes. Do not reuse the 10-second generic write retry because a lost response must not repeat a fan-speed command.
- [ ] Add `apiService` methods that call the Wails runtime fallback style already used for newer APIs. Return typed session, target, and result payloads.
- [ ] Test request serialization, malformed input, missing-session errors, and the no-replay behavior after an IPC response timeout.
- [ ] Run `go test ./internal/ipc ./internal/guiapp ./internal/coreapp -count=1` and expect PASS.
- [ ] Commit only IPC/API files and tests: `feat: expose noise diagnostic session api`.

## Task 4: Implement Frontend Measurement Utilities

**Files:**
- Create: `frontend/src/app/lib/noise-diagnostic.ts`
- Test: `frontend/src/app/lib/noise-diagnostic.test.ts`

- [ ] Reuse Web Audio API and existing Recharts data shapes; do not add an audio or FFT dependency. `NoiseMeter` must request `echoCancellation: false`, `noiseSuppression: false`, and `autoGainControl: false`, keep the selected `MediaStreamTrack`, and close every track/context in `close()`.
- [ ] Implement baseline gating with a minimum valid-frame count and robust spread. Expose the reason when the baseline is unstable.
- [ ] Implement `sampleLevel()` returning median level, robust spread, valid-frame ratio, and retry eligibility. Discard transient frames before calculating the result.
- [ ] Implement `buildDiagnosticSteps(range)` with a small coarse grid and an optional local refinement around the largest slope change. Preserve the device unit.
- [ ] Implement `analyzeNoiseDiagnostic(points, initialBaseline, finalBaseline)` returning robust low/high rise, knee, confidence, and a reason. A local peak must be named `suspectedPeak`, never `axisNoise`.
- [ ] Test A-weighting bounds, no-microphone cleanup, stable/unstable baselines, transient rejection, invalid point handling, low/high robust rise, baseline drift, and confidence classification.
- [ ] Run the focused frontend test command used by existing `frontend/tests` and `cd frontend; npx tsc --noEmit` after the utility is wired.
- [ ] Commit only utility files and tests: `feat: add noise diagnostic measurement utilities`.

## Task 5: Build the THRM-Style Frontend Dialog

**Files:**
- Create: `frontend/src/app/components/NoiseDiagnostic.tsx`
- Modify: `frontend/src/app/components/FanCurve.tsx`
- Modify: `frontend/src/app/locales/en-US/translation.json`
- Modify: `frontend/src/app/locales/ja-JP/translation.json`
- Modify: `frontend/src/app/locales/zh-CN/translation.json`
- Test: `frontend/tests/noise-diagnostic-ui.test.mjs`

- [ ] Add the learning-area entry point beside the existing learning/noise controls in `FanCurve.tsx`, passing `config`, `isConnected`, `fanData`, `runtimeDeviceProfile`, `runtimeDeviceCapabilities`, and `onConfigChange` through the existing component pattern.
- [ ] Implement the four dialog phases from the design. Reuse `Dialog`, `Button`, `Badge`, `Select`, `Input`, `Slider`, `DialogFooter`, existing chart theme variables, and existing toast patterns.
- [ ] Setup must render the backend range source, allow only min/max edits within bounds, show the current device identity, and mark a dirty range. Start stays disabled for no connection, unsupported speed control, invalid range, unstable baseline, or an active restore.
- [ ] When a dirty range is submitted, show one confirmation dialog with the agreed disclaimer and an explicit acknowledgement control. Do not show repeated prompts while typing.
- [ ] Running must call the Core session methods, wait for actual speed through existing fan-data updates, show requested/actual values, collect microphone samples, and cancel on device disconnect or unmount.
- [ ] Result must render the relative curve, rise classification, knee, confidence, invalid-point reasons, and restore status. It must not expose an “axis noise detected” label or automatically toggle the existing avoidance configuration.
- [ ] Save only the compact result after a successful restore; refresh config through the existing config-update event path.
- [ ] Add complete localized strings for setup, disclaimer, errors, progress, result, confidence, and restore states.
- [ ] Add UI source-contract tests checking the two device floors, max-bound enforcement, dirty-range confirmation, disconnect cancellation, and absence of automatic axis-avoidance writes.
- [ ] Run `cd frontend; npx tsc --noEmit`, the focused frontend tests, and `npm run build`; expect PASS.
- [ ] Commit only frontend component/locale/tests: `feat: add noise diagnostic dialog`.

## Task 6: Cross-Layer Verification

**Files:**
- No new product files; review all files from Tasks 1-5.

- [ ] Run `go test ./... -count=1`.
- [ ] Run `go vet ./...`.
- [ ] Run `cd frontend; npx tsc --noEmit`.
- [ ] Run `cd frontend; npm run build`.
- [ ] Run `git diff --check` and `codegraph sync` followed by `codegraph status`.
- [ ] Inspect the generated Wails bindings only if the build regenerates them; keep unrelated generated changes out of the commit.
- [ ] Build the normal Windows package with `cmd /c build.bat` only after all tests pass, then inspect the installer/portable contents for absence of raw audio files and unexpected dependencies.
- [ ] Commit the verified integration only if the preceding feature commits are clean: `test: verify noise diagnostic integration`.

## Out Of Scope During This Plan

- Do not modify the existing axis-noise avoidance algorithm or automatically write its configuration.
- Do not integrate the result into SmartControl learning.
- Do not add absolute dBA calibration, raw recording, native audio libraries, or a new device arbitration subsystem.
- Do not change global fan-curve minimums merely to support diagnostic bounds.
