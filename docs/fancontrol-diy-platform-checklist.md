# FanControl DIY Platform Checklist

Date: 2026-06-06

This checklist records the planned direction for turning FanControl into an extensible fan/cooler control platform while preserving the current FanControl 2.0 behavior.

## Resume Rule

- [ ] After any context compaction, archive handoff, resumed thread, or new Codex session, reread this checklist before changing code.
- [ ] Also reread the root memory files before implementation:
  - [ ] `<workspace-root>/MEMORY_INDEX.md`.
  - [ ] `<workspace-root>/SKILL_ROUTER_MEMORY.md`.
  - [ ] The latest relevant handoff under `FanControlPortable/docs/codex-handoffs/`.
- [ ] Do not rely on old chat memory for architecture decisions if this checklist says otherwise.
- [ ] Before implementation, restate the active phase and constraints from this checklist in the working notes.

## Release Scope

- [ ] Target release line is `2.1.0`.
- [ ] Do not commit changes unless the user explicitly asks for a commit.
- [ ] Keep work reviewable as staged/unstaged files until the user approves.
- [ ] Update README-facing documentation for 2.1.0 features when implementation changes user-visible behavior.
- [ ] Prepare release notes for `FanControl 2.1.0`.
- [ ] Release notes must preserve the user-facing app name `FanControl`.
- [ ] Release notes must not rename the repository/updater away from `Eureka-o/FanControlPortable`.
- [ ] Release notes should explain advanced DIY/device-profile features carefully and warn ordinary users that the default supported-device workflow remains available.
- [ ] Portable and installer builds must preserve the Windows elevation manifests used by the main app/Core; do not accidentally ship a non-elevating build when hardware-temperature access requires admin.
- [ ] Portable and installer builds must preserve app and tray icon resources: `FanControl.exe`, `FanControl Core.exe`, favicon/brand assets, and the Core embedded tray icon should all use the current FanControl icon.

## Core Direction

- [ ] Keep the user-visible app name as `FanControl`.
- [ ] Keep the repository and updater URL as `Eureka-o/FanControlPortable`.
- [ ] Preserve migration from old `FanControlPortable` configs, tasks, history, and assets.
- [ ] Keep coexistence with the original/reference THRM app.
- [ ] Keep reference-style/imported themes working after device architecture changes.
- [ ] Avoid turning new work into a single-device shell; new device support should be profile/driver based.
- [ ] Keep the project easy to extend: new devices should be added through profiles/drivers, not broad edits across frontend, smart-control, and transport internals.
- [ ] Keep performance predictable: new advanced features must be lazy, bounded, and inactive unless selected or opened.
- [ ] FanControl is a PC/Wails desktop application; narrow-window checks are desktop resize checks, not mobile or phone support.
- [ ] Do not spend default design, implementation, or QA effort on phone/mobile interaction patterns, touch-first layouts, mobile breakpoints, or phone screenshots unless the user explicitly asks.

## Collaboration And Implementation Guardrails

- [ ] Sub-agents may be used for module design exploration, UI layout alternatives, architecture review, or code-reading summaries.
- [ ] The primary Codex agent must personally verify tests, builds, screenshots, and final quality.
- [ ] Do not outsource final test judgment to a sub-agent.
- [ ] Prefer non-UAC tests and commands so the work can be completed autonomously.
- [ ] Do not run installer flows, driver install/uninstall flows, scheduled-task elevation flows, or other UAC-triggering validation unless the user explicitly approves.
- [ ] Prefer unit tests, frontend builds, mock-device tests, and non-elevated integration tests.
- [ ] Keep destructive operations out of the workflow unless explicitly requested.
- [ ] Do not commit, tag, push, or publish release assets unless the user explicitly asks.

## Architecture Goals

- [ ] Split device control into clear layers:
  - [ ] Device profile.
  - [ ] Transport.
  - [ ] Framing.
  - [ ] Command template.
  - [ ] Checksum/encoding.
  - [ ] Send scheduling/rate limiting.
  - [ ] Response parsing.
  - [ ] State mapping.
  - [ ] Speed-unit mapping.
- [ ] Make future devices easy to add by implementing a driver/profile instead of editing core UI logic.
- [ ] Let UI render from capabilities instead of hardcoded device names such as BS1/BS2/HID.
- [ ] Bind frontend display and backend control decisions to explicit device information:
  - [ ] Speed unit and speed range come from the enabled device profile, not from a global percent/RPM assumption.
  - [ ] Control path is selected by the enabled transport/profile channel: WiFi, BLE, virtual COM/serial, or legacy HID/RPM.
  - [ ] Stale telemetry from another transport must not override the currently selected profile's unit or range.
- [ ] Keep legacy/reference RPM code isolated from the new percent-speed path.
- [ ] Keep code highly cohesive and loosely coupled:
  - [ ] Device-type-specific code should live in separate packages/files where practical.
  - [ ] Percent control/learning code should not be mixed into legacy RPM driver code.
  - [ ] WiFi/BLE/serial/HID communication details should not be mixed into smart-control learning code.
  - [ ] Shared helpers are allowed only when they are genuinely unit-neutral and do not hide transport or device-specific behavior.
  - [ ] Avoid adding every new function to one large file; split by responsibility as modules grow.
  - [ ] Do not let unrelated code types accumulate in one function, one controller, or one catch-all package; split device drivers, transports, protocol codecs, control policy, learning, persistence, and UI orchestration into high-cohesion modules with narrow interfaces.
- [ ] Treat code isolation as a hard requirement for every new device/transport/control feature:
  - [ ] Each device family, transport, and protocol should own its private packet-building, parsing, validation, and runtime state.
  - [ ] Shared contracts should stay narrow and explicit: profile/config in, capabilities/state/target-speed command out.
  - [ ] If one file starts mixing unrelated device types, UI flows, transport details, and control decisions, split it before adding more behavior.
  - [ ] Avoid broad cross-device conditionals in hot paths; select the active profile/driver once and call clear interfaces.
- [ ] Non-speed device features must be whitelist/capability driven:
  - [ ] Do not infer lighting, screen/display, power-on-start, smart start/stop, raw debug frames, or vendor-specific functions from the connection type alone.
  - [ ] Default WiFi, library BLE, serial, and legacy RPM profiles should expose only speed/read capabilities unless a specific device whitelist enables more.
  - [ ] Devices with extra hardware such as lighting or a small screen need explicit profile/capability fields and dedicated backend executors before the frontend displays those controls.
  - [ ] Unsupported feature controls should be hidden or disabled by capability, not left visible with a transport-only assumption.
- [ ] Isolate different device types and connection paths as first-class modules:
  - [ ] WiFi percent devices, BLE devices, virtual serial/COM devices, and legacy RPM/HID devices should have separate driver/profile code paths.
  - [ ] Device-profile data models, transport I/O, protocol/framing, command templates, response parsers, control decisions, and learning logic should stay in their own layers.
  - [ ] New device support should usually add a small driver/profile plus tests, not expand generic files such as `device.go`, `manager_wifi.go`, `target.go`, or a single large frontend component with unrelated logic.
  - [ ] Cross-layer communication should use explicit interfaces/contracts with speed units and capabilities, not hidden globals or transport-specific conditionals scattered through control/UI code.
  - [ ] Avoid circular dependencies between profile, transport, protocol, control, coreapp, and frontend-facing API layers.
- [ ] Sub-agents may be used for module boundary design or code organization review when the task becomes large, but primary Codex keeps final implementation and test responsibility.
- [ ] When a task becomes broad, use sub-agents for focused design/review of modules where helpful, then record the chosen boundaries before implementation.

## UI Design Inheritance Requirements

- [ ] New UI must inherit the existing FanControl app shell instead of introducing a separate visual language.
- [ ] Use the existing left sidebar pattern for new primary/submenu entries.
- [ ] New sidebar items must use existing sidebar theme tokens:
  - [ ] `bg-sidebar`.
  - [ ] `text-sidebar-foreground`.
  - [ ] `border-sidebar-border`.
  - [ ] `sidebar-accent`.
  - [ ] `primary`.
- [ ] New panels/rows/buttons must use existing app tokens:
  - [ ] `bg-background`.
  - [ ] `bg-card`.
  - [ ] `bg-muted`.
  - [ ] `text-foreground`.
  - [ ] `text-muted-foreground`.
  - [ ] `border-border`.
  - [ ] `primary`.
- [ ] New icons should use the existing lucide icon style unless a local component already provides a better match.
- [ ] Do not introduce a marketing/landing-page style screen for advanced device management.
- [ ] Keep the UI quiet, operational, and tool-like: dense enough for repeated use, but not cluttered.
- [ ] Do not put cards inside cards.
- [ ] Do not add decorative gradient blobs, oversized hero areas, or unrelated visual flourishes.
- [ ] Keep text sizes appropriate for app panels and sidebars; avoid hero-scale headings inside tool surfaces.
- [ ] Keep button text short and prevent wrapping/overflow on narrow windows.
- [ ] Use existing reusable controls where possible: buttons, switches, selects, sliders, dialogs, tabs, collapsibles, badges, tooltips.
- [x] New UI must respect imported/reference themes and custom theme CSS variables.
- [ ] If a new visual token is truly needed, add it in a theme-compatible way and verify imported themes still render acceptably.
- [ ] The "advanced device/profile" area must visually feel like part of the current `AppShell`, not a separate application.

## Localization Requirements

- [ ] Treat Simplified Chinese (`zh-CN`), English (`en-US`), and Japanese (`ja-JP`) as first-class UI languages.
- [ ] Treat Japanese (`ja-JP`) as a quality gate, not a fallback translation after Chinese and English are done.
- [ ] Every new user-facing string must be added to all three locale files in the same change.
- [ ] Japanese strings must be reviewed for readable product UI wording, not just literal or machine-style translation.
- [ ] Do not leave Chinese or English placeholder text in `ja-JP` keys unless it is a deliberate brand/product term.
- [ ] For new settings, dialogs, warnings, and profile/debug flows, write Japanese copy that is short enough for the existing FanControl layout while still being clear.
- [ ] Avoid encoding regressions and mojibake in locale files, docs, generated Wails bindings, and release-note snippets.
- [ ] Add or run a locale-content sanity check for mojibake, replacement characters, and accidentally swapped common action labels before considering a localization slice done.
- [ ] Keep warning gates, raw-command confirmations, device import/export, profile editor fields, runtime-policy fields, validation errors, and empty states fully localized in all three languages.
- [ ] Verify Japanese UI fit for every new advanced-device flow in desktop/Wails app windows.
- [ ] Do not design, test, or optimize for phone/mobile use by default; FanControl is a PC/Wails desktop application.
- [ ] New rendered QA runs should target normal desktop and narrow desktop/Wails-style windows by default; do not require or produce mobile/browser-phone screenshots unless the user explicitly asks.
- [ ] Treat narrow desktop QA as a text-fit and resize check for the desktop app, not as a mobile breakpoint or mobile UX target.
- [ ] Button labels, badges, dialog titles, placeholders, and table/list rows must not clip or overflow in `ja-JP` at normal desktop and narrow desktop window widths.
- [ ] Locale key parity must be checked for `en-US`, `zh-CN`, and `ja-JP` before considering a UI slice done.
- [ ] When README or release notes describe new user-visible 2.1.0 behavior, make sure language/localization status is not contradicted by the UI.
- [x] Imported/reference themes must still render localized Japanese text acceptably after new panels or dialogs are added.

## Advanced Device Menu Requirements

- [x] Add a supported-devices / advanced-device menu entry in the existing left sidebar or as an integrated submenu.
- [x] The entry should inherit existing sidebar icon, hover, active, tooltip, and theme behavior.
- [x] On first entry, show an empty guarded page with warning text and an "I understand" action.
- [x] After that action, show a modal/dialog with a second warning and confirmation.
- [x] Only after the warning flow should the actual advanced device/profile UI be shown.
- [x] Keep the unlock session-scoped by default unless the user later requests persistent unlock.
- [x] The actual page should include:
  - [x] Supported/known devices.
  - [x] User-entered devices.
  - [x] Import device information.
  - [x] Export device information.
  - [x] Top-right `Add device` action.
  - [x] User device library owns `Enable device`, edit, delete, and batch-export actions; no separate top-right `Manage devices` action should duplicate the device library.
  - [x] Debug/test area for profile creation.
- [x] `Add device` should support both importing a device profile and creating a new device profile manually.
- [x] New/manual device creation should branch by device type/transport, because WiFi, BLE, serial, and legacy RPM devices require different fields.
- [x] Known/supported WiFi devices should be selectable from a built-in library when available, then the user manually selects the current active device.
- [x] Ordinary users should be able to ignore this page without changing normal FanControl behavior.
- [x] Remove the UX conflict between "Add Device -> import" and the page-level import area:
  - [x] Add Device should focus on creating a device from template/manual fields.
  - [x] Page-level import should own batch device-file import.
  - [x] Labels should distinguish templates from user devices in Chinese, English, and Japanese.
- [x] Add a connection-category banner or segmented control inside a device detail view so one device can expose WiFi/BLE/virtual COM/legacy HID connection entries where appropriate.

## Legacy RPM Platform

- [ ] Move/reference THRM-style HID/RPM logic behind an isolated legacy RPM driver.
- [ ] Use the reference application's RPM control and learning behavior as the baseline for RPM devices, with only small parameterization needed for FanControl integration.
- [ ] Do not rewrite RPM behavior through the FanControl percent-control implementation.
- [ ] Keep the reference-style RPM learning method for RPM profiles.
- [ ] Treat one RPM learning unit as `1 RPM`.
- [ ] Keep old debug frame parsing available only where the legacy driver supports it.
- [ ] Do not let legacy RPM names leak into the new percent-speed UX.
- [ ] Preserve attribution and license boundaries for code derived from/reference-compatible with THRM.

## Percent-Speed Platform

- [ ] Make percent speed the clean FanControl main path.
- [ ] Implement percent control and percent learning as FanControl-owned code, separate from the legacy/reference RPM path.
- [ ] Use internal percent speed ticks where `1 tick = 0.1%`.
- [ ] Store learned percent offsets in ticks when the profile is percent-based.
- [ ] Round/clamp at packet-send time for devices that only accept integer `0-100%`.
- [ ] Allow future devices that support decimal percent to send decimal values directly.
- [ ] Keep existing JSON/config fields compatible until a safe migration layer exists.

## Learning Mechanism

- [x] Keep the reference-style learning approach:
  - [x] Observe steady-state temperature and speed.
  - [x] Estimate local cooling effect.
  - [x] Apply learned offsets to the curve.
  - [x] Smooth offsets.
  - [x] Enforce monotonic speed.
- [x] Make learning unit-aware:
  - [x] RPM profile: unit is `1 RPM`.
  - [x] Percent profile: unit is `0.1%`.
- [x] Keep learning bias modes:
  - [x] Balanced.
  - [x] Cooling-only.
  - [x] Quiet-only.
- [x] Keep reset learned offsets behavior.
- [x] Do not break existing learned offset config migration.
- [x] Add tests for percent learning precision and send-time rounding.
- [x] Add tests for RPM learning compatibility.

## Device Profile Model

- [x] Add a reusable `DeviceProfile` concept.
- [x] Support built-in profiles and user-created profiles at the backend/API level.
- [ ] Split the product language and data model into two visible concepts:
  - [x] Device templates/profiles: reusable protocol/control templates, including built-in supported-device templates and DIY templates.
  - [x] User devices: actual imported or manually created devices that FanControl can enable and control.
  - [x] Normal control UI should say "enable device" / "enabled device", not "start profile" or "active profile".
  - [x] The home/status connection name should come from the enabled user device where available, not only from the template/profile name.
  - [x] The built-in/default WiFi supported device name and model should be `Slim压风散热器Pro`, so the Devices page and normal enabled-device display match the product name.
- [x] Separate the device library from the template/profile library:
  - [x] Template library: read-only or reusable baselines for packet shape, learning unit, speed mapping, parsers, and transport defaults.
  - [x] Device library: user-owned devices already imported/created and available for normal control.
  - [x] Add Device should create a device from a template or a blank form; it should not duplicate the main import-device workflow.
- [x] Import/export should become device-file based:
  - [x] Export all user devices in one file by default, including name, speed unit/range, transport settings, command templates, parsers, runtime policy, and enough data to control the device after import.
  - [x] Import should accept drag-and-drop and file picker selection.
  - [x] Import should merge by union, deduplicate by stable device/profile IDs, and avoid silently deleting existing user devices.
  - [x] The current `FCDP1.` string import/export can remain as a compatibility/debug path inside the `.fcdp` file workflow.
- [x] Enabled-device selection rules:
  - [x] Each connection type should have at most one enabled user device at a time.
  - [x] FanControl may keep one enabled device for each supported connection type simultaneously: WiFi, BLE, virtual serial/COM, and legacy HID.
  - [x] The normal Settings connection selector chooses the current connection category; detailed device selection belongs in Devices.
- [x] Treat WiFi as a single active device path for now; multiple WiFi devices should be selected manually instead of being auto-driven at the same time.
- [x] Allow the currently selected WiFi device/profile to overwrite the default WiFi profile settings, so the normal workflow stays simple.
- [x] Keep the active connection type in normal settings so users can switch between WiFi and BLE without entering the advanced panel.
- [x] Use conditional settings fields:
  - [x] When connection type is WiFi, show only IP/address in normal Settings; advanced endpoints and protocol fields stay in Devices.
  - [x] When connection type is BLE, show BLE scan/matching fields.
    - [x] BLE matching fields from the selected profile are shown in normal settings.
    - [x] Actual BLE scan action is available from the advanced BLE profile editor.
    - [x] Normal Settings has a direct manual BLE scan shortcut that can apply a matched device back to the active BLE profile.
  - [x] When connection type is serial, show virtual COM/port and baud rate in normal Settings; frame/protocol fields stay in Devices.
  - [x] Hide irrelevant transport-specific fields, similar to how dependent options only appear when their parent mode is enabled.
- [x] Store profile fields for:
  - [x] Display name.
  - [x] Device model/vendor notes.
  - [x] Transport type.
  - [x] Speed unit.
  - [x] Speed range.
  - [x] Optional percent-to-RPM mapping.
  - [x] Connection settings.
  - [x] Command templates.
  - [x] Response parsing rules.
  - [x] Capability flags.
- [x] Let users save a tested debug setup as a profile.
- [x] Keep profiles portable with the app config where possible.
- [x] Backend import/export device information must include enough data for FanControl to use the device without hidden local assumptions:
  - [x] Profile ID and display name.
  - [x] Vendor/model/notes.
  - [x] Transport type and connection settings.
  - [x] Speed unit, speed range, step/tick scale, and optional percent-to-RPM mapping.
  - [x] State read endpoint/interface.
  - [x] Speed set endpoint/interface.
  - [x] HTTP method, payload shape, raw frame/template string, encoding, and checksum mode as applicable.
  - [x] Response parser rules for state, current speed, target speed, success/failure, and errors.
  - [x] Capability flags used by the UI.
  - [x] Version/schema marker for future migrations.
  - [x] UI import/export flow.

## Transports

- [x] WiFi:
  - [x] Device address/IP.
  - [x] State endpoint.
  - [x] Speed control endpoint.
  - [x] HTTP method.
  - [x] JSON field mapping.
  - [x] Timeout and retry policy.
  - [x] Manual profile selection for more than one known WiFi device.
  - [x] Default WiFi device can be replaced by the selected/imported profile in normal use.
  - [x] Built-in WiFi device library entry for known supported controllers where packet shape is already known.
- [ ] BLE:
  - [x] Device scan.
  - [x] Device-name filter.
  - [x] Service UUID.
  - [x] Write characteristic UUID is represented in profiles and matching accepts discovered values.
  - [x] Notify characteristic UUID is represented in profiles and matching accepts discovered values.
  - [x] Write mode: with response / without response is represented in profiles.
  - [x] Real GATT service/characteristic discovery at BLE runtime for configured service/write/notify UUIDs.
  - [x] Optional GATT probing before profile save to discover and suggest characteristics automatically.
  - [x] BLE connect/read-state/set-speed runtime through an isolated BLE executor.
- [x] Virtual serial/COM:
  - [x] Manual port selection from the active profile.
  - [x] Baud rate.
  - [x] Data bits / stop bits / parity if needed.
  - [x] Hex or ASCII payload.
  - [x] Newline/frame delimiter.
  - [x] Backend connect/read-state/set-speed runtime through an isolated serial executor.
  - [x] Windows COM handle open/configuration implemented without adding a new dependency.
  - [x] Tests cover fake serial command writes, response parsing, rate limiting, manager connection, target speed updates, and disconnect cleanup.
  - [x] Automatic COM port discovery/listing.
- [ ] Legacy HID:
  - [ ] Kept behind the legacy RPM driver.
  - [ ] Exposed only when the profile/driver supports it.

## Debug-To-Profile Workflow

- [ ] Upgrade the current debug panel into a profile builder.
- [ ] Let the user choose transport in the area currently used for device IP.
- [ ] Let the user choose speed mode: percent or RPM.
- [ ] Let the user test:
  - [x] Connect.
  - [x] Read status.
  - [x] Set speed.
  - [x] Raw command send.
  - [x] Response parsing.
- [ ] Support common parsers:
  - [x] JSON path.
  - [x] Byte offset.
  - [x] Regex.
  - [x] Plain success/failure response.
- [x] Support common checksum modes:
  - [x] None.
  - [x] Sum8.
  - [x] XOR8.
  - [x] CRC16 if needed.
  - [x] Apply checksum modes to `hex`, `ascii`, and `raw` byte payloads.
  - [x] Keep structured `json` commands as pure JSON and reject binary checksum modes.
- [x] Add dangerous-command confirmation for raw sends.
- [x] Add send rate limiting.
- [x] Keep bounded logs for debug sessions.
- [x] Save successful settings as a reusable device profile.

## Control / Communication Split

- [x] Keep the control module focused on target speed decisions, learning, smoothing, ramp limits, and safety constraints.
- [x] Keep the communication module focused on transport, framing, endpoint selection, payload encoding, send timing, and response parsing.
- [x] Control code should call an abstract target-speed command with explicit unit instead of directly knowing WiFi/BLE/HID packet details.
- [x] Communication code should report capability/state data back to control without owning smart-control policy.
- [ ] This split should make it possible to add a new transport without editing the learning algorithm, and add a new control strategy without rewriting packet send code.

## Frontend Adaptation

- [ ] Replace hardcoded UI assumptions with `DeviceCapabilities`.
- [ ] Show percent curve controls for percent profiles.
- [ ] Show RPM curve controls for RPM profiles.
- [ ] Hide unsupported features instead of showing broken controls.
- [ ] If a profile cannot read device state, show target speed and last-send status only.
- [x] Keep the current reference-style theme import and theme switching working.
- [ ] Use existing theme tokens for new UI:
  - [x] `bg-card`.
  - [x] `text-foreground`.
  - [x] `text-muted-foreground`.
  - [x] `border-border`.
  - [x] `primary`.
- [ ] Avoid hardcoded colors for new platform controls unless a theme variable is added.

## Performance Requirements

- [ ] Keep control loops lightweight and predictable.
- [ ] Pre-compile reusable profile templates/parsers instead of parsing them on every send.
  - [x] Response parsers are compiled when the WiFi profile executor is created.
  - [ ] Command templates still use bounded string rendering; add a dedicated compiled template form only if later templates become more complex or show measurable overhead.
- [x] Reuse HTTP clients and transport handles.
- [ ] Do not block the UI thread on WiFi/BLE/serial I/O.
- [x] Rate-limit device sends to avoid flooding DIY controllers.
- [ ] Rate-limit high-frequency IPC events.
- [x] Keep debug logs bounded by size/count.
- [ ] Avoid unbounded goroutines per device event.
- [ ] Avoid large allocations in temperature/control hot paths.
- [ ] Keep learning work proportional to curve length, not history size.
- [ ] Keep response parsing deterministic and time-bounded.
- [x] Do not rescan BLE devices continuously while already connected unless the user starts a scan.
- [ ] Do not poll serial/WiFi faster than the profile's configured interval.
- [ ] Add benchmarks or targeted tests before introducing expensive parsers or scripting hooks.
- [ ] Preserve performance while adding isolation:
  - [ ] Do not put reflection, dynamic scripting, repeated schema parsing, or repeated template compilation in hot control/send paths.
  - [ ] Keep abstractions thin enough that normal percent/RPM control paths do not pay for inactive BLE/serial/debug features.
  - [ ] Prefer compiled profile executors/parsers and injected interfaces over runtime string dispatch in polling/control loops.

## Compatibility Requirements

- [x] Existing `FanControl` 2.0 config loads without manual migration.
- [ ] Existing old `FanControlPortable` config migration remains intact.
- [x] Software updates must preserve all user configuration, including WiFi IP/address, fan curves, curve-profile selection, enabled devices, imported user devices, and device-profile data.
- [x] Installer upgrades must back up and restore the current `config/config.json` path as well as legacy root-level config files.
- [x] Legacy install-root `config.json` is migrated into the current `config/config.json` path when the current portable config path is missing.
- [x] Installer runs clear stale `%TEMP%` config backup files before copying fresh upgrade backups, so an interrupted older install cannot be accidentally restored later.
- [ ] Existing history file migration remains intact.
- [ ] Existing autostart/task cleanup remains intact.
- [ ] Existing GitHub updater URL remains intact.
- [x] Existing imported/reference themes remain usable.
- [ ] Existing mock WiFi device remains usable for regression tests.

## Test Plan

- [x] Go tests for speed-unit conversion:
  - [x] Percent ticks to integer percent.
  - [x] Percent ticks to decimal percent.
  - [x] RPM pass-through.
- [x] Go tests for learning:
  - [x] Percent profile steady-state offset.
  - [x] Percent send-time rounding.
  - [x] RPM profile compatibility.
  - [x] Learning bias constraints.
- [x] Go tests for profile parsing:
  - [x] WiFi JSON profile.
  - [x] BLE UUID profile validation.
  - [x] Serial hex/ascii profile validation.
  - [x] Invalid profile rejection.
  - [x] Command checksum modes and JSON checksum rejection.
- [x] Go tests for BLE scan matching:
  - [x] Device-name filter match.
  - [x] Service UUID match.
  - [x] Write/notify characteristic match when characteristic data is available.
  - [x] Profile-based match scoring.
  - [x] Empty scan result.
- [x] Go tests for BLE runtime:
  - [x] Fake BLE connector/client open path.
  - [x] `readState` command write and response parsing.
  - [x] `setSpeed` command write with percent tick template rendering.
  - [x] Write-with-response / write-without-response selection.
  - [x] Synthetic target-state fallback for write-only BLE profiles.
  - [x] BLE send rate limiting.
  - [x] Normal device manager BLE connect/set-speed/settings/disconnect path.
- [x] Go tests for transport send scheduling and rate limits.
- [x] Frontend build after Wails model changes.
- [ ] Frontend checks for percent/RPM adaptive labels.
- [x] Locale JSON parse and key parity for `en-US`, `zh-CN`, and `ja-JP`.
- [x] Locale content sanity check for `ja-JP`, including no mojibake, no copied Chinese/English placeholders, and no swapped common action labels.
- [x] Rendered Japanese (`ja-JP`) QA for advanced-device desktop/Wails app flows after each UI change.
- [x] Japanese text overflow/clipping scan for warning gates, dialogs, forms, buttons, badges, and profile rows at normal and narrow desktop window widths.
- [x] Do not spend default QA time on phone-sized browser/mobile layouts; test normal desktop and narrow desktop/Wails-style windows unless explicitly requested.
- [ ] Manual test with mock WiFi device.
- [x] Manual test importing a reference-style theme.
- [ ] Manual test that original/reference THRM can coexist with FanControl.

## Suggested Implementation Order

- [ ] Phase 1: document and freeze current compatibility constraints.
- [ ] Phase 2: introduce neutral speed-unit types while keeping old fields compatible.
- [ ] Phase 3: isolate the legacy RPM driver.
- [ ] Phase 4: make WiFi the first clean percent driver.
- [ ] Phase 5: make learning unit-aware.
- [ ] Phase 6: add `DeviceCapabilities` and UI capability rendering.
- [ ] Phase 7: add `DeviceProfile` storage and validation.
- [ ] Phase 8: upgrade debug panel into debug-to-profile workflow.
- [ ] Phase 9: add BLE and serial profile support.
- [ ] Phase 10: performance pass and full regression validation.

## Implementation Notes

### 2026-06-06 Backend Foundation

- [x] Added explicit speed-unit helpers for percent ticks and RPM values.
- [x] Percent path now has FanControl-owned `0.1%` tick helpers and send-time integer-percent rounding helpers.
- [x] Legacy/reference RPM defaults were restored for RPM profiles:
  - [x] RPM curve default: `1000-4000 RPM`.
  - [x] Smart-control defaults: `MinRPMChange=50`, `RampUpLimit=220`, `RampDownLimit=160`, `MaxLearnOffset=300`.
  - [x] Manual gear defaults: reference-style `1300-4000 RPM` table.
- [x] Added initial `DeviceProfile` / `DeviceCapabilities` model and built-in WiFi percent + legacy RPM profiles.
- [x] Legacy RPM profile can preserve HID or BLE transport instead of forcing BLE configs back to HID.
- [x] Added unit-aware config and curve validation so RPM curves are not clamped to `0-100%`.
- [x] Preserved old frontend/config compatibility by keeping new device profile fields when older config update payloads omit them.
- [x] Split smart-control entry points:
  - [x] `percent_control.go` for FanControl percent/tick control and learning wrappers.
  - [x] `legacy_rpm.go` for reference-style RPM control and learning wrappers.
  - [x] Shared helpers remain in common files only for unit-neutral math.
- [x] Added explicit device-manager target speed API:
  - [x] Default WiFi build supports percent target sends.
  - [x] Default WiFi build rejects direct RPM target sends instead of pretending they are percent.
  - [x] Legacy build routes RPM target sends to the existing reference-style RPM path.
- [x] Added focused Go tests for speed conversion, device profile defaults, RPM/percent config validation, and percent/RPM smart-control paths.
- [x] Verified with non-UAC `go test ./...`.
- [x] Advanced device UI, import/export profile UI, README update, and rendered UI checks were continued in the next 2.1.0 UI pass.
- [x] Full connect/read/set-speed profile-builder controls are available through transient draft-profile tests.
- [ ] Real BLE hardware validation and release notes finalization are still pending.

### 2026-06-06 Device Profile Import/Export Foundation

- [x] Added isolated `internal/deviceprofiles` package for profile clone, validation, import, and export logic.
- [x] Added `FCDP1.` device-profile export format with explicit schema/version marker.
- [x] Added profile validation for:
  - [x] WiFi endpoint/state/speed endpoint and HTTP method.
  - [x] BLE service/write/notify UUID shape.
  - [x] Serial port, baud rate, data bits, stop bits, and parity.
  - [x] Command template encoding/checksum.
  - [x] Response parser type/expression.
  - [x] Percent-to-RPM speed map ordering and bounds.
- [x] Added CoreApp/IPC/GUIApp APIs for configured profiles, supported profiles, user profiles, active-profile selection, save, delete, import, and export.
- [x] Regenerated Wails bindings so frontend models include `DeviceProfile`, `DeviceProfilesPayload`, `activeDeviceProfileId`, and `deviceProfiles`.
- [x] Added frontend API service wrappers for the new device-profile APIs.
- [x] Verified with non-UAC `go test ./...`.
- [x] Verified frontend production build with `npm run build`.
- [x] Advanced UI screens and import/export UI were added in the next 2.1.0 UI pass.
- [x] Actual debug-to-profile save workflow is now covered by the advanced UI save-from-raw flow.
- [x] Full profile-builder connect/read/set-speed controls are wired through a draft-profile test API without saving or selecting the profile.
- [ ] Real BLE hardware validation is still pending.

### 2026-06-06 Advanced Devices UI Foundation

- [x] Added a left-sidebar `Devices` entry that uses the existing AppShell/sidebar icon and active-state styling.
- [x] Added a session-scoped warning gate and second confirmation dialog before showing advanced device controls.
- [x] Added an advanced device panel with supported/known profiles, user profiles, import/export profile strings, top-right Add/Manage actions, and a bounded raw-command test area.
- [x] Added a per-send confirmation dialog for advanced raw-command sends, covering both button clicks and Enter-key sends.
- [x] Added a save-from-debug flow that turns the last successful raw-command result plus the active profile connection settings into an editable, reusable device profile draft.
- [x] Added an Add Device dialog that supports import or manual profile creation, with transport-specific fields for WiFi, BLE, serial/virtual COM, and legacy HID/RPM.
- [x] Advanced Devices has a protected user-device library: built-in templates stay read-only, user devices can be enabled, edited, and deleted from their row actions, and delete uses an in-app confirmation dialog instead of a native browser confirm.
- [x] Improved Japanese localization for the new UI and fixed the Japanese `apply`/`import` button labels.
- [x] Normalized frontend `BRAND.name` to `FanControl` so browser title, About panel, and sidebar/logo accessibility labels do not show an old user-visible name.
- [x] Updated README-facing docs for the 2.1.0 advanced-device/profile workflow.
- [x] Verified with non-UAC `go test ./...`, `npx tsc --noEmit`, `npm run build`, and locale JSON parsing.

### 2026-06-06 WiFi Profile Runtime Foundation

- [x] Added isolated `internal/deviceprofileexec` runtime support for WiFi device profiles.
- [x] WiFi profile executors now support custom state endpoints, speed endpoints, HTTP methods, JSON/ascii/raw/hex command templates, checksum modes, and response parsers.
- [x] Response parsers are compiled when the WiFi executor is created, including JSON path, byte offset, regex, and plain response modes.
- [x] The default WiFi manager now receives the full active `DeviceProfile` instead of only transport/IP.
- [x] The selected/imported WiFi profile can replace the built-in `/api/data` and `/api/speed` runtime behavior without editing smart-control code.
- [x] Percent target-speed sends preserve internal `0.1%` ticks until the executor renders a command template, then expose `{{percent}}`, `{{percentTicks}}`, and `{{decimalPercent}}`.
- [x] WiFi RPM profiles can send explicit RPM values through their own profile runtime, while default percent WiFi profiles still reject direct RPM sends.
- [x] CoreApp now updates the device manager from the active profile on startup, config updates, connect, and reconnect.
- [x] Profile changes with the same WiFi connection target update the executor without forcing a disconnect; true connection-target changes still disconnect/reconnect.
- [x] Added focused Go tests for custom WiFi percent profiles, WiFi RPM profiles, template rendering, parser behavior, and device failure response rejection.
- [x] Verified with non-UAC `go test ./...`.
- [ ] `legacydevice` tag verification is still blocked in this local environment by `github.com/sstallion/go-hid` build-symbol errors before FanControl code is compiled.
- [x] Added WiFi runtime policy fields for request timeout, minimum send interval, max retries, and retry backoff.
- [x] WiFi `SetSpeed` now applies the configured minimum send interval before sending speed commands.
- [x] WiFi HTTP requests now use per-profile request timeout and retry retryable HTTP/network failures with bounded backoff.
- [x] WiFi runtime policy fields are validated, preserved in `FCDP1.` import/export, exposed in Wails models, and editable from the advanced device profile dialog.
- [x] Added focused Go tests for WiFi retry behavior, send rate limiting, and invalid runtime-policy rejection.
- [x] Added a shared 100-frame debug buffer helper and normal-build tests so WiFi/debug attempts cannot grow logs without bound.
- [x] Full connect/read/set-speed profile-builder controls can test WiFi/BLE/serial drafts through temporary executors.
- [ ] Real BLE hardware validation remains pending.

### 2026-06-06 Serial Profile Runtime Foundation

- [x] Added isolated `internal/deviceprofileexec` runtime support for virtual serial/COM device profiles.
- [x] Added build-tagged serial port opening/configuration:
  - [x] Windows COM handle open via existing `golang.org/x/sys/windows`.
  - [x] Baud rate, data bits, stop bits, parity, and read/write timeouts are applied from the selected profile.
  - [x] Non-Windows builds return a clear unsupported error instead of silently pretending serial is available.
- [x] Serial executors support command-template rendering, ASCII/raw/hex payloads, checksum modes inherited from the shared command encoder, newline/frame delimiters, bounded response reads, retry/backoff, and send rate limiting.
- [x] Serial profiles can connect through the normal device manager, read state when a `readState` command exists, set percent or RPM targets through profile command templates, and report serial transport/source data to `FanControl`.
- [x] Write-only serial controllers get a synthetic target-state fallback so FanControl can still show the last requested speed when no response parser is configured.
- [x] Core health checks now refresh serial state through the serial path when the active connected transport is serial.
- [x] Added fake-port tests for serial executor command writes, response parsing, delimiter handling, rate limiting, manager connect/set-speed behavior, settings reporting, and disconnect cleanup.
- [x] Automatic COM port discovery/listing is now wired through backend discovery, Wails bindings, and the advanced profile editor.
- [x] Advanced Devices profile-builder buttons can test serial connect/read/set-speed through the transient draft-profile tester.

### 2026-06-06 Normal Settings Device Connection UX

- [x] Added normal Settings controls for active device connection selection.
- [x] Users can switch WiFi, BLE, virtual COM/serial, and legacy HID transports from normal settings without entering the advanced Devices page.
- [x] Cross-transport active profile selection now preserves the selected profile transport in backend config normalization.
- [x] Normal Settings no longer exposes the device-profile selector; it shows the enabled device name and keeps detailed device/template selection in Devices.
- [x] FanControl still drives one enabled device for the current connection category at a time; future work should preserve one enabled device per category across WiFi/BLE/serial/HID.
- [x] WiFi settings show and save only the IP/address in normal Settings; state endpoint, speed endpoint, packet templates, and parsers stay in Devices.
- [x] BLE settings show selected-profile matching details: name filter, service UUID, write characteristic, and notify characteristic.
- [x] BLE settings include a manual scan shortcut; detected devices can apply address/name/service/write/notify suggestions to the active BLE profile without opening the advanced Devices page.
- [x] Serial settings allow editing the virtual COM/port and baud rate from normal Settings, with lazy COM-port detection and manual virtual-port entry.
- [x] Serial frame shape, delimiter, command templates, and response parsers stay in Devices.
- [x] Irrelevant transport-specific settings are hidden when another transport is selected.
- [x] Added English, Simplified Chinese, and Japanese locale strings for the normal device-connection controls and toasts.
- [x] Removed remaining hardcoded Chinese from visible normal Settings/Debug text so `ja-JP` renders from locale keys instead of mixed-language literals.
- [x] Verified Japanese normal Settings device-connection rendering in desktop and narrow desktop Wails-style windows, including WiFi/BLE/virtual COM/legacy HID field switching, no horizontal overflow, and no clipped buttons.
- [x] Verified Japanese normal Settings BLE scan shortcut in desktop and narrow desktop Wails-style windows: mocked `ScanBLEDevices` returned a matched device, the result was shown, clicking it saved BLE address/service/write/notify fields to the active BLE profile, and no horizontal overflow or clipped buttons were found.
- [x] Confirmed mobile/browser-phone layout is no longer a release gate for FanControl unless explicitly requested.
- [x] Normal Settings includes the direct BLE scan shortcut.

### 2026-06-06 Device Library / Template Library Split

- [x] Added per-transport enabled-device memory through `activeDeviceProfileIdsByTransport` while preserving legacy `activeDeviceProfileId` compatibility.
- [x] Normalization now remembers the enabled WiFi/BLE/serial/HID device independently, so switching connection categories can return to the last enabled device for that category instead of always choosing the first profile.
- [x] Device-profile export/import now preserves the per-transport enabled-device map when exported user devices include those IDs.
- [x] Device import remains union/merge based and does not delete existing user devices.
- [x] Deleting the active user device now falls back to another device with the same transport when possible.
- [x] Advanced Devices now shows a connection-category banner for WiFi, BLE, virtual COM, and legacy HID with the enabled device for each category.
- [x] The template library is shown as a read-only template source; the user device library is grouped by connection type and owns `Enable device`.
- [x] The user device library owns batch export to one `FanControl-devices.fcdp` file; page-level import owns file picker / drag-and-drop import, while Add Device focuses on creating from a template or blank/manual fields.
- [x] Device editing preserves user-device IDs so saving an edit updates the existing device instead of creating an accidental duplicate.
- [x] The built-in/default WiFi supported device now uses `Slim压风散热器Pro` as display name and model in normalization tests.
- [x] Focused tests cover per-transport active-device normalization, export/import mapping, SetActive mapping, and delete fallback.
- [ ] A future deeper migration may split a dedicated `UserDevice` persistence type from `DeviceProfile`; the current 2.1.0 slice keeps config compatibility by using `DeviceProfile` plus `BuiltIn` and per-transport active IDs.

### 2026-06-06 Device Library Edit/Delete/Batch Export Cleanup

- [x] Removed the visible top-level `Manage devices` action because it duplicated the device-library role.
- [x] Moved batch export into the user device library action area.
- [x] User-device rows now expose edit and delete actions directly in the device library.
- [x] Device edit mode preserves the existing user-device ID so saving updates the same device.
- [x] Delete now uses an in-app warning dialog that shows device name, transport, speed range, and connection summary instead of a native confirm.
- [x] The built-in/default WiFi supported device name/model is normalized to `Slim压风散热器Pro`.
- [x] User requested build-level validation as sufficient for this slice; deeper rendered display QA is deferred until the user previews the portable build and reports visible issues.

### 2026-06-06 Native Export And Default Device Display Cleanup

- [x] Device-library batch export now uses a Wails native save-file dialog instead of browser-style blob/download behavior.
- [x] The WiFi template-library entry is separated from the concrete default device name; the template uses `FanControl WiFi 百分比控制模板`, while the default built-in device remains `Slim压风散热器Pro`.
- [x] The Devices page device library now shows the built-in default device alongside imported/manual devices, while keeping built-in devices non-editable and non-deletable.
- [x] Template rows and device rows use separate badges (`template` vs `built-in device`) so users do not confuse reusable templates with enabled devices.
- [x] The homepage disconnected state now uses the enabled device name and shows the disconnected/waiting panel instead of showing live temperature gauges plus an empty fan-speed gauge while the cooler is offline.
- [x] Verified this slice with build-level validation per user request: `go test ./...`, `npx tsc --noEmit`, locale key parity, `npm run build`, Core build, Wails build, and `git diff --check`.

### 2026-06-06 Enabled Device Name Consistency

- [x] Default-build WiFi `GetModelName()` now returns the enabled active profile display name instead of always returning the fixed built-in product name.
- [x] WiFi connection info now uses the enabled active profile display name/vendor for `product`, `model`, and `manufacturer` where available.
- [x] `QueryDeviceSettings()` now reports the enabled active profile display name for WiFi as well as BLE/serial/HID paths.
- [x] Added a WiFi manager regression test proving a user-created WiFi device name reaches connection info, status model name, and device settings.

### 2026-06-06 Automatic COM Port Discovery

- [x] Added `SerialPortInfo` data returned through the GUI/Wails API.
- [x] Added Windows COM port discovery from `HKLM\HARDWARE\DEVICEMAP\SERIALCOMM` using the existing `golang.org/x/sys/windows/registry` dependency.
- [x] Added normalization, deduplication, and COM-number sorting so `COM3` sorts before `COM12`.
- [x] Added non-Windows fallback that returns a clear unsupported result.
- [x] Regenerated Wails bindings and added the frontend `apiService.listSerialPorts()` wrapper.
- [x] The advanced device profile editor now lazy-loads detected ports when the selected transport is virtual COM/serial.
- [x] Users can pick a detected port or type a virtual/manual port name directly.
- [x] The refresh action is icon-only with localized accessible text/title and inherits existing button styling.
- [x] Verified rendered `ja-JP` UI in normal desktop `1280x900` and narrow desktop `900x700` windows, with `COM3`/`COM12` visible, `COM12` selection syncing into manual input, no framework overlay, no relevant console errors, no horizontal overflow, and no clipped buttons.

### 2026-06-06 BLE Scan And Matching Foundation

- [x] Added `BLEDeviceInfo`, `BLEManufacturerData`, and `BLEScanParams` models for Wails/API use.
- [x] Added isolated `internal/deviceprofileexec` BLE scan/matching foundation with an injectable scanner interface.
- [x] Real scans are manual, lazy, time-bounded, and capture advertised address, name, RSSI, service UUIDs, and manufacturer data where the desktop BLE stack provides them.
- [x] Matching scores can use name filters, service UUIDs, write characteristic UUIDs, notify characteristic UUIDs, and BLE profiles.
- [x] Characteristic UUID fields are modeled and matchable when a future GATT probe provides them; advertisement-only scans usually cannot discover write/notify characteristic UUIDs.
- [x] Added GUI/Wails `ScanBLEDevices` API and regenerated bindings.
- [x] Added advanced BLE profile editor UI for manual BLE scan, matched device rows, empty/error states, and one-click fill for name filter/service UUID suggestions.
- [x] Added English, Simplified Chinese, and Japanese strings for the BLE scan UI.
- [x] Added Go tests for name/service/characteristic/profile matching, matched-only filtering, and empty scan results.
- [x] Rendered the BLE scan/profile-editor flow in Japanese desktop windows:
  - [x] Normal desktop `1280x900`.
  - [x] Narrow desktop/Wails-style `900x700`.
  - [x] Flow covered Devices -> warning gate -> second confirm -> Add Device -> BLE -> Scan BLE -> matched result visible -> click result -> BLE name/service/write/notify fields filled.
  - [x] Mocked matched BLE advertisement data included address, name, RSSI, service UUIDs, write UUID, notify UUID, and matched profile display name.
  - [x] No relevant console errors, no horizontal overflow, and no clipped button text were found in this flow.
  - [x] Screenshot evidence saved under `%TEMP%`: `fancontrol-ble-scan-desktop-result-ja.png`, `fancontrol-ble-scan-desktop-filled-ja.png`, `fancontrol-ble-scan-narrow-desktop-result-ja.png`, and `fancontrol-ble-scan-narrow-desktop-filled-ja.png`.
- [x] Full BLE connect/read-state/set-speed backend runtime foundation is implemented through an isolated executor.
- [x] Optional GATT service/characteristic discovery before profile save is implemented through the manual GATT probe flow.

### 2026-06-06 BLE Profile Runtime Foundation

- [x] Added isolated BLE profile executor code under `internal/deviceprofileexec`.
- [x] Added injectable `BLEConnector` / `BLEClient` interfaces so runtime behavior can be unit-tested without real BLE hardware.
- [x] Added default BLE connector behavior using the existing `tinygo.org/x/bluetooth` dependency:
  - [x] Use profile `endpoint` as a BLE address when provided.
  - [x] Otherwise run a bounded profile-based scan/match before connecting.
  - [x] Connect to the matched/addressed BLE device.
  - [x] Discover configured GATT service, write characteristic, and notify/read characteristic.
  - [x] Enable notifications when available, with a bounded notification buffer.
- [x] BLE executors support command-template rendering, JSON/ascii/raw/hex payloads, shared checksum modes, configurable write-with-response behavior, response parsing, retry/backoff, send rate limiting, and write-only synthetic target state.
- [x] Normal device manager can connect selected/imported BLE profiles, read state when a `readState` command or readable/notify characteristic is configured, set percent or RPM targets, report BLE source/settings data, refresh BLE state in health checks, and close BLE clients on disconnect/reconfigure.
- [x] Added fake-client tests for BLE executor read/set/rate-limit behavior and normal manager BLE connect/set-speed/settings/disconnect behavior.
- [x] Verified the focused BLE runtime slice with non-UAC `go test ./internal/deviceprofileexec ./internal/device`.
- [ ] Real BLE hardware validation is still pending.
- [x] Optional GATT probing before profile save is implemented through the manual profile editor.
- [x] Full connect/read/set-speed profile-builder UI controls are wired through the transient draft-profile tester.

### 2026-06-06 BLE GATT Probe Foundation

- [x] Added `BLEGATTProbeParams`, `BLEGATTProbeResult`, service info, and characteristic info models for Wails/API use.
- [x] Added isolated `internal/deviceprofileexec` BLE GATT probe support with an injectable prober interface for tests.
- [x] The default prober resolves a BLE target from an explicit address or existing profile scan/match fields, connects through the desktop BLE stack, discovers services/characteristics, and disconnects after probing.
- [x] Characteristic properties are captured when the local BLE stack exposes them; Windows exposes read/write/write-without-response/notify/indicate style flags through the current dependency.
- [x] Probe results suggest service UUID, write characteristic, and notify/read characteristic without saving or selecting the draft profile.
- [x] Added GUI/Wails `ProbeBLEGATT` API and regenerated bindings.
- [x] The advanced BLE profile editor now includes a manual GATT probe button, result list, and apply-suggestions action.
- [x] Clicking a BLE scan result now also fills the BLE address, and BLE drafts no longer inherit the WiFi default IP as a hidden endpoint.
- [x] Added English, Simplified Chinese, and Japanese strings for the GATT probe UI.
- [x] Added focused Go tests for fake GATT probe result normalization, preferred UUID preservation, and suggestion generation.
- [x] Rendered Japanese desktop QA for the GATT probe flow:
  - [x] Normal desktop `1280x900`.
  - [x] Narrow desktop/Wails-style `900x700`.
  - [x] Flow covered Devices -> warning gate -> second confirmation -> Add Device -> BLE -> BLE scan -> click matched device -> GATT probe -> apply suggested UUIDs.
  - [x] Mock Wails API recorded `ScanBLEDevices` and `ProbeBLEGATT` calls, then the draft fields held BLE address `AA:BB:CC:DD:EE:01`, service UUID `fff0`, write characteristic `fff2`, and notify characteristic `fff1`.
  - [x] No visible framework overlay, relevant console errors, horizontal overflow, or clipped button text were found in this flow.
  - [x] Screenshot evidence saved under `%TEMP%`: `fancontrol-gatt-probe-desktop-result-ja.png`, `fancontrol-gatt-probe-desktop-applied-ja.png`, `fancontrol-gatt-probe-narrow-result-ja.png`, and `fancontrol-gatt-probe-narrow-applied-ja.png`.
- [ ] Real BLE hardware validation of the GATT probe remains pending.

### 2026-06-06 Draft Profile Test Controls

- [x] Added a transient profile-test API that accepts a draft `DeviceProfile` without saving it or selecting it as active.
- [x] The draft tester uses the isolated WiFi, BLE, and serial executors instead of the active device manager, so profile-building tests do not mutate the normal control path.
- [x] Added profile-builder controls for:
  - [x] Connect.
  - [x] Read status.
  - [x] Set speed.
- [x] Percent test speed accepts decimal values and is converted through internal `0.1%` ticks before packet/template rendering.
- [x] RPM test speed remains an integer RPM value.
- [x] Added Wails/API bindings and frontend service wrappers for the draft profile-test path.
- [x] Added English, Simplified Chinese, and Japanese UI strings for the profile-test controls and results.
- [x] Added focused Go tests for WiFi draft connect, serial draft read/close, and BLE draft percent set-speed with tick rendering.
- [x] Rendered Japanese desktop QA for the profile-test controls:
  - [x] Normal desktop `1280x900`.
  - [x] Narrow desktop/Wails-style `900x700`.
  - [x] Flow covered Devices -> warning gate -> second confirmation -> Add Device -> profile test Connect -> Set speed.
  - [x] Mock Wails API recorded transient `TestDeviceProfile` calls for WiFi percent connect and set-speed without saving or selecting the draft.
  - [x] No relevant console errors, horizontal overflow, or clipped button text were found in this flow.
  - [x] Screenshot evidence saved under `%TEMP%`: `fancontrol-profile-test-desktop-ja.png`, `fancontrol-profile-test-narrow-ja.png`, and `fancontrol-profile-test-narrow-result-ja.png`.

### 2026-06-06 Theme Compatibility Regression

- [x] Verified the advanced Devices page and Add Device dialog use existing theme tokens rather than a separate visual language.
- [x] Rendered Japanese desktop QA with a mock imported custom theme:
  - [x] Normal desktop `1280x900`.
  - [x] Narrow desktop/Wails-style `900x700`.
  - [x] Mock Wails APIs returned `QA Contrast Theme`; `SystemThemeSync` called `ListThemes` and `GetThemeCSS`.
  - [x] `<html data-theme="qa-contrast">` was applied, `thrm-custom-theme-style` was injected, and computed root variables changed for `--background`, `--card`, `--primary`, and `--sidebar`.
  - [x] The Devices page and Add Device dialog rendered without visible framework overlays, relevant console errors, horizontal overflow, or clipped button text.
  - [x] Screenshot evidence saved under `%TEMP%`: `fancontrol-theme-advanced-devices-desktop-main-ja.png`, `fancontrol-theme-advanced-devices-desktop-dialog-ja.png`, `fancontrol-theme-advanced-devices-narrow-main-ja.png`, and `fancontrol-theme-advanced-devices-narrow-dialog-ja.png`.
- [x] Rendered Japanese desktop QA with the bundled reference-style `themes/thrm/theme.css`:
  - [x] Normal desktop `1280x900`.
  - [x] Narrow desktop/Wails-style `900x700`.
  - [x] `<html data-theme="thrm">` was applied, the bundled theme CSS was injected, and computed variables included `--background: #f7fbff`, `--card: #fbfdff`, `--primary: #2f74ff`, and `--sidebar: #f8fbff`.
  - [x] The Devices page and Add Device dialog rendered without visible framework overlays, relevant console errors, horizontal overflow, or clipped button text.
  - [x] Screenshot evidence saved under `%TEMP%`: `fancontrol-theme-thrm-advanced-devices-desktop-main-ja.png`, `fancontrol-theme-thrm-advanced-devices-desktop-dialog-ja.png`, `fancontrol-theme-thrm-advanced-devices-narrow-main-ja.png`, and `fancontrol-theme-thrm-advanced-devices-narrow-dialog-ja.png`.

### 2026-06-06 Command Checksum Modes

- [x] Profile command templates now apply `none`, `sum8`, `xor8`, and Modbus-style `crc16` checksum modes to `hex`, `ascii`, and `raw` byte payloads.
- [x] Structured `json` command templates remain pure JSON and reject non-`none` checksum modes instead of silently ignoring them.
- [x] The manual profile editor now locks checksum to `none` for `json` commands and exposes checksum choices for `hex`, `ascii`, and `raw` commands.
- [x] Device-profile validation accepts the supported checksum modes, rejects unsupported checksum names, and rejects JSON commands with binary checksum modes.
- [x] The transient profile-builder test path sends checksum-appended serial draft packets without saving or selecting the draft profile.
- [x] Verified with non-UAC `go test ./internal/deviceprofileexec ./internal/deviceprofiles`.
- [x] Rendered Japanese desktop QA for checksum editor adaptation:
  - [x] Normal desktop `1280x900`.
  - [x] Narrow desktop/Wails-style `900x700`.
  - [x] Flow covered Devices -> warning gate -> second confirmation -> Add Device -> command encoding `json` -> checksum locked to disabled `none` -> command encoding `hex` -> checksum options `none` / `sum8` / `xor8` / `crc16` -> select `xor8`.
  - [x] No visible framework overlay, relevant console errors, horizontal overflow, or clipped button text were found in this flow.
  - [x] Screenshot evidence saved under `%TEMP%`: `fancontrol-checksum-desktop-ja.png` and `fancontrol-checksum-narrow-desktop-ja.png`.

### 2026-06-06 Learning Precision Regression

- [x] Added regression tests proving percent smart-control targets can keep `0.1%` tick precision instead of collapsing to integer percent internally.
- [x] Added regression tests proving percent learning stores learned offsets in ticks, including a `25` tick (`2.5%`) learned offset.
- [x] Added regression tests proving send-time conversion rounds percent ticks to integer percent only at the device-send boundary.
- [x] Added regression tests proving legacy/reference RPM learning keeps `1 RPM` offset units.
- [x] Added stable-observer tests proving percent samples stay in ticks while legacy RPM samples stay in RPM.
- [x] Verified the focused learning/speed tests with non-UAC `go test ./internal/smartcontrol ./internal/types`.
- [x] Re-verified the broader non-UAC suite after this slice:
  - [x] `go test ./...`.
  - [x] `npx tsc --noEmit`.
  - [x] `npm run build`.
  - [x] Locale key parity for `en-US`, `zh-CN`, and `ja-JP`.
  - [x] `git diff --check`.

### 2026-06-06 Legacy HID Normal Build Gate

- [x] Added an explicit normal-build legacy HID/RPM stub under `internal/device`.
- [x] Default builds now route `hid` transport through the stub instead of falling through to WiFi/BLE behavior.
- [x] The stub reports that the legacy HID/RPM driver is not enabled unless the `legacydevice` build path is used.
- [x] Normal-build model/settings reporting now preserves the active legacy RPM profile name for HID transport.
- [x] Percent and RPM speed commands remain rejected while the normal-build HID stub is disconnected.
- [x] Verified with non-UAC `go test ./internal/device`.
- [ ] `legacydevice` tag verification remains blocked by the local `github.com/sstallion/go-hid` bus-type symbol errors.

### 2026-06-06 FanControl 2.1.1 Speed-Unit Decoupling Fixes

- [x] Confirmed 2.1.1 scope is a focused bugfix release; WiFi smart start/stop and BLE smart start/stop are deferred to 2.2.0.
- [x] Fixed frontend active-device lookup so the selected connection transport takes priority over stale global `activeDeviceProfileId` values.
- [x] Made fan-speed helpers profile-aware for speed unit, speed range, clamping, and display formatting.
- [x] Made stale `fanData` transport-safe so telemetry from a previous device no longer overrides the current enabled device unit/range after switching connection types.
- [x] Switched built-in/library non-speed capabilities to whitelist-only defaults: library BLE and legacy RPM no longer advertise lighting, power-on-start, smart start/stop, raw commands, or debug-frame support unless future device-specific profiles opt in.
- [x] Normal Settings custom-speed controls now render from the current device's capability declaration instead of assuming every device has the same visible controls.
- [x] Normal Settings device-feature panels now render from capabilities: lighting controls appear only for profiles with `supportsLighting`, power-on-start only for `supportsPowerOnStart`, and smart start/stop only for `supportsSmartStartStop`.
- [x] Backend non-speed control entry points now reject unsupported active-device capabilities before updating config, and the default WiFi manager no longer returns fake success for lighting, power-on-start, smart start/stop, brightness, light-strip, or RGB-off calls.
- [x] Reaffirmed that extra hardware such as lighting or small screens must be future whitelist/profile features, not inferred from BLE/WiFi/serial/HID connection type.
- [x] Updated the fan-curve page so percent learned offsets stored as `0.1%` ticks display as decimal percent values, while RPM offsets remain 1-RPM units.
- [x] Updated fan-curve axes, tooltips, history summaries, manual gear editing, and manual gear tooltip/range labels to follow the current device unit and range.
- [x] Updated homepage realtime fan gauge, mini curve preview, history fan chart, and title-bar status badges to follow the active device profile instead of assuming `0-100%`.
- [x] Updated custom speed controls, warning dialog, and range hint to show the current device range/unit instead of fixed `0-100%`.
- [x] Added backend curve-profile protection so percent-shaped curves are replaced with RPM defaults when normalizing for RPM devices.
- [x] Added 2.1.1 draft release notes at `docs/release-notes/fancontrol-2.1.1-draft.md`.
- [x] Verified with `go test ./internal/curveprofiles ./internal/coreapp ./internal/smartcontrol ./internal/types`, `go test ./...`, `npx tsc --noEmit`, `npm run build`, locale key parity, and `git diff --check`.

### 2026-06-07 FanControl 2.2.0 FlyDigi Scope Correction

- [x] FlyDigi BS1/BS2/BS2PRO/BS3/BS3PRO backend protocol and enumeration work may remain in the tree as beta groundwork.
- [x] FlyDigi profiles must not be registered as user-visible built-in/supported devices, shown in Advanced Devices, or saved into user `deviceProfiles`.
- [x] FlyDigi auto-match must not update `activeDeviceProfileId`, `activeDeviceProfileIdsByTransport`, or normal Settings display.
- [x] Config normalization should remove hidden FlyDigi profile IDs that were accidentally written during this 2.2.0 development pass.
- [x] Frontend visible device connection should remain at the 2.1.2 formal-release level until FlyDigi real-device behavior is explicitly reopened.
- [x] Verified this boundary with `go test ./...`.

### 2026-06-07 FanControl 2.2.0 Compatibility Mode Split And Device Binding

- [x] Settings `Device connection` no longer uses one outer compatibility-mode panel for both manual transports.
- [x] WiFi compatibility mode is shown as its own normal Settings `SettingRow` with title, description, and switch.
- [x] Virtual serial/COM compatibility mode is shown as its own normal Settings `SettingRow` with title, description, and switch.
- [x] Compatibility submenus expand through the same `AnimatePresence` + child-row pattern used by the temperature-smoothing option, not through large nested cards inside `Device connection`.
- [x] WiFi compatibility mode stays enabled by default for the existing Slim pressure-fan WiFi workflow.
- [x] Virtual serial/COM compatibility mode follows the saved configuration state.
- [x] WiFi and virtual serial/COM compatibility modes are for DIY controllers, older devices, or devices that cannot be used through automatic/native identification.
- [x] BLE and HID transports remain available in Advanced Devices for imported or manually configured device profiles.
- [x] Only FlyDigi device information and the FlyDigi automatic-identification frontend entry are hidden for now; BLE/HID as connection types must not be removed.
- [x] WiFi binding is by device/profile name, not by IP address.
- [x] The WiFi dropdown label is `Bound device`; the selected profile display name, such as `Slim压风散热器Pro`, is the user-facing identity.
- [x] IP address is a mutable connection property on the selected WiFi device profile, not the device identity.
- [x] Saving WiFi connection settings updates the selected profile's `connection.endpoint` and still syncs `fanControlDeviceIp` only as a legacy compatibility field.
- [x] The current WiFi binding is remembered in `activeDeviceProfileIdsByTransport.wifi`.
- [x] Future WiFi IP discovery and reconnect scanning should update only the bound profile's IP property, then reuse the same device identity.
- [x] Virtual serial/COM keeps the existing profile-selection behavior and remembers the current binding in `activeDeviceProfileIdsByTransport.serial`.
- [x] Saving virtual serial/COM settings updates the selected profile's port and baud-rate connection properties.
- [x] Closing either compatibility-mode switch must not delete the underlying device profile.
- [x] Advanced Devices is the source of truth for device information: names, models, connection defaults, ports, endpoints, UUIDs, command templates, parsers, speed units, and capability flags.
- [x] Normal Settings is the daily connection surface: choose the bound profile and edit only the practical interface fields such as IP, COM port, or baud rate.
- [x] Chinese copy should make this distinction explicit so users understand that Settings changes the interface property while Devices stores the actual device definition.
- [x] The normal `Device connection` row now exposes a right-side `Scan available devices` action with copy explaining that it scans BLE/HID auto-identifiable devices only, not WiFi or serial.
- [x] Normal Settings now shows `Discovered devices` only after the user actively scans; before scanning, the area stays focused on saved devices.
- [x] The former `Paired devices` wording is now `Saved devices` because the source of truth is the Advanced Device Settings library, not a Bluetooth-style pairing record.
- [x] Hidden FlyDigi beta profiles are not shown as normal saved user devices; BLE/HID user profiles remain visible when the user has saved their own profiles.
- [x] WiFi compatibility mode keeps the same saved-device model but stays compact: saved device selection plus a WiFi-only manual IP entry.
- [x] WiFi manual IP add/edit continues to update the saved profile IP property and legacy `fanControlDeviceIp` field, not create IP-as-identity devices.
- [x] WiFi discovered-device UI is reserved for the later IP discovery/scanning work and should appear only after active scanning exists.
- [x] WiFi compatibility submenu is split into separate normal Settings child rows instead of one crowded card/panel: connected device, saved device, scan/deep-scan reserve, manual IP add, and reconnect reserve.
- [x] Manual WiFi IP entry is presented as an add action for the selected saved device type, while the saved device name remains the identity and the IP remains a mutable connection property.
- [x] WiFi reconnect scanning is reserved as a disabled child-panel switch so the future断联重连 implementation can update only the saved profile IP without changing the bound device identity.
- [x] Virtual serial/COM Settings no longer exposes port, baud-rate, or port-detection controls because those conflict with Advanced Device Settings ownership.
- [x] Virtual serial/COM Settings only enables the compatibility path and selects a saved serial profile; port, baud, protocol, command, and parser fields stay in Advanced Device Settings.
- [x] Serial compatibility mode handles the no-profile state as a normal disabled empty state, avoiding Radix Select empty-value runtime crashes when the submenu opens.
- [x] Virtual serial/COM compatibility submenu uses separate normal Settings child rows when information grows; current rows are connected device, saved device, and interface summary.
- [x] Serial interface summary in normal Settings is read-only. Port, baud rate, protocol templates, packet framing, parser rules, and other device facts remain owned by Advanced Device Settings.

### 2026-06-08 FanControl 2.1.3 WiFi IP Discovery Scope

- [x] The in-progress 2.2.0 compatibility/discovery work is temporarily re-scoped as the 2.1.3 release line.
- [x] FlyDigi BS1/BS2/BS2PRO/BS3/BS3PRO protocol code may stay in the backend as hidden beta groundwork, but normal frontend UI must not expose FlyDigi device information, auto-identification wording, or user-visible built-in profiles until the feature is reopened.
- [x] Normal Settings keeps BLE/HID connection types available for imported or manually configured profiles, while WiFi and virtual serial/COM remain separate compatibility modes.
- [x] WiFi device identity remains the saved device/profile name. IP is only a mutable `connection.endpoint` property and is still mirrored to `fanControlDeviceIp` for legacy config compatibility.
- [x] Added WiFi `Find IP` flow: normal search checks the current endpoint, the saved endpoint `/24`, active adapter `/24` ranges, Windows hotspot `192.168.137.0/24`, and device AP point checks.
- [x] Added manual `Deep scan` as a secondary action that appears only after normal search fails; it adds common local ranges such as `192.168.0.0/24`, `192.168.1.0/24`, `192.168.2.0/24`, and `192.168.4.0/24`.
- [x] Added `Dynamic IP compatibility` for WiFi. It is off by default and is meant for dynamic-IP cases such as Windows hotspot; users with router static IP reservation usually do not need it.
- [x] Dynamic IP compatibility only changes the last octet of the saved endpoint during reconnect recovery, then updates the bound WiFi profile endpoint if a matching device responds.
- [x] WiFi probing validates the existing `/api/data` style state response and does not change the `/api/speed` send protocol.
- [x] WiFi discovery requires a saved WiFi device/profile so the software knows which protocol to probe; it can run while the device is not currently connected, but it does not blind-scan without a saved device library entry.
- [x] Deep scan uses a longer IPC timeout, while normal control/config IPC calls keep the existing short timeout.
- [x] Version metadata for this line is now `2.1.3`.

### 2026-06-08 FanControl 2.1.4 WiFi Scan Display And Deep Scan Expansion

- [x] Version metadata for this line is now `2.1.4`.
- [x] WiFi scan results include `scannedCount` so the UI can distinguish total candidates from addresses actually checked before a result or timeout.
- [x] WiFi scan elapsed-time display now treats invalid values as empty and updates live while a scan is running.
- [x] Normal WiFi scan remains lightweight: exact endpoint, saved `/24`, active adapter `/24`, Windows hotspot, and device AP checks.
- [x] Deep scan is staged: first check saved/current/common LAN ranges, then only expand to wider candidate ranges if no device is found.
- [x] Expanded deep scan now covers all `192.168.0.0/24` through `192.168.255.0/24` ranges plus additional common `10.x` and `172.16-31.x` style LAN ranges.
- [x] Deep scan UI shows a spinner, elapsed time, estimated progress, and after the common-range phase displays copy that the search range is being expanded and may be slower.
- [x] Fixed WiFi scan elapsed time reporting so completed scans no longer show `0ms` because of a non-named Go return value.
- [x] Deep scan can be paused, resumed, or canceled through a separate IPC control request; control requests use a separate GUI IPC connection so they are not blocked by the long-running scan request.
- [x] Settings `Find device IP` description no longer truncates like a clipped sentence, and the `Deep scan` action is reserved to the left of `Find IP` to avoid button layout jump.
- [x] Chinese and Japanese scan strings were repaired where prior draft text had been written as question marks.
- [x] README and 2.1.4 release notes describe the WiFi scan display fixes and staged deep-scan behavior without SHA256 hashes.

## Definition Of Done

- [x] Adding a new simple WiFi percent device does not require editing smart-control internals.
- [ ] Adding a new RPM legacy-compatible device does not require editing percent-speed code.
- [x] The UI changes automatically when switching speed unit or transport.
- [x] Learning works for both RPM and percent profiles.
- [ ] New user-facing 2.1.0 UI is complete in Simplified Chinese, English, and readable Japanese, with `ja-JP` layout verified in desktop/Wails app windows.
- [x] Imported/reference themes still render the new UI acceptably.
- [ ] Control loops remain responsive under normal polling intervals.
- [ ] Debug traffic cannot accidentally flood a device without user action.
- [ ] Old configs and release behavior remain compatible.
- [x] Upgrade regression tests prove old install config keeps WiFi IP and fan curve profiles after normalization.
