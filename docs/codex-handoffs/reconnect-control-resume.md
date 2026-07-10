# Reconnect And Control Resume Handoff

## Current Request

- Fix disconnect/reconnect behavior for FanControl/THRM-style device control.
- Keep the implementation minimal: reuse the existing reconnect loop, WiFi dynamic-IP recovery, and auto-control force-send path.
- Product code edits should be done by small subagents; the main agent verifies diffs and validation.

## User Feedback Being Addressed

1. After hibernate/wake, Windows may show the FlyDigi/BS device as connected, but the app does not automatically reconnect until the user clicks connect.
2. When the app starts in tray/autostart mode, the cooler can stay on the previous manual gear until the user opens the app once.
3. Reconnect should inherit the last connected device. For WiFi, dynamic IP recovery should scan the same subnet like the existing dynamic-IP compatibility flow.

## Relevant Existing Code

- `internal/coreapp/system_device.go`
  - `onDeviceDisconnect` calls `requestReconnect`.
  - `runReconnectLoop` calls `reconnectDevice`, then `reapplyConfigAfterReconnect` when `IgnoreDeviceOnReconnect` is enabled.
  - `reconnectDevice` currently only retries native vs compatibility connection.
  - `reapplyConfigAfterReconnect` logs auto-control restart but does not force the next target send.
- `internal/coreapp/wifi_discovery.go`
  - `recoverDynamicWiFiEndpoint` already scans from the previous endpoint in `WiFiDiscoveryModeDynamic`, updates the active WiFi profile endpoint, saves config, reconfigures the device manager, and broadcasts config.
- `internal/config/config.go`
  - Old configs missing `ignoreDeviceOnReconnect` currently unmarshal to `false`.
- `internal/types/types.go`
  - New default config already sets `IgnoreDeviceOnReconnect: true`.
- `internal/coreapp/monitoring.go`
  - `forceNextAutoTarget` already forces one automatic target send on the next valid monitoring tick.

## Minimal Fix Shape

- Backfill missing `ignoreDeviceOnReconnect` to `true` while preserving explicit `false`.
- On compatibility reconnect failure, try `recoverDynamicWiFiEndpoint` for active WiFi profiles, then retry `ConnectDevice`.
- On reconnect with `AutoControl`, set `forceNextAutoTarget` so the next monitor tick takes control again.

## Boundaries

- Do not add deep WiFi scan to automatic reconnect.
- Do not create new schedulers, broad abstractions, or one-implementation interfaces.
- Do not touch OMEN frontend/plugin work for this fix.
- Do not publish, commit, or package unless the user separately asks.

## Verification Target

- `go test ./internal/config ./internal/coreapp ./internal/device ./internal/types -count=1`
- `gofmt` on touched Go files.
- `git diff --check`.
- `codegraph sync; codegraph status`.
