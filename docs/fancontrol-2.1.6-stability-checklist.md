# FanControl 2.1.6 Stability Checklist

Date: 2026-06-10

This checklist records the proposed scope for FanControl 2.1.6 after reviewing the current FanControl codebase and the THRM v3.1.5 reference project. The version goal is stability and intuitive behavior, not a new device-platform feature release.

After context compaction or handoff, read this file again before implementing 2.1.6 work.

## Version Goal

- Keep FanControl 2.1.6 as a focused stability and behavior-polish release.
- Preserve all 2.0/2.1 user data: config, fan curves, learned offsets, themes, WiFi IP, device profiles, autostart, tray behavior, and installer upgrade flow.
- Prefer fixes that reduce user confusion, especially around PawnIO, learning curves, stopped-fan recovery, WiFi scan behavior, and crash diagnosis.
- Keep FlyDigi BLE/HID frontend exposure hidden for this version.
- Do not introduce DIY protocol-template UI or large device-management redesign in this version.
- Do not affect the original THRM/reference project.

## Must Do

### 1. Port THRM v3.1.5 Smart-Control Offset Preservation

- [ ] Add per-curve learned-offset storage equivalent to the reference project's `smartcontrol_profile_offsets.go`.
- [ ] Preserve existing single `SmartControl.LearnedOffsets` as the active curve's current view for backward compatibility.
- [ ] Add or adapt a `LearnedOffsetsByProfile`-style field so each fan-curve profile keeps its own learned offsets.
- [ ] On config load, curve switch, curve save, and learning update:
  - [ ] load offsets for the active curve profile.
  - [ ] store updated offsets back to the active curve profile.
  - [ ] keep old configs without per-profile offsets working.
- [ ] Keep percent devices in 0.1% tick units and RPM devices in 1 RPM units.
- [ ] Do not break imported themes or current fan-curve UI display.

Reference:
- `D:\Desktop\风扇控制便携版\THRM-reference-git\internal\coreapp\smartcontrol_profile_offsets.go`
- `D:\Desktop\风扇控制便携版\THRM-reference-git\internal\coreapp\config_control.go`
- `D:\Desktop\风扇控制便携版\THRM-reference-git\internal\coreapp\curve_profiles.go`
- `D:\Desktop\风扇控制便携版\THRM-reference-git\internal\coreapp\monitoring.go`

Our likely files:
- `internal\types\types.go`
- `internal\coreapp\config_control.go`
- `internal\coreapp\curve_profiles.go`
- `internal\coreapp\lifecycle.go`
- `internal\coreapp\monitoring.go`
- `internal\smartcontrol\percent_control.go`
- `internal\smartcontrol\legacy_rpm.go`

### 2. Port Stopped-Fan Wakeup Send Logic

- [ ] Align auto-control send decisions with THRM v3.1.5 where appropriate.
- [ ] If the target speed is positive and the device reports target speed 0 or current speed 0, force a resend.
- [ ] Keep percent/RPM decoupling:
  - [ ] RPM path compares RPM.
  - [ ] percent path compares 0.1% ticks internally.
  - [ ] percent send still rounds only at the device-send boundary.
- [ ] Keep WiFi device behavior stable and do not change the existing WiFi `/api/speed` packet format.
- [ ] Add regression tests for:
  - [ ] positive target with device target 0.
  - [ ] positive target with current speed 0.
  - [ ] small drift that should not resend.
  - [ ] percent and RPM units separately.

Reference:
- `D:\Desktop\风扇控制便携版\THRM-reference-git\internal\coreapp\monitoring.go`
- `D:\Desktop\风扇控制便携版\THRM-reference-git\internal\coreapp\app_power_test.go`

Our likely files:
- `internal\coreapp\monitoring.go`
- `internal\coreapp\app_power_test.go`
- `internal\smartcontrol\unit_paths_test.go`

### 3. Add Core Fatal Crash Logs

- [ ] Port the THRM v3.1.5 fatal-output capture idea into FanControl Core.
- [ ] Write fatal logs to a user-accessible log location consistent with existing FanControl logging.
- [ ] Capture unexpected panic/fatal output from the Core process.
- [ ] Keep non-Windows stubs compiling.
- [ ] Make the log useful for users reporting startup or background crashes.
- [ ] Avoid changing normal log verbosity.

Reference:
- `D:\Desktop\风扇控制便携版\THRM-reference-git\cmd\core\fatal_log_windows.go`
- `D:\Desktop\风扇控制便携版\THRM-reference-git\cmd\core\fatal_log_other.go`
- `D:\Desktop\风扇控制便携版\THRM-reference-git\cmd\core\main.go`
- `D:\Desktop\风扇控制便携版\THRM-reference-git\internal\coreapp\crash_report.go`

Our likely files:
- `cmd\core\main.go`
- `cmd\core\fatal_log_windows.go`
- `cmd\core\fatal_log_other.go`
- `internal\coreapp\crash_report.go`

### 4. Audit PawnIO Repair UX And Runtime Logic

- [ ] Review current installer strategy before changing it:
  - [ ] FanControl currently skips PawnIO install/update if any existing PawnIO is detected.
  - [ ] THRM v3.1.5 attempts silent update only when the installed PawnIO version is older than the bundled version.
- [ ] Decide whether 2.1.6 should adopt the reference installer strategy or keep the conservative 2.1.2 hotfix behavior.
- [ ] Review the in-app `Reinstall PawnIO` action:
  - [ ] current behavior uninstalls first, then installs.
  - [ ] this may recreate the "driver marked for deletion" style issue.
- [ ] Prefer a safer UX split if implemented:
  - [ ] `修复/更新 PawnIO`: try install/update first, no uninstall.
  - [ ] `彻底重装 PawnIO`: advanced action with warning and reboot guidance.
- [ ] Do not remove the current recovery path unless the replacement is tested.
- [ ] Avoid UAC-heavy validation unless the user explicitly helps with it.

Reference:
- `D:\Desktop\风扇控制便携版\THRM-reference-git\build\windows\installer\project.nsi`
- `D:\Desktop\风扇控制便携版\THRM-reference-git\build\windows\installer\project_strings.nsh`

Our likely files:
- `build\windows\installer\project.nsi`
- `build\windows\installer\project_strings.nsh`
- `internal\coreapp\platform_windows.go`
- frontend settings/diagnostic strings if button text changes

### 5. Guard WiFi Scan Against Overlap

- [ ] Add backend protection so only one manual WiFi scan runs at a time.
- [ ] If a new scan starts while another scan is running, either:
  - [ ] reject it with a clear result, or
  - [ ] cancel the old scan before starting the new one.
- [ ] Pause/resume/cancel should only affect the active scan.
- [ ] Keep normal scan lightweight.
- [ ] Keep deep scan user-triggered and visibly cancelable.
- [ ] Do not implement new scan ranges in this version unless needed for a bug fix.

Likely files:
- `internal\coreapp\wifi_discovery.go`
- `internal\types\wifi_discovery.go`
- `internal\device\wifi_discovery.go`
- `frontend\src\app\components\ControlPanel.tsx`

### 6. Recheck WiFi Smart Start/Stop Beta Recovery

- [ ] Keep WiFi smart start/stop as Beta.
- [ ] Confirm standby-speed send still runs on app quit and system suspend.
- [ ] Confirm it only applies to WiFi percent devices.
- [ ] Add or update tests where practical for:
  - [ ] disabled option does nothing.
  - [ ] non-WiFi or non-percent device does nothing.
  - [ ] failed send clears the internal applied flag so future attempts are not permanently blocked.
- [ ] Do not add full software-simulated smart start/stop state machine in 2.1.6.

Likely files:
- `internal\coreapp\system_device.go`
- `internal\types\types.go`
- settings UI only if a bug is found

### 7. Add Temperature Rise Prediction Beta

- [ ] Add `温升预判` as a conservative Beta smart-control option, default off.
- [ ] Place it near fan-curve smart-control settings, not in device connection or advanced device management.
- [ ] Try to collect CPU/GPU power through the temperature bridge when LibreHardwareMonitor exposes power sensors.
- [ ] If power data is unavailable, fall back to temperature-rise trend only and keep the UI clear about the fallback.
- [ ] Weight power draw and temperature rise rate together, but cap the extra speed boost:
  - [ ] percent devices use 0.1% ticks internally.
  - [ ] RPM devices use real RPM units.
- [ ] Do not allow one isolated temperature jump to trigger prediction.
- [ ] Keep prediction independent from the transient spike filter:
  - [ ] if spike filtering is enabled and suppresses a sample, prediction should not boost.
  - [ ] if spike filtering is disabled, prediction still requires a multi-sample trend.
- [ ] Keep true high-temperature curve response unaffected.

Likely files:
- `bridge\TempBridge\Program.cs`
- `internal\types\types.go`
- `internal\temperature\temperature.go`
- `internal\smartcontrol\rise_prediction.go`
- `internal\coreapp\monitoring.go`
- `frontend\src\app\components\FanCurve.tsx`
- locale files for `zh-CN`, `en-US`, and `ja-JP`

## Should Do If Low Risk

- [ ] Add a short release-note draft in `docs\release-notes\fancontrol-2.1.6.md`.
- [ ] Add a small developer note explaining that WiFi identity is profile/name based and IP is a mutable connection property.
- [ ] Review current Chinese strings touched by this release only; do not run broad language cleanup.
- [ ] Keep Japanese and English locale key parity for any new strings.

## Defer

- [ ] FlyDigi BLE/HID frontend exposure.
- [ ] FlyDigi real-device validation.
- [ ] DIY protocol-template UI.
- [ ] Device-feature whitelist expansion beyond bug fixes.
- [ ] Noise fingerprint / axial-noise avoidance UI.
- [ ] Full noise-zone avoidance based on fan acoustic fingerprint.
- [ ] Time-based curve schedules.
- [ ] Stdio bridge mode from THRM v3.1.5.
- [ ] Large settings-page redesign.
- [ ] Full WiFi software smart start/stop state machine.

## Compatibility Rules

- [ ] 2.0 and 2.1 users must upgrade without losing fan curves.
- [ ] Existing WiFi IP must survive upgrade and remain mirrored to legacy `fanControlDeviceIp`.
- [ ] Existing device profiles must survive upgrade.
- [ ] Hidden FlyDigi beta profiles must not become user-visible built-in devices.
- [ ] Existing imported themes must keep rendering after any UI text/control changes.
- [ ] Autostart and tray behavior must not regress.
- [ ] Installer behavior must not damage shared PawnIO installations used by other software.

## Test Plan

### Backend

- [ ] `go test ./internal/smartcontrol ./internal/types`
- [ ] `go test ./internal/coreapp`
- [ ] `go test ./internal/device`
- [ ] `go test ./...`
- [ ] Add focused tests for per-curve learned offsets.
- [ ] Add focused tests for stopped-fan wakeup resend behavior.
- [ ] Add focused tests for WiFi scan overlap/cancel behavior if scan locking is implemented.

### Frontend

- [ ] `npx tsc --noEmit`
- [ ] `npm run build`
- [ ] Locale key parity for `zh-CN`, `en-US`, and `ja-JP` if strings change.
- [ ] User will do final UI preview; still keep obvious layout regressions out.

### Packaging

- [ ] Build portable package.
- [ ] Build installer only when release is requested.
- [ ] If installer changes PawnIO behavior, test the no-PawnIO, current-PawnIO, and older-PawnIO paths as far as possible.
- [ ] Do not commit, push, or release unless explicitly requested.

## Acceptance Criteria

- [ ] Switching curve profiles no longer shares learned offsets unexpectedly.
- [ ] Percent learning still uses 0.1% precision internally.
- [ ] RPM learning still uses 1 RPM precision.
- [ ] A stopped or zero-target device receives a positive wakeup target when auto control wants it running.
- [ ] Core fatal crashes leave useful logs for diagnosis.
- [ ] WiFi scan controls do not become confused by overlapping scan requests.
- [ ] PawnIO repair/update behavior is either kept intentionally conservative or improved with clear user-facing wording.
- [ ] No existing user configuration is discarded during normalization.
- [ ] Build and tests pass before preview.

## Implementation Status - 2026-06-10

- [x] Added per-curve learned offset preservation based on the THRM v3.1.5 approach. Existing single `SmartControl.LearnedOffsets` remains the active profile view, and first migration preserves old learning data for the active curve.
- [x] Added stopped-fan wakeup resend logic for auto control. A positive target is resent when the device reports current speed 0 or target speed 0.
- [x] Added Core fatal stdout/stderr capture on Windows with a non-Windows stub.
- [x] Kept the installer-level PawnIO strategy conservative; changed only the in-app repair/update action so it no longer uninstalls first.
- [x] Added backend WiFi scan overlap protection so repeated scan requests are rejected while one scan is already running.
- [x] Rechecked WiFi smart start/stop Beta code path: quit, stop, and system suspend call the standby-speed send; failed sends clear the internal applied flag; resume clears the flag.
- [x] Added `温升预判` Beta, default off, near fan-curve smart-control settings. It uses sustained temperature rise and optional CPU/GPU power readings, caps boost, and ignores isolated spikes.
- [x] Added `docs/release-notes/fancontrol-2.1.6.md` and updated README/version metadata to 2.1.6.
- [x] Verified before packaging with `git diff --check`, `go test ./...`, `npx tsc --noEmit`, `npm run build`, and `dotnet build bridge/TempBridge/TempBridge.csproj -c Release`.
