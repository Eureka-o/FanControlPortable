# Settings Page Tabs Design

## Goal

Shorten the settings workflow without redesigning existing controls. Keep the current card appearance, top bar, themes, settings behavior, and device debug panel.

## Layout

The settings page keeps a single-column outer flow:

1. Persistent overview card.
2. A three-option segmented control: Device Features, Fan Control, System Settings.
3. The selected existing settings section.
4. The existing offline notice when applicable.
5. The existing device debug panel, always visible below every tab.

The first tab is selected whenever the settings page is newly mounted. Tab state is local UI state and is not written to application config.

## Overview Card

Keep the existing `settings-overview` card and replace its three equal tiles with a responsive two-column layout.

- Left tile: CPU and GPU groups. Each group shows current temperature and power.
- Right tile: connected device name, connection state and transport, current fan speed, and current control mode.
- Desktop uses a wider telemetry tile and a narrower device tile. Narrow windows stack the two tiles.
- Missing values render as `--`.
- A GPU in `notPolled` state remains `--`; rendering the overview must not request or wake GPU telemetry.

Use only telemetry and device data already passed to `ControlPanel`. Do not add backend APIs, polling, or dependencies.

## Tab Contents

The first version only separates the existing top-level sections:

- Device Features renders the existing `DeviceFeaturePanel` unchanged.
- Fan Control renders the existing `FanControlSection` unchanged.
- System Settings renders the existing `SystemSettingsSection` unchanged, including its current device connection content.

Keep the three sections mounted and hide inactive panels so switching does not reset drafts, rerun mount-time checks, or reload data. The overview, offline notice, and `DeviceDebugPanel` remain outside the tab panels and are never remounted by tab changes.

The segmented control uses semantic tab roles and existing button/card styling. No new navigation route or settings abstraction is introduced.

## Theme Compatibility

Do not replace or edit theme files in the initial implementation.

- Preserve `data-theme-section="settings-page"`.
- Preserve the outer `data-theme-card="settings-overview"` hook.
- Reuse `settings-overview-temperature` for the combined temperature/power tile.
- Reuse `settings-overview-device` for the device tile.
- Preserve all existing `setting-section`, `setting-row`, and related hooks inside the three sections.
- Give the segmented control a stable `data-theme-ui` hook for optional future theme overrides.

The removed speed tile selector may remain unused in theme CSS. Theme-specific changes are deferred unless visual verification finds a real incompatibility.

## Interaction And Errors

Tab switching changes only visibility and performs no save or API call. Existing settings retain their current loading, validation, toast, and error behavior. The overview uses `--` for unavailable telemetry and keeps the existing offline state behavior.

## Verification

- Add one focused frontend test covering the three tabs, persistent overview/debug placement, and GPU `notPolled` fallback.
- Run the existing frontend tests and TypeScript check.
- Build the frontend and full Windows package after implementation.
- Visually check default light/dark themes plus Dune, Shinchan, and Xiaoba Deluxe because those themes explicitly target settings overview tiles.

## Out Of Scope

- Top bar, sidebar, and global page navigation changes.
- Reordering or redesigning controls inside existing settings sections.
- Moving device connection settings between sections.
- New telemetry sources, GPU wake behavior, or backend changes.
- Theme-specific redesigns.
