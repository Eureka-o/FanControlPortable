# 2.5.0 Device Safety Design

**Scope:** Device status, automatic-control safety, and the existing per-device curve/manual-RPM state. SmartControl explanations and new preset storage are deferred.

## Decision

- Keep the current device manager and its `writesBlocked` barrier.
- Derive persistent status from existing connection, suspend, settings, and capability data. Add only a short-lived connect phase so the UI can distinguish discovery from connecting.
- Gate automatic fan writes on a connected, writable, capability-confirmed device plus fresh valid control temperature.
- When telemetry becomes invalid, retain the device's last confirmed target and stop calculating or sending new targets. Do not invent a fixed safe RPM because no configured safety curve exists.
- When a valid sample returns after a safe pause, force one fresh target write through the existing active device curve.

## Status Contract

`GetDeviceStatus` gains a `runtime` object while retaining `connected` unchanged:

- `disconnected`: no active physical connection.
- `discovering`: an automatic scan/reconnect is in progress.
- `connecting`: a selected device is being opened.
- `capabilities`: connected but device settings/capability confirmation is incomplete.
- `ready`: connected, writable, and supports speed control.
- `unavailable`: suspended or connected hardware does not allow speed control.

The status card shows one compact state label beside the existing connection indicator. Temperature, fan speed, and control mode remain the visual focus.

## Existing Configuration Reused

`FanCurveProfilesByDevice` already scopes fan curves and `ManualGearRPM` by device. This slice preserves and displays that behavior; a full preset center, temperature-source scoping, and import conflict preview are separate work.

## Verification

- Unit tests cover status resolution, automatic-control freshness gating, and forced recovery target behavior.
- TypeScript check covers the frontend contract.
- Touched Go packages are tested on Windows; the full repository suite remains outside this slice because its upstream Linux/HID requirements are not stable on this machine.
