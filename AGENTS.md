# Agent Guide

This is the `Eureka-o/FanControlPortable` repository. User-visible software name is `FanControl`.

## Always Start Here

Before code work, read:

1. `../MEMORY_INDEX.md`
2. `../SKILL_ROUTER_MEMORY.md`
3. `MEMORY_INDEX.md`
4. `SKILL_ROUTER_MEMORY.md`
5. A relevant file under `docs/codex-handoffs/`, when present

Do not rely only on old conversation context.

## Product Rules

- Preserve upgrade compatibility for 2.0/2.1/2.2 users: config, WiFi IP, device profiles, curve profiles, learning data, themes, tray settings, and autostart settings should survive updates.
- WiFi and virtual serial/COM are compatibility modes and may be manually selected/configured.
- BLE/HID native devices are scanned/traversed automatically. Built-in FlyDigi profiles should not be manually enabled as compatibility devices.
- FlyDigi built-ins:
  - `飞智（FlyDigi）BS1`: BLE, RPM.
  - `飞智（FlyDigi）BS2`, `BS2PRO`, `BS3`, `BS3PRO`: HID, RPM.
- Device-specific UI/settings should be driven by device capabilities and whitelists, not hard-coded transport assumptions.
- Homepage, tray, curve editor, learning state, and manual gear state must respect the active runtime device and its speed unit.

## Architecture Notes

- Keep communication layers separated by transport/protocol. Avoid putting all device behavior into one large function.
- WiFi executor should keep existing packet behavior but keep protocol parsing, HTTP transport, and control flow separate.
- HID FlyDigi code has a cgo/HIDAPI path and a Windows native fallback path.
- Do not fake current fan speed from target speed. Target fallback may fill target only, not current speed.
- Native BLE/HID curve state must stay device-scoped and RPM-safe.

## Reference Software

When the user asks to compare or port from the reference project, verify the checkout first:

```powershell
git -C "D:\Desktop\风扇控制便携版\THRM-reference-git" fetch --tags
git -C "D:\Desktop\风扇控制便携版\THRM-reference-git" describe --tags --always --dirty
git -C "D:\Desktop\风扇控制便携版\THRM-reference-git" log -1 --decorate --oneline
```

Then inspect the reference implementation before claiming behavior.

## Validation

Use the smallest meaningful checks while iterating, and run broader checks before packaging/release:

```powershell
go test ./...
cd frontend; npx tsc --noEmit
cd frontend; npm run build
cmd /c build.bat
```

If `build.bat` hits a transient file lock during portable compression, retry packaging with:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\package_portable.ps1 -Version <version>
```

## Git And Release

- Do not commit, push, tag, or publish unless explicitly asked.
- When committing, stage explicit files only; do not use broad `git add .` in this repo.
- Write release notes for users: emphasize visible changes and compatibility, not debug logs or internal experiments.
- Do not mention sponsor/About UI changes in release notes unless the user asks.
