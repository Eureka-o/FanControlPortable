# Noise Diagnostic Design

**Status:** Draft for user review

**Goal:** Add a THRM-style microphone noise diagnostic to FanControl that measures whether fan-speed increases produce a noticeable relative noise increase, while keeping axis-noise avoidance and long-term control separate.

## Scope

The first version includes a guided microphone test, device-aware speed limits, robust sampling, confidence reporting, and reliable restoration of the pre-test control state.

The first version does not diagnose bearing or axis noise, record raw audio, claim calibrated dBA values, automatically enable axis-noise avoidance, or feed the result into SmartControl learning.

## User Experience

The entry point is the existing curve page learning area. The dialog follows THRM's four phases:

1. Introduction: explain relative-noise measurement, microphone permission, temporary fan takeover, and safety limits.
2. Setup: show the connected device, speed unit, default test range, microphone selector, live level, environment baseline, and connection state.
3. Running: show target speed, actual speed, settling/sampling state, progress, valid sample count, and a cancel action.
4. Result: show the relative noise curve, robust low-to-high increase, noise-growth knee, confidence, invalid-point reasons, and restore status.

The range is editable before the test starts. Editing marks the range as changed; clicking Start opens one confirmation dialog rather than prompting on every keystroke. The confirmation requires acknowledgement of these facts: the test temporarily controls fan speed, audible noise and temperature may change, the microphone must remain in place, the test is relative rather than calibrated dBA, and disconnect/cancel paths attempt to restore the previous state.

The highest selectable speed is the current device capability. The user may lower it, but cannot exceed it. The lowest selectable speed is the device-aware diagnostic floor. The UI displays the source of both limits.

## Device-Aware Range Rules

- FlyDigi BLE/HID devices start at `1000 RPM`.
- Percent-speed devices start at `5%`.
- Other RPM devices use the declared profile minimum; if no reliable minimum exists, the diagnostic is unavailable rather than sending zero or an arbitrary fixed RPM.
- The upper bound comes from the runtime capability when available, including FlyDigi's current maximum gear; otherwise it uses the normalized device profile maximum.
- The final range is normalized with the existing speed-unit and profile helpers. The diagnostic must not introduce a second device-identification path.
- A requested point is accepted only after the Core reports a stable actual speed. If hardware clamps the requested value, the actual stable value is stored with the sample.

The diagnostic floor is a test boundary, not a global change to fan-control limits or saved curve points.

## Measurement

The frontend uses the Web Audio API already available in WebView2. It requests microphone access with echo cancellation, noise suppression, and automatic gain control disabled, then reports a warning if the browser/driver does not apply those settings. No raw audio leaves the process or is persisted.

Before sweeping, the meter collects a 3-5 second baseline at the starting speed. A baseline is stable only after enough valid frames have arrived and its robust spread is within the configured tolerance. An unstable baseline blocks Start.

The sweep uses the device range and a small coarse grid. Each point performs:

1. Set the target through the Core test session.
2. Wait for stable actual speed with a timeout.
3. Wait for airflow to settle.
4. Collect audio frames for the sample window.
5. Remove transient frames and compute a median level plus MAD/IQR spread.
6. Retry one unstable point; if it remains unstable, mark the point invalid.

After the sweep, the starting speed is measured again. A large baseline drift marks the result low-confidence. The analysis compares robust aggregates of the lowest and highest valid points instead of letting one outlier define the total increase.

The result is relative A-weighted noise in the same session, not an absolute sound-pressure measurement. User-facing classification is based on relative increase: not noticeable, slight, clear, or significant. A local peak is labelled only as a suspected noise peak; it is never labelled as axis noise.

## Control Session

The Core owns an exclusive noise-test session because the device manager and BLE/HID/WiFi arbitration must remain authoritative. The session snapshots automatic control, custom speed, manual gear/level, active profile context, and the connected device identity.

While active, the session rejects competing speed writes, pauses automatic target updates, and emits progress/state events to the GUI. Cancellation, timeout, connection loss, sleep, GUI close, and Core shutdown all end the session. Restoration runs in a deferred cleanup path and is guarded by the session identity so it cannot overwrite a newer user action after the test ended.

## Persistence And Separation

Only the latest compact diagnostic summary is stored per device: device identity, speed unit, points, baseline, confidence, microphone label, and timestamp. Raw audio is never stored.

Axis-noise avoidance remains a separate persistent control setting. The diagnostic result does not automatically write or enable that setting. A later feature may offer a user-confirmed suggestion, but the first version must not infer a mechanical source from microphone data.

## Error Handling

- No connected device: disable Start and explain that a connected controllable device is required.
- Unknown speed range or unsupported set-speed capability: disable the diagnostic.
- Microphone denied/unavailable: show a localized permission/device error and keep the dialog usable for retry.
- Actual speed never stabilizes: mark the point unavailable or abort with a clear reason; do not use the requested target as a fake actual value.
- Repeated audio instability: keep the point invalid and lower confidence.
- Device disconnect: stop sampling, restore the snapshot, and report that the result is incomplete.
- Restore failure: keep the result marked incomplete and surface the restore error prominently.

## Validation

Backend tests cover device-floor selection, runtime maximum selection, unit conversion, range validation, exclusive-session behavior, cancellation, disconnect cleanup, and restoration without overwriting a newer config revision.

Frontend tests cover default FlyDigi/percent ranges, editable bounds, confirmation-on-dirty-range behavior, microphone setup errors, stable/unstable baseline gating, robust sample classification, low-confidence result rendering, and the separation between diagnostic output and axis-noise avoidance.

The release check remains the existing project sequence: `go test ./...`, `go vet ./...`, frontend TypeScript check, frontend production build, `git diff --check`, and the normal Windows package build. No new audio dependency is added.

## Acceptance Criteria

- A FlyDigi device opens with a `1000 RPM` lower test bound and its current capability as the upper bound.
- A percent device opens with a `5%` lower test bound and its profile maximum as the upper bound.
- Users can narrow the range but cannot exceed device limits.
- A changed range always requires one explicit confirmation before control starts.
- No test writes below the selected diagnostic floor or above the runtime maximum.
- Test cancellation and disconnect restore the previous control state.
- Results clearly describe relative noise increase and confidence, never claim axis-noise detection, and do not enable axis-noise avoidance automatically.
