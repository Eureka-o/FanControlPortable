# FanControl 2.1.1 Release Notes

FanControl 2.1.1 is a focused bugfix release for the 2.1.x line. This update fixes percent-learning display, percent/RPM device switching, and device capability binding. All users are recommended to update, especially users who have imported custom devices, switched between percent and RPM profiles, or used learning curves.

## Downloads

- `FanControl-2.1.1-amd64-installer.exe`: recommended for most Windows users.
- `FanControl-2.1.1-portable.zip`: portable build; extract and run `FanControl.exe`.

SHA256:

- `FanControl-2.1.1-amd64-installer.exe`
  - `3E7FC770487EB358E0ECC7885C391411E8E431D9742B11F2E7DAEC10A91E535F`
- `FanControl-2.1.1-portable.zip`
  - `F1D6C69308D88DFF8E5ADBB3CEAD4756E48CBA69BFEA5AF3C5758B1F51F5F385`

## Fixed

- Fixed percent-learning display on the fan-curve page. Percent learned offsets are stored internally as `0.1%` ticks, so a 25-tick offset now displays as `2.5%` instead of `25%`.
- Fixed speed-unit switching between percent and RPM devices. Speed labels, ranges, curve axes, tooltips, learned-curve summaries, manual gear editing, custom speed input, homepage gauges, mini charts, and title-bar badges now follow the active device profile.
- Fixed stale fan-speed display after switching devices or connection types. Frontend speed readings now trust device telemetry only when its transport and speed unit match the currently enabled device profile.
- Fixed RPM manual/custom speed control after switching from a percent device. Backend profile selection now prefers the enabled device for the current connection type, so RPM targets are no longer clamped or sent through the previous percent profile.
- Fixed RPM-device curve normalization. Old 0-100 percent-shaped curves are no longer reused as valid-looking 0-4000 RPM curves after switching to an RPM profile.
- Fixed non-speed capability defaults. WiFi, library BLE, serial, and legacy RPM/HID profiles no longer infer lighting, power-on-start, smart start/stop, raw commands, or debug-frame support from the connection type alone.
- Fixed backend capability enforcement. Unsupported non-speed actions are rejected before config is updated; the default WiFi runtime no longer reports fake success for lighting, power-on-start, smart start/stop, brightness, light-strip, or RGB-off calls.
- Fixed Settings UI capability binding. Lighting, power-on-start, and smart start/stop controls stay hidden unless a future whitelisted device profile explicitly enables them.

## Compatibility

- Existing FanControl 2.0/2.1 configs, WiFi IP, curve profiles, learning data, user device profiles, and imported themes remain compatible.
- The user-facing app name remains `FanControl`; the repository and updater continue to use `Eureka-o/FanControlPortable`.
- This release does not change the original/reference THRM app and keeps FanControl's config, process, task, IPC, and updater paths isolated.

## Notes

- WiFi smart start/stop and BLE smart start/stop are deferred to FanControl 2.2.0 so they can be implemented with explicit capability checks, probing, rollback, and device-specific safety handling.
- Real hardware validation is still dependent on device-owner feedback; this release was validated with unit tests, frontend builds, mock/runtime paths, and Windows packaging.

## Validation

- `go test ./internal/device ./internal/types ./internal/coreapp`
- `go test ./...`
- `npx tsc --noEmit`
- `npm run build`
- locale key parity for `en-US`, `zh-CN`, and `ja-JP`
- `git diff --check`
- `build.bat`
- portable zip content inspection
- file version checks for `FanControl.exe`, `FanControl Core.exe`, and the installer
